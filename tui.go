package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/cursork/gritt/ride"
	_ "github.com/mattn/go-sqlite3"
)

// AccentColor is the UI accent, configurable via "accent" in gritt.json.
// Defaults to Dyalog orange (#F2A74F). Set e.g. "#808080" for grey.
var AccentColor color.Color

func initColors(profile colorprofile.Profile, accent string) {
	complete := lipgloss.Complete(profile)
	if accent != "" {
		AccentColor = complete(
			lipgloss.Color("7"),     // ANSI: white (safe neutral fallback)
			lipgloss.Color("245"),   // ANSI256: grey
			lipgloss.Color(accent),  // TrueColor: user's choice
		)
	} else {
		AccentColor = complete(
			lipgloss.Color("3"),       // ANSI: yellow
			lipgloss.Color("215"),     // ANSI256: #ffaf5f
			lipgloss.Color("#F2A74F"), // TrueColor: Dyalog orange
		)
	}
}

// Cursor style - inverted colors
var cursorStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("255")).
	Foreground(lipgloss.Color("0"))

const aplIndent = "      " // 6 spaces - APL convention

// Line is a single line in the session.
type Line struct {
	Text     string
	Original string
	Edited   bool // True if this line has been modified
}

// Model holds all state for the TUI.
type Model struct {
	client *ride.Client
	msgs   <-chan rideEvent

	// Connection state
	addr      string
	connected bool

	// Session state
	lines        []Line
	cursorRow    int
	cursorCol    int
	ready        bool   // Interpreter ready for input
	lastExecute  string // Last text we sent via Execute (to skip our own echo)
	pendingQuit  bool   // True if last command was )off

	// Debug log (shared with debug pane, survives Model copies)
	debugLog *LogBuffer
	logFile  io.Writer // Optional file for logging (shared across copies)

	// Floating panes
	panes     *PaneManager
	debugPane *DebugPane // Keep reference to update log

	// Editor windows tracked by token
	editors map[int]*EditorWindow

	// Tracer state (for debugger windows)
	tracerStack   []int // Tokens in stack order: bottom to top
	tracerCurrent int   // Currently displayed tracer token (0 = none)

	// Help
	help   help.Model
	keys   KeyMap
	config Config

	// Leader key state
	leaderActive bool
	showQuitHint bool
	confirmQuit  bool
	paneMoveMode bool // Arrow keys move/resize focused pane

	// Save prompt state
	savePromptActive   bool
	savePromptFilename string

	// Backtick mode for APL symbol input
	backtickActive bool

	// Documentation database
	docsDB *sql.DB

	// Autocomplete state
	acPending bool          // True if waiting for ReplyGetAutocomplete
	acPopup   *Autocomplete // Non-nil when popup is showing

	// Internal queries (don't display in session)
	internalQuery    string                   // Command text being executed internally
	internalCallback func(outputs []string)   // Where to send results
	internalOutputs  []string                 // Accumulated outputs for internal query

	// Terminal dimensions
	width  int
	height int

	err error
}

// rideEvent wraps messages from the RIDE reader goroutine.
type rideEvent struct {
	msg *ride.Message
	raw string
	err error
}

// NewModel creates a Model connected to the given RIDE client.
func NewModel(client *ride.Client, addr string, logFile io.Writer, profile colorprofile.Profile) Model {
	cfg := LoadConfig()
	initColors(profile, cfg.Accent)
	m := Model{
		client:    client,
		addr:      addr,
		connected: true,
		ready:     true, // Handshake already completed
		lines:     []Line{{Text: aplIndent}},
		debugLog:  &LogBuffer{}, // Shared buffer survives Model copies
		logFile:   logFile,
		panes:     NewPaneManager(80, 24), // Will be updated on WindowSizeMsg
		editors:   make(map[int]*EditorWindow),
		config:    cfg,
		help:      help.New(),
		keys:      cfg.ToKeyMap(),
	}
	m.cursorCol = len(aplIndent)
	m.msgs = m.startRecvLoop()
	m.log("Connected to %s", addr)

	// Open docs database (optional — F1 help is unavailable without it)
	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "gritt", "dyalog-docs.db")
	if db, err := sql.Open("sqlite3", dbPath+"?mode=ro"); err == nil {
		// Verify the database is usable
		if err := db.Ping(); err == nil {
			m.docsDB = db
			m.log("Docs database loaded: %s", dbPath)
		} else {
			db.Close()
		}
	}

	return m
}

// startRecvLoop starts a goroutine to receive RIDE messages.
func (m *Model) startRecvLoop() <-chan rideEvent {
	ch := make(chan rideEvent)
	go func() {
		for {
			msg, raw, err := m.client.Recv()
			ch <- rideEvent{msg: msg, raw: raw, err: err}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

// send wraps client.Send and handles disconnection on error.
func (m *Model) send(cmd string, args map[string]any) error {
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	err := m.client.Send(cmd, args)
	if err != nil {
		m.connected = false
		m.ready = false
		m.log("Send failed, disconnected: %v", err)
	}
	return err
}

// executeInternal sends an Execute command that won't display in the session.
// The callback receives the outputs when SetPromptType signals completion.
// If a query is already pending, it's replaced (old callback won't be called).
func (m *Model) executeInternal(code string, callback func(outputs []string)) error {
	if m.internalQuery != "" {
		m.log("Replacing pending internal query: %s → %s", m.internalQuery, code)
	}
	m.internalQuery = code
	m.internalCallback = callback
	m.internalOutputs = nil
	m.log("→ Internal: %s", code)
	return m.send("Execute", map[string]any{"text": code + "\n", "trace": 0})
}

// reconnect attempts to reconnect to the RIDE server.
func (m Model) reconnect() (tea.Model, tea.Cmd) {
	if m.connected {
		m.log("Already connected")
		return m, nil
	}

	m.log("Reconnecting to %s...", m.addr)

	// Close old client if exists
	if m.client != nil {
		m.client.Close()
	}

	// Try to connect
	client, err := ride.Connect(m.addr)
	if err != nil {
		m.log("Reconnect failed: %v", err)
		return m, nil
	}

	m.client = client
	m.connected = true
	m.ready = true
	m.msgs = m.startRecvLoop()
	m.log("Reconnected to %s", m.addr)

	// Request any open windows from Dyalog (restores orphaned editors)
	m.send("GetWindowLayout", map[string]any{})

	return m, waitForRide(m.msgs)
}

func (m *Model) log(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	m.debugLog.Lines = append(m.debugLog.Lines, line)
	if len(m.debugLog.Lines) > 500 {
		m.debugLog.Lines = m.debugLog.Lines[len(m.debugLog.Lines)-500:]
	}
	// Also write to log file if set
	if m.logFile != nil {
		ts := time.Now().Format("15:04:05.000")
		fmt.Fprintf(m.logFile, "[%s] %s\n", ts, line)
	}
}

// waitForRide waits for the next RIDE message.
func waitForRide(ch <-chan rideEvent) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m Model) Init() tea.Cmd {
	// Request any open windows from Dyalog (restores orphaned editors on reconnect)
	m.send("GetWindowLayout", map[string]any{})
	return waitForRide(m.msgs)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.panes.UpdateSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case APLcartLoaded:
		if pane := m.panes.Get("aplcart"); pane != nil {
			if ac, ok := pane.Content.(*APLcart); ok {
				ac.SetData(msg.Entries, msg.Err)
			}
		}
		return m, nil

	case rideEvent:
		return m.handleRide(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle autocomplete popup - must be first to intercept keys
	if m.acPopup != nil {
		switch msg.Type {
		case tea.KeyTab, tea.KeyDown:
			m.acPopup.CycleNext()
			return m, nil
		case tea.KeyShiftTab, tea.KeyUp:
			m.acPopup.CyclePrev()
			return m, nil
		case tea.KeyEnter:
			// Select and insert
			m.insertAutocomplete()
			return m, nil
		case tea.KeyEscape:
			// Cancel
			m.acPopup = nil
			return m, nil
		default:
			// Any other key cancels autocomplete and gets processed normally
			m.acPopup = nil
			// Fall through to normal key handling
		}
	}

	// Handle backtick mode - insert APL symbol
	// insertTarget routes to focused editor if one exists, otherwise session
	insertTarget := func(r rune) {
		if fp := m.panes.FocusedPane(); fp != nil {
			if ep, ok := fp.Content.(*EditorPane); ok && !ep.InTracerMode() {
				ep.insertChar(r)
				return
			}
		}
		m.insertChar(r)
	}

	if m.backtickActive {
		m.backtickActive = false
		if len(msg.Runes) > 0 {
			r := msg.Runes[0]
			if sym, ok := backtickMap[r]; ok {
				// Insert symbol at cursor
				insertTarget(sym)
				return m, nil
			}
			// Unknown key - insert the backtick and the key
			insertTarget('`')
			insertTarget(r)
			return m, nil
		}
		// Special key after backtick - just insert backtick
		insertTarget('`')
		return m, nil
	}

	// Check for backtick
	if len(msg.Runes) == 1 && msg.Runes[0] == '`' {
		m.backtickActive = true
		return m, nil
	}

	// Handle quit confirmation
	if m.confirmQuit {
		m.confirmQuit = false
		if msg.String() == "y" || msg.String() == "Y" {
			return m, tea.Quit
		}
		return m, nil
	}

	// Handle save prompt
	if m.savePromptActive {
		switch msg.Type {
		case tea.KeyEscape:
			m.savePromptActive = false
			m.log("Save cancelled")
			return m, nil
		case tea.KeyEnter:
			m.savePromptActive = false
			m.doSaveSession()
			return m, nil
		case tea.KeyBackspace:
			if len(m.savePromptFilename) > 0 {
				m.savePromptFilename = m.savePromptFilename[:len(m.savePromptFilename)-1]
			}
			return m, nil
		default:
			if len(msg.Runes) > 0 {
				m.savePromptFilename += string(msg.Runes)
			}
			return m, nil
		}
	}

	// Handle pane move mode
	if m.paneMoveMode {
		fp := m.panes.FocusedPane()
		if fp == nil {
			m.paneMoveMode = false
			return m, nil
		}

		step := 1
		k := msg.String()
		switch {
		case msg.Type == tea.KeyEscape || msg.Type == tea.KeyEnter:
			m.paneMoveMode = false
			return m, nil
		case k == "up":
			fp.Y = max(0, fp.Y-step)
			return m, nil
		case k == "shift+up":
			fp.Height = max(5, fp.Height-step)
			return m, nil
		case k == "down":
			fp.Y = min(m.height-fp.Height-1, fp.Y+step)
			return m, nil
		case k == "shift+down":
			fp.Height = min(m.height-2, fp.Height+step)
			return m, nil
		case k == "left":
			fp.X = max(0, fp.X-step)
			return m, nil
		case k == "shift+left":
			fp.Width = max(10, fp.Width-step)
			return m, nil
		case k == "right":
			fp.X = min(m.width-fp.Width-1, fp.X+step)
			return m, nil
		case k == "shift+right":
			fp.Width = min(m.width-2, fp.Width+step)
			return m, nil
		}
		return m, nil
	}

	// Handle leader key sequences
	if m.leaderActive {
		m.leaderActive = false // Reset on any key
		switch {
		case key.Matches(msg, m.keys.ToggleDebug):
			m.toggleDebugPane()
			return m, nil
		case key.Matches(msg, m.keys.ToggleStack):
			m.toggleStackPane()
			return m, nil
		case key.Matches(msg, m.keys.ToggleLocals):
			m.toggleVariablesPane()
			return m, nil
		case key.Matches(msg, m.keys.ToggleBreakpoint):
			m.toggleBreakpoint()
			return m, nil
		case key.Matches(msg, m.keys.Reconnect):
			return m.reconnect()
		case key.Matches(msg, m.keys.CommandPalette):
			m.openCommandPalette()
			return m, nil
		case key.Matches(msg, m.keys.PaneMoveMode):
			if m.panes.FocusedPane() != nil {
				m.paneMoveMode = true
			}
			return m, nil
		case key.Matches(msg, m.keys.ShowKeys):
			m.toggleKeysPane()
			return m, nil
		case key.Matches(msg, m.keys.CyclePane):
			if m.panes.HasPanes() {
				m.panes.FocusNext()
			}
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			m.confirmQuit = true
			return m, nil
		}
		// Unknown leader sequence - ignore
		return m, nil
	}

	// Check for leader key
	if key.Matches(msg, m.keys.Leader) {
		m.leaderActive = true
		return m, nil
	}

	// Clear quit hint on any key
	m.showQuitHint = false

	// Ctrl+C shows quit hint (vim style)
	if msg.Type == tea.KeyCtrlC {
		m.showQuitHint = true
		return m, nil
	}

	// Context-sensitive documentation
	if key.Matches(msg, m.keys.DocHelp) {
		return m.openDocHelp()
	}

	// Global shortcuts (always work regardless of focus)
	switch {

	case key.Matches(msg, m.keys.Autocomplete):
		// Trigger autocomplete request - works in session or editor (edit mode)
		if fp := m.panes.FocusedPane(); fp != nil {
			// Editor in edit mode - trigger autocomplete
			if ep, ok := fp.Content.(*EditorPane); ok && !ep.InTracerMode() {
				m.requestAutocomplete(ep.window.Token)
				return m, nil
			}
			// Other pane focused - autocomplete not applicable
		} else {
			// Session context - no panes focused
			m.requestAutocomplete(0)
			return m, nil
		}

	case key.Matches(msg, m.keys.ClosePane):
		if fp := m.panes.FocusedPane(); fp != nil {
			if fp.ID == "tracer" {
				// Check if tracer is in edit mode - if so, let the pane handle it
				if ep, ok := fp.Content.(*EditorPane); ok && ep.editMode {
					// Don't close - fall through to pane's HandleKey
					break
				}
				// Close current tracer - pops the stack
				if m.tracerCurrent != 0 {
					m.closeEditor(m.tracerCurrent)
				}
			} else if strings.HasPrefix(fp.ID, "editor:") {
				// Regular editor pane
				var token int
				fmt.Sscanf(fp.ID, "editor:%d", &token)
				m.closeEditor(token)
			} else {
				m.panes.Remove(fp.ID)
			}
		}
		return m, nil
	}

	// Route to focused pane - pane gets ALL keys when focused
	if fp := m.panes.FocusedPane(); fp != nil && fp.Content != nil {
		fp.Content.HandleKey(msg)

		// Check if command palette selected an action
		if cp, ok := fp.Content.(*CommandPalette); ok && cp.SelectedAction != "" {
			action := cp.SelectedAction
			cp.SelectedAction = ""
			m.panes.Remove("commands")
			// For breakpoint action, focus the tracer/editor pane first
			if action == "breakpoint" {
				if m.panes.Get("tracer") != nil {
					m.panes.Focus("tracer")
				}
			}
			return (&m).dispatchCommand(action)
		}

		// Check if symbol search selected a symbol
		if ss, ok := fp.Content.(*SymbolSearch); ok && ss.SelectedSymbol != 0 {
			sym := ss.SelectedSymbol
			ss.SelectedSymbol = 0
			m.panes.Remove("symbols")
			m.insertChar(sym)
			return m, nil
		}

		// Check if APLcart selected a syntax
		if ac, ok := fp.Content.(*APLcart); ok && ac.SelectedSyntax != "" {
			syntax := ac.SelectedSyntax
			ac.SelectedSyntax = ""
			m.panes.Remove("aplcart")
			// Insert the syntax
			for _, r := range syntax {
				m.insertChar(r)
			}
			return m, nil
		}

		// Check if variables pane needs refresh (after mode toggle)
		if vp, ok := fp.Content.(*VariablesPane); ok && vp.loading {
			m.fetchVariables(vp)
			return m, nil
		}

		return m, nil // Focused pane consumes all input
	}

	// Session key handling (only when no pane is focused)
	return m.handleSessionKey(msg)
}

func (m *Model) toggleKeysPane() {
	if m.panes.Get("keys") != nil {
		m.panes.Remove("keys")
	} else {
		// Center the pane
		paneW := 40
		paneH := 25
		paneX := (m.width - paneW) / 2
		paneY := (m.height - paneH) / 2
		if paneX < 0 {
			paneX = 0
		}
		if paneY < 0 {
			paneY = 0
		}

		keysPane := NewKeysPane(m.keys)
		pane := NewPane("keys", keysPane, paneX, paneY, paneW, paneH)
		m.panes.Add(pane)
		m.panes.Focus("keys")
	}
}

func (m *Model) toggleDebugPane() {
	if m.panes.Get("debug") != nil {
		m.panes.Remove("debug")
		m.debugPane = nil
	} else {
		// Calculate available height (account for help line and session border)
		helpHeight := 1
		if m.help.ShowAll {
			helpHeight = 4
		}
		availH := m.height - helpHeight - 2 // -2 for session top/bottom borders

		paneW := 50
		paneH := availH - 2 // Leave margin
		if paneH < 10 {
			paneH = 10
		}
		paneX := m.width - paneW - 2
		if paneX < 0 {
			paneX = 0
		}

		m.debugPane = NewDebugPane(m.debugLog)
		pane := NewPane("debug", m.debugPane, paneX, 1, paneW, paneH)
		m.panes.Add(pane)
		m.panes.Focus("debug")
	}
}

func (m Model) handleSessionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		return m.execute()

	case tea.KeyBackspace:
		m.deleteCharBack()
		return m, nil

	case tea.KeyDelete:
		m.deleteCharForward()
		return m, nil

	case tea.KeyLeft:
		if m.cursorCol > 0 {
			m.cursorCol--
		}
		return m, nil

	case tea.KeyRight:
		if m.cursorCol < len(m.currentLineRunes()) {
			m.cursorCol++
		}
		return m, nil

	case tea.KeyUp:
		if m.cursorRow > 0 {
			m.cursorRow--
			m.clampCol()
		}
		return m, nil

	case tea.KeyDown:
		if m.cursorRow < len(m.lines)-1 {
			m.cursorRow++
			m.clampCol()
		}
		return m, nil

	case tea.KeyHome:
		m.cursorCol = 0
		return m, nil

	case tea.KeyEnd:
		m.cursorCol = len(m.currentLineRunes())
		return m, nil

	case tea.KeyPgUp:
		pageSize := m.height - 4
		if pageSize < 1 {
			pageSize = 10
		}
		m.cursorRow -= pageSize
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		m.clampCol()
		return m, nil

	case tea.KeyPgDown:
		pageSize := m.height - 4
		if pageSize < 1 {
			pageSize = 10
		}
		m.cursorRow += pageSize
		if m.cursorRow >= len(m.lines) {
			m.cursorRow = len(m.lines) - 1
		}
		m.clampCol()
		return m, nil

	case tea.KeySpace:
		m.insertChar(' ')
		return m, nil

	default:
		if len(msg.Runes) > 0 {
			for _, r := range msg.Runes {
				m.insertChar(r)
			}
		}
		return m, nil
	}
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Check if any pane is being dragged
	for _, id := range m.panes.zOrder {
		pane := m.panes.panes[id]
		if pane != nil && pane.dragging {
			switch msg.Type {
			case tea.MouseMotion:
				pane.UpdateDrag(msg.X, msg.Y, m.panes.screenW, m.panes.screenH)
				return m, nil
			case tea.MouseRelease:
				pane.StopDrag()
				return m, nil
			}
		}
	}

	// Hit test for pane interactions
	pane := m.panes.PaneAt(msg.X, msg.Y)

	switch msg.Type {
	case tea.MouseLeft:
		if pane != nil {
			zone := pane.HitZone(msg.X, msg.Y)
			switch zone {
			case ZoneTitleBar:
				m.panes.Focus(pane.ID)
				pane.StartDrag(DragMove, msg.X, msg.Y)
			case ZoneEdgeN, ZoneEdgeS, ZoneEdgeE, ZoneEdgeW,
				ZoneCornerNE, ZoneCornerNW, ZoneCornerSE, ZoneCornerSW:
				m.panes.Focus(pane.ID)
				pane.StartDrag(zoneToDragMode(zone), msg.X, msg.Y)
			case ZoneContent:
				m.panes.Focus(pane.ID)
				// Pass click to pane content (relative coords)
				if pane.Content != nil {
					pane.Content.HandleMouse(msg.X-pane.X-1, msg.Y-pane.Y-1, msg)
				}
			}
		} else {
			// Click on session - unfocus panes
			if fp := m.panes.FocusedPane(); fp != nil {
				fp.Focused = false
				m.panes.focusedID = ""
			}
		}
		return m, nil

	case tea.MouseWheelUp, tea.MouseWheelDown:
		if pane != nil && pane.Content != nil {
			// Pass scroll to pane
			pane.Content.HandleMouse(msg.X-pane.X-1, msg.Y-pane.Y-1, msg)
		} else {
			// Scroll session
			if msg.Type == tea.MouseWheelUp {
				m.cursorRow -= 3
				if m.cursorRow < 0 {
					m.cursorRow = 0
				}
			} else {
				m.cursorRow += 3
				if m.cursorRow >= len(m.lines) {
					m.cursorRow = len(m.lines) - 1
				}
			}
			m.clampCol()
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) currentLine() string {
	if m.cursorRow >= 0 && m.cursorRow < len(m.lines) {
		return m.lines[m.cursorRow].Text
	}
	return ""
}

func (m *Model) currentLineRunes() []rune {
	return []rune(m.currentLine())
}

func (m *Model) setCurrentLine(text string) {
	if m.cursorRow < 0 || m.cursorRow >= len(m.lines) {
		return
	}
	// Mark as edited on first modification
	if !m.lines[m.cursorRow].Edited {
		m.lines[m.cursorRow].Original = m.lines[m.cursorRow].Text
		m.lines[m.cursorRow].Edited = true
	}
	m.lines[m.cursorRow].Text = text
}

func (m *Model) insertChar(r rune) {
	runes := m.currentLineRunes()
	newRunes := make([]rune, 0, len(runes)+1)
	newRunes = append(newRunes, runes[:m.cursorCol]...)
	newRunes = append(newRunes, r)
	newRunes = append(newRunes, runes[m.cursorCol:]...)
	m.setCurrentLine(string(newRunes))
	m.cursorCol++
}

func (m *Model) deleteCharBack() {
	if m.cursorCol > 0 {
		runes := m.currentLineRunes()
		newRunes := make([]rune, 0, len(runes)-1)
		newRunes = append(newRunes, runes[:m.cursorCol-1]...)
		newRunes = append(newRunes, runes[m.cursorCol:]...)
		m.setCurrentLine(string(newRunes))
		m.cursorCol--
	}
}

func (m *Model) deleteCharForward() {
	runes := m.currentLineRunes()
	if m.cursorCol < len(runes) {
		newRunes := make([]rune, 0, len(runes)-1)
		newRunes = append(newRunes, runes[:m.cursorCol]...)
		newRunes = append(newRunes, runes[m.cursorCol+1:]...)
		m.setCurrentLine(string(newRunes))
	}
}

func (m *Model) clampCol() {
	lineLen := len(m.currentLineRunes())
	if m.cursorCol > lineLen {
		m.cursorCol = lineLen
	}
}

func (m Model) execute() (tea.Model, tea.Cmd) {
	if !m.ready {
		m.log("Execute blocked: not ready")
		return m, nil
	}

	editedText := m.currentLine()
	code := strings.TrimSpace(editedText)
	isInputLine := m.cursorRow == len(m.lines)-1

	// Empty line - just add a new input line for spacing
	if code == "" {
		if isInputLine {
			// Keep current line, add new input line
			m.lines = append(m.lines, Line{Text: aplIndent})
			m.cursorRow = len(m.lines) - 1
			m.cursorCol = len(aplIndent)
		}
		// If on history line with no edits, do nothing
		return m, nil
	}

	if isInputLine {
		// On the input line - keep it as typed
		m.lines[m.cursorRow].Text = editedText
		m.lines[m.cursorRow].Edited = false
		m.lines[m.cursorRow].Original = ""
	} else {
		// On a history line - restore it and replace the input line (last line)
		if m.lines[m.cursorRow].Edited {
			m.lines[m.cursorRow].Text = m.lines[m.cursorRow].Original
			m.lines[m.cursorRow].Edited = false
			m.lines[m.cursorRow].Original = ""
		}
		// Replace the last line (input line) with new input
		lastIdx := len(m.lines) - 1
		m.lines[lastIdx].Text = editedText
		m.lines[lastIdx].Edited = false
		m.lines[lastIdx].Original = ""
		m.cursorRow = lastIdx
	}
	m.cursorCol = len([]rune(editedText))

	m.ready = false
	m.lastExecute = editedText + "\n" // Track what we sent to skip our own echo
	m.pendingQuit = strings.TrimSpace(editedText) == ")off"
	m.log("→ Execute %q", editedText)

	// Send to interpreter
	if err := m.send("Execute", map[string]any{"text": m.lastExecute, "trace": 0}); err != nil {
		// Disconnect handled by send(), just return
		return m, nil
	}

	return m, nil
}

func (m *Model) saveEditor(token int) {
	w, exists := m.editors[token]
	if !exists {
		return
	}

	// Build text array
	text := make([]any, len(w.Text))
	for i, line := range w.Text {
		text[i] = line
	}

	// Build stop array (breakpoints)
	stop := make([]any, len(w.Stop))
	for i, s := range w.Stop {
		stop[i] = s
	}

	// Build monitor array
	monitor := make([]any, len(w.Monitor))
	for i, s := range w.Monitor {
		monitor[i] = s
	}

	// Build trace array
	trace := make([]any, len(w.Trace))
	for i, s := range w.Trace {
		trace[i] = s
	}

	m.log("→ SaveChanges win=%d", token)

	m.send("SaveChanges", map[string]any{
		"win":     token,
		"text":    text,
		"stop":    stop,
		"monitor": monitor,
		"trace":   trace,
	})
}

// sendSetLineAttributes sends breakpoint state for tracer windows (immediate update)
func (m *Model) sendSetLineAttributes(token int) {
	w, exists := m.editors[token]
	if !exists {
		return
	}

	// Build stop array (breakpoints)
	stop := make([]any, len(w.Stop))
	for i, s := range w.Stop {
		stop[i] = s
	}

	// Build monitor array
	monitor := make([]any, len(w.Monitor))
	for i, s := range w.Monitor {
		monitor[i] = s
	}

	// Build trace array
	trace := make([]any, len(w.Trace))
	for i, s := range w.Trace {
		trace[i] = s
	}

	m.log("→ SetLineAttributes win=%d stop=%v", token, w.Stop)

	m.send("SetLineAttributes", map[string]any{
		"win":     token,
		"stop":    stop,
		"monitor": monitor,
		"trace":   trace,
	})
}

// Tracer control methods

func (m *Model) tracerStepInto() {
	if m.tracerCurrent == 0 {
		return
	}
	m.log("→ StepInto win=%d", m.tracerCurrent)
	m.send("StepInto", map[string]any{"win": m.tracerCurrent})
}

func (m *Model) tracerStepOver() {
	if m.tracerCurrent == 0 {
		return
	}
	m.log("→ RunCurrentLine win=%d", m.tracerCurrent)
	m.send("RunCurrentLine", map[string]any{"win": m.tracerCurrent})
}

func (m *Model) tracerStepOut() {
	if m.tracerCurrent == 0 {
		return
	}
	m.log("→ ContinueTrace win=%d", m.tracerCurrent)
	m.send("ContinueTrace", map[string]any{"win": m.tracerCurrent})
}

func (m *Model) tracerContinue() {
	if m.tracerCurrent == 0 {
		return
	}
	m.log("→ Continue win=%d", m.tracerCurrent)
	m.send("Continue", map[string]any{"win": m.tracerCurrent})
}

func (m *Model) tracerResumeAll() {
	m.log("→ RestartThreads")
	m.send("RestartThreads", map[string]any{})
}

func (m *Model) tracerBackward() {
	if m.tracerCurrent == 0 {
		return
	}
	m.log("→ TraceBackward win=%d", m.tracerCurrent)
	m.send("TraceBackward", map[string]any{"win": m.tracerCurrent})
}

func (m *Model) tracerForward() {
	if m.tracerCurrent == 0 {
		return
	}
	m.log("→ TraceForward win=%d", m.tracerCurrent)
	m.send("TraceForward", map[string]any{"win": m.tracerCurrent})
}

// closeAllWindows sends CloseAllWindows to Dyalog to close all editor/tracer windows
// Useful for clearing stuck state after a crash or disconnect
func (m *Model) closeAllWindows() {
	m.log("→ CloseAllWindows")
	m.send("CloseAllWindows", map[string]any{})
	// Clear local state
	m.editors = make(map[int]*EditorWindow)
	m.tracerStack = nil
	m.tracerCurrent = 0
	// Remove panes
	m.panes.Remove("editor")
	m.panes.Remove("tracer")
	m.panes.Remove("stack")
}

func (m *Model) closeEditor(token int) {
	w, exists := m.editors[token]
	if !exists {
		return
	}

	// If modified, save first and wait for ReplySaveChanges before closing
	if w.Modified {
		w.PendingClose = true
		m.saveEditor(token)
		m.log("  (waiting for ReplySaveChanges before CloseWindow)")
		return
	}

	// Not modified - close immediately
	m.sendCloseWindow(token)
}

func (m *Model) sendCloseWindow(token int) {
	m.log("→ CloseWindow win=%d", token)

	m.send("CloseWindow", map[string]any{
		"win": token,
	})
	// Don't remove pane yet - wait for CloseWindow from Dyalog
}

// Tracer stack management

func (m *Model) isInTracerStack(token int) bool {
	for _, t := range m.tracerStack {
		if t == token {
			return true
		}
	}
	return false
}

func (m *Model) removeFromTracerStack(token int) {
	// Remove token from stack
	for i, t := range m.tracerStack {
		if t == token {
			m.tracerStack = append(m.tracerStack[:i], m.tracerStack[i+1:]...)
			break
		}
	}

	// If we removed the current tracer, switch to new top of stack
	if m.tracerCurrent == token {
		if len(m.tracerStack) > 0 {
			// Show the new top of stack
			m.showTracer(m.tracerStack[len(m.tracerStack)-1])
		} else {
			// Stack empty - hide tracer pane
			m.tracerCurrent = 0
			m.panes.Remove("tracer")
		}
	}
}

func (m *Model) showTracer(token int) {
	m.tracerCurrent = token

	w, exists := m.editors[token]
	if !exists {
		return
	}

	// Check if tracer pane exists
	if pane := m.panes.Get("tracer"); pane != nil {
		// Update existing pane's window
		if ep, ok := pane.Content.(*EditorPane); ok {
			ep.SetWindow(w)
		}
	} else {
		// Create tracer pane
		editorPane := NewEditorPane(w, m.config.TracerKeys,
			func() { m.saveEditor(m.tracerCurrent) },
			func() { m.closeEditor(m.tracerCurrent) },
		)

		// Set tracer control callbacks
		editorPane.SetTracerCallbacks(TracerCallbacks{
			StepInto:  func() { m.tracerStepInto() },
			StepOver:  func() { m.tracerStepOver() },
			StepOut:   func() { m.tracerStepOut() },
			Continue:  func() { m.tracerContinue() },
			ResumeAll: func() { m.tracerResumeAll() },
			Backward:  func() { m.tracerBackward() },
			Forward:   func() { m.tracerForward() },
		})

		// Position: center of screen
		paneW := min(m.width-4, 60)
		paneH := min(m.height-6, 20)
		if paneW < 30 {
			paneW = 30
		}
		if paneH < 10 {
			paneH = 10
		}
		paneX := (m.width - paneW) / 2
		paneY := (m.height - paneH) / 2

		pane := NewPane("tracer", editorPane, paneX, paneY, paneW, paneH)
		m.panes.Add(pane)
		m.panes.Focus("tracer")
	}
}

func (m *Model) getStackFrames() []StackFrame {
	frames := make([]StackFrame, 0, len(m.tracerStack))
	for _, token := range m.tracerStack {
		if w, exists := m.editors[token]; exists {
			code := ""
			if w.CurrentRow >= 0 && w.CurrentRow < len(w.Text) {
				code = strings.TrimSpace(w.Text[w.CurrentRow])
			}
			frames = append(frames, StackFrame{
				Token:   token,
				Name:    w.Name,
				Line:    w.CurrentRow,
				Code:    code,
				Current: token == m.tracerCurrent,
			})
		}
	}
	return frames
}

func (m *Model) toggleStackPane() {
	if m.panes.Get("stack") != nil {
		m.panes.Remove("stack")
		return
	}

	// Create stack pane
	stackPane := NewStackPane(
		func() []StackFrame { return m.getStackFrames() },
		func(token int) { m.showTracer(token) },
	)

	// Position: right side of screen
	paneW := 30
	paneH := min(m.height-4, 15)
	if paneH < 5 {
		paneH = 5
	}
	paneX := m.width - paneW - 2
	paneY := 2

	pane := NewPane("stack", stackPane, paneX, paneY, paneW, paneH)
	m.panes.Add(pane)
	m.panes.Focus("stack")
}

func (m *Model) toggleVariablesPane() {
	if p := m.panes.Get("variables"); p != nil {
		// If already open but not focused, focus it; otherwise close
		if m.panes.FocusedPane() == p {
			m.panes.Remove("variables")
		} else {
			m.panes.Focus("variables")
		}
		return
	}

	// Create variables pane with callbacks
	var varsPane *VariablesPane
	varsPane = NewVariablesPane(
		func(name string) {
			// Open variable with )ed - use send not Execute to avoid race
			m.send("Execute", map[string]any{"text": ")ed " + name + "\n", "trace": 0})
		},
		func(mode VarsMode) {
			// Re-fetch when mode changes
			m.fetchVariables(varsPane)
		},
	)

	// Fetch variables if we're in a tracer
	m.fetchVariables(varsPane)

	// Position: right side of screen, below stack pane if present
	paneW := 35
	paneH := min(m.height-4, 12)
	if paneH < 5 {
		paneH = 5
	}
	paneX := m.width - paneW - 2
	paneY := 2

	// If stack pane is open, position below it
	if stackPane := m.panes.Get("stack"); stackPane != nil {
		paneY = stackPane.Y + stackPane.Height + 1
	}

	pane := NewPane("variables", varsPane, paneX, paneY, paneW, paneH)
	m.panes.Add(pane)
	m.panes.Focus("variables")
}

// fetchVariables fetches variables and populates the variables pane
func (m *Model) fetchVariables(pane *VariablesPane) {
	if m.client == nil {
		pane.Clear()
		return
	}

	pane.SetLoading(true)

	// For locals mode in tracer, we can extract names from source without querying
	if len(m.tracerStack) > 0 && pane.Mode() == VarsModeLocals {
		w, exists := m.editors[m.tracerCurrent]
		if !exists {
			pane.Clear()
			return
		}

		// Parse function source for assignments (name←)
		var varNames []string
		assignedVars := make(map[string]bool)
		for _, line := range w.Text {
			// Look for assignment pattern: name←
			for i := 0; i < len(line); i++ {
				// Find ← and extract preceding name
				if i+3 <= len(line) && line[i:i+3] == "←" {
					// Walk back to find variable name start
					end := i
					start := end - 1
					for start >= 0 {
						c := line[start]
						if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
							(c >= '0' && c <= '9') || c == '_' ||
							c >= 0x80 { // Allow APL characters
							start--
						} else {
							break
						}
					}
					start++
					if start < end {
						name := line[start:end]
						assignedVars[name] = true
					}
				}
			}
		}

		if len(assignedVars) == 0 {
			pane.Clear()
			return
		}

		for name := range assignedVars {
			varNames = append(varNames, name)
		}
		sort.Strings(varNames)
		// In locals mode, all vars shown are local
		m.fetchVarValuesInternal(pane, varNames, assignedVars)
		return
	}

	// For all mode (tracer or session): query ⎕NL 2 to get variable names
	// First, get local variable names from function header (if in tracer)
	localVars := make(map[string]bool)
	if len(m.tracerStack) > 0 {
		if w, exists := m.editors[m.tracerCurrent]; exists && len(w.Text) > 0 {
			// Parse header line: "FnName;local1;local2;..."
			header := w.Text[0]
			parts := strings.Split(header, ";")
			for i := 1; i < len(parts); i++ { // Skip function name (first part)
				local := strings.TrimSpace(parts[i])
				if local != "" {
					localVars[local] = true
				}
			}
		}
	}

	// Single query: get names and values in one shot
	// {⎕←⍵,'=',⍕⍎⍵}¨↓⎕NL 2 - for each name from ⎕NL 2, print name=value
	m.executeInternal("{⎕←⍵,'=',⍕⍎⍵}¨↓⎕NL 2", func(outputs []string) {
		var vars []LocalVar
		for _, output := range outputs {
			for _, line := range strings.Split(output, "\n") {
				line = strings.TrimSpace(line)
				if idx := strings.Index(line, "="); idx > 0 {
					name := strings.TrimSpace(line[:idx])
					value := strings.TrimSpace(line[idx+1:])
					if nl := strings.Index(value, "\n"); nl != -1 {
						value = value[:nl] + "..."
					}
					vars = append(vars, LocalVar{Name: name, Value: value, IsLocal: localVars[name]})
				}
			}
		}
		if len(vars) == 0 {
			pane.Clear()
			return
		}
		sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
		pane.SetVars(vars)
	})
}

// fetchVarValuesInternal fetches all variable values in one APL query
// locals is a set of variable names declared as local in the function header
func (m *Model) fetchVarValuesInternal(pane *VariablesPane, names []string, locals map[string]bool) {
	if len(names) == 0 {
		pane.Clear()
		return
	}

	// Build APL expression: {⎕←⍵,'=',⍕⍎⍵}¨'a' 'b' 'c'
	// This prints each name=value on its own line
	var quotedNames []string
	for _, name := range names {
		quotedNames = append(quotedNames, "'"+name+"'")
	}
	expr := "{⎕←⍵,'=',⍕⍎⍵}¨" + strings.Join(quotedNames, " ")

	m.executeInternal(expr, func(outputs []string) {
		// Parse name=value lines from output
		valueMap := make(map[string]string)
		for _, output := range outputs {
			for _, line := range strings.Split(output, "\n") {
				line = strings.TrimSpace(line)
				if idx := strings.Index(line, "="); idx > 0 {
					name := strings.TrimSpace(line[:idx])
					value := strings.TrimSpace(line[idx+1:])
					valueMap[name] = value
				}
			}
		}

		// Build vars list with values
		var vars []LocalVar
		for _, name := range names {
			value := valueMap[name]
			// Truncate multi-line values for display
			if nl := strings.Index(value, "\n"); nl != -1 {
				value = value[:nl] + "..."
			}
			vars = append(vars, LocalVar{Name: name, Value: value, IsLocal: locals[name]})
		}
		pane.SetVars(vars)
	})
}

// Autocomplete methods

// requestAutocomplete sends a GetAutocomplete request for the current line/position
// token is 0 for session, or window token for editor
func (m *Model) requestAutocomplete(token int) {
	line, pos := m.getAutocompleteContext(token)
	if line == "" || pos == 0 {
		return
	}

	// Check if cursor follows APL name characters
	if !m.shouldTriggerAutocomplete(line, pos) {
		return
	}

	m.acPending = true
	m.log("→ GetAutocomplete line=%q pos=%d token=%d", line, pos, token)
	m.send("GetAutocomplete", map[string]any{
		"line":  line,
		"pos":   pos,
		"token": token,
	})
}

// shouldTriggerAutocomplete checks if cursor position is valid for autocomplete
// (cursor must follow APL name characters)
func (m *Model) shouldTriggerAutocomplete(line string, pos int) bool {
	if pos == 0 || pos > len(line) {
		return false
	}

	// Get character before cursor
	runes := []rune(line)
	if pos > len(runes) {
		return false
	}

	// Check if previous character is an APL name character
	prevChar := runes[pos-1]
	return isAPLNameChar(prevChar)
}

// isAPLNameChar returns true if the rune is valid in an APL name
func isAPLNameChar(r rune) bool {
	// APL names can contain: A-Z, a-z, 0-9, _, ⎕, ∆, ⍙
	if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r == '_' || r == '⎕' || r == '∆' || r == '⍙' {
		return true
	}
	return false
}

// getAutocompleteContext returns the line text and cursor position for autocomplete
func (m *Model) getAutocompleteContext(token int) (string, int) {
	if token == 0 {
		// Session context
		return m.currentLine(), m.cursorCol
	}

	// Editor context - find the editor
	if w, exists := m.editors[token]; exists {
		if w.CursorRow >= 0 && w.CursorRow < len(w.Text) {
			return w.Text[w.CursorRow], w.CursorCol
		}
	}

	return "", 0
}

// showAutocomplete displays the autocomplete popup with options
func (m *Model) showAutocomplete(options []string, skip, token, triggerCol int) {
	m.acPopup = NewAutocomplete(options, skip, token, triggerCol)
}

// insertAutocomplete inserts the selected completion option
func (m *Model) insertAutocomplete() {
	if m.acPopup == nil {
		return
	}

	option := m.acPopup.SelectedOption()
	skip := m.acPopup.Skip
	token := m.acPopup.Token
	triggerCol := m.acPopup.TriggerCol

	m.log("  inserting completion: %s (skip=%d, triggerCol=%d)", option, skip, triggerCol)

	if token == 0 {
		// Session context - replace from triggerCol-skip to triggerCol
		runes := m.currentLineRunes()
		replaceStart := triggerCol - skip
		if replaceStart < 0 {
			replaceStart = 0
		}

		newRunes := make([]rune, 0, len(runes)-skip+len([]rune(option)))
		newRunes = append(newRunes, runes[:replaceStart]...)
		newRunes = append(newRunes, []rune(option)...)
		newRunes = append(newRunes, runes[triggerCol:]...)

		m.setCurrentLine(string(newRunes))
		m.cursorCol = replaceStart + len([]rune(option))
	} else {
		// Editor context
		if w, exists := m.editors[token]; exists {
			if w.CursorRow >= 0 && w.CursorRow < len(w.Text) {
				runes := []rune(w.Text[w.CursorRow])
				replaceStart := triggerCol - skip
				if replaceStart < 0 {
					replaceStart = 0
				}

				newRunes := make([]rune, 0, len(runes)-skip+len([]rune(option)))
				newRunes = append(newRunes, runes[:replaceStart]...)
				newRunes = append(newRunes, []rune(option)...)
				newRunes = append(newRunes, runes[triggerCol:]...)

				w.Text[w.CursorRow] = string(newRunes)
				w.CursorCol = replaceStart + len([]rune(option))
				w.Modified = true
			}
		}
	}

	m.acPopup = nil
}

// renderAutocompleteOverlay renders the autocomplete popup over the base content
func (m *Model) renderAutocompleteOverlay(base string, screenW, screenH int) string {
	if m.acPopup == nil {
		return base
	}

	popup := m.acPopup.Render(40, screenH-4)
	popupLines := strings.Split(popup, "\n")
	popupH := len(popupLines)
	popupW := 0
	for _, line := range popupLines {
		if lipgloss.Width(line) > popupW {
			popupW = lipgloss.Width(line)
		}
	}

	// Position popup at top-right (same position as stack pane)
	popupX := screenW - popupW - 2
	popupY := 2
	if popupX < 0 {
		popupX = 0
	}

	// Use cellbuf for ANSI-aware compositing
	baseLines := strings.Split(base, "\n")
	baseH := len(baseLines)

	buf := cellbuf.NewBuffer(screenW, baseH)
	cellbuf.SetContent(buf, base)

	// Create popup buffer and overlay it
	popupBuf := cellbuf.NewBuffer(popupW, popupH)
	cellbuf.SetContent(popupBuf, popup)

	for dy := 0; dy < popupH; dy++ {
		y := popupY + dy
		if y < 0 || y >= baseH {
			continue
		}
		for dx := 0; dx < popupW; dx++ {
			x := popupX + dx
			if x < 0 || x >= screenW {
				continue
			}
			cell := popupBuf.Cell(dx, dy)
			if cell != nil {
				buf.SetCell(x, y, cell)
			}
		}
	}

	return cellbuf.Render(buf)
}

// isTracerFocused returns true if the focused pane is a tracer in trace mode (not edit mode)
func (m *Model) isTracerFocused() bool {
	fp := m.panes.FocusedPane()
	if fp == nil {
		return false
	}
	ep, ok := fp.Content.(*EditorPane)
	if !ok {
		return false
	}
	return ep.InTracerMode()
}

// toggleBreakpoint toggles a breakpoint on the current line in the focused editor/tracer
func (m *Model) toggleBreakpoint() {
	fp := m.panes.FocusedPane()
	if fp == nil {
		return
	}

	ep, ok := fp.Content.(*EditorPane)
	if !ok {
		return
	}

	// Toggle the breakpoint
	ep.window.ToggleStop(ep.window.CursorRow)

	// Send immediately so breakpoint takes effect without requiring save
	m.sendSetLineAttributes(ep.window.Token)
}

func (m *Model) dispatchCommand(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "debug":
		m.toggleDebugPane()
	case "stack":
		m.toggleStackPane()
	case "variables":
		m.toggleVariablesPane()
	case "breakpoint":
		m.toggleBreakpoint()
	case "keys":
		m.toggleKeysPane()
	case "symbols":
		m.openSymbolSearch()
	case "aplcart":
		return m.openAPLcart()
	case "reconnect":
		return m.reconnect()
	case "save":
		m.saveSession()
	case "quit":
		m.confirmQuit = true
	// Tracer controls
	case "step-into":
		m.tracerStepInto()
	case "step-over":
		m.tracerStepOver()
	case "step-out":
		m.tracerStepOut()
	case "continue":
		m.tracerContinue()
	case "resume-all":
		m.tracerResumeAll()
	case "trace-back":
		m.tracerBackward()
	case "trace-forward":
		m.tracerForward()
	case "close-all-windows":
		m.closeAllWindows()
	}
	return *m, nil
}

func (m *Model) openSymbolSearch() {
	if m.panes.Get("symbols") != nil {
		m.panes.Remove("symbols")
		return
	}

	ss := NewSymbolSearch()

	// Position: center
	paneW := 50
	paneH := min(20, m.height-4)
	paneX := (m.width - paneW) / 2
	paneY := (m.height - paneH) / 2

	pane := NewPane("symbols", ss, paneX, paneY, paneW, paneH)
	m.panes.Add(pane)
	m.panes.Focus("symbols")
}

func (m *Model) openDocHelp() (tea.Model, tea.Cmd) {
	// Toggle off if already open
	if m.panes.Get("docs") != nil {
		m.panes.Remove("docs")
		return *m, nil
	}

	if m.docsDB == nil {
		m.log("No docs database (run bundle-docs, copy to ~/.config/gritt/dyalog-docs.db)")
		return *m, nil
	}

	// Get symbol at cursor (to the left of cursor position)
	symbol := m.symbolAtCursor()
	if symbol == "" {
		m.log("No symbol at cursor")
		return *m, nil
	}

	// Look up in help_urls
	var navPath string
	err := m.docsDB.QueryRow("SELECT path FROM help_urls WHERE symbol = ?", symbol).Scan(&navPath)
	if err != nil {
		m.log("No help for %q", symbol)
		return *m, nil
	}

	// Fetch content
	var file, content string
	err = m.docsDB.QueryRow("SELECT file, content FROM docs WHERE path = ?", navPath).Scan(&file, &content)
	if err != nil {
		m.log("Doc not found: %s", navPath)
		return *m, nil
	}

	// Open pane
	paneW := min(90, m.width-4)
	paneH := min(35, m.height-4)
	paneX := (m.width - paneW) / 2
	paneY := (m.height - paneH) / 2

	processed, links := processLinks(content, file)
	rendered := RenderMarkdown(processed, paneW-2)
	doc := NewDocPane(navPath, file, rendered, links, m.docsDB, paneW-2)
	pane := NewPane("docs", doc, paneX, paneY, paneW, paneH)
	m.panes.Add(pane)
	m.panes.Focus("docs")

	return *m, nil
}

// symbolAtCursor returns the APL symbol or keyword at/before the cursor.
func (m *Model) symbolAtCursor() string {
	runes := m.currentLineRunes()
	col := m.cursorCol

	// Look at the character to the left of the cursor
	if col <= 0 || col > len(runes) {
		return ""
	}

	r := runes[col-1]

	// Single-character APL symbol
	if r > 127 || strings.ContainsRune("+-×÷*!?|<>=≠≤≥∨∧~,./\\@&#;:()[]{}", r) {
		return string(r)
	}

	return ""
}

func (m *Model) openAPLcart() (tea.Model, tea.Cmd) {
	if m.panes.Get("aplcart") != nil {
		m.panes.Remove("aplcart")
		return *m, nil
	}

	ac := NewAPLcart()

	// Position: center, larger
	paneW := 70
	paneH := min(25, m.height-4)
	paneX := (m.width - paneW) / 2
	paneY := (m.height - paneH) / 2

	pane := NewPane("aplcart", ac, paneX, paneY, paneW, paneH)
	m.panes.Add(pane)
	m.panes.Focus("aplcart")

	// Start fetching data
	return *m, FetchAPLcart
}

func (m *Model) saveSession() {
	m.savePromptActive = true
	m.savePromptFilename = fmt.Sprintf("session-%s", time.Now().Format("20060102-150405"))
}

func (m *Model) doSaveSession() {
	filename := m.savePromptFilename
	if filename == "" {
		m.log("Save cancelled")
		return
	}
	var sb strings.Builder
	for _, line := range m.lines {
		sb.WriteString(line.Text)
		sb.WriteString("\n")
	}
	if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		m.log("Failed to save session: %v", err)
	} else {
		m.log("Session saved to %s", filename)
	}
}

func (m *Model) openCommandPalette() {
	// Remove existing palette if open
	if m.panes.Get("commands") != nil {
		m.panes.Remove("commands")
		return
	}

	// Build command list
	commands := []Command{
		{Name: "debug", Help: "Toggle debug pane"},
		{Name: "stack", Help: "Toggle stack pane"},
		{Name: "variables", Help: "Toggle variables pane (tracer)"},
		{Name: "breakpoint", Help: "Toggle breakpoint on current line"},
		{Name: "step-into", Help: "Tracer: step into (Enter)"},
		{Name: "step-over", Help: "Tracer: step over (n)"},
		{Name: "step-out", Help: "Tracer: step out (o)"},
		{Name: "continue", Help: "Tracer: continue (c)"},
		{Name: "resume-all", Help: "Tracer: resume all threads (r)"},
		{Name: "trace-back", Help: "Tracer: move back (p)"},
		{Name: "trace-forward", Help: "Tracer: move forward (f)"},
		{Name: "keys", Help: "Show key bindings"},
		{Name: "symbols", Help: "Search APL symbols"},
		{Name: "aplcart", Help: "Search APLcart idioms"},
		{Name: "reconnect", Help: "Reconnect to Dyalog"},
		{Name: "close-all-windows", Help: "Close all editors/tracers (clear stuck state)"},
		{Name: "save", Help: "Save session to file"},
		{Name: "quit", Help: "Quit gritt"},
	}

	palette := NewCommandPalette(commands)

	// Position: center top
	paneW := 40
	paneH := min(len(commands)+3, 15)
	paneX := (m.width - paneW) / 2
	paneY := 2

	pane := NewPane("commands", palette, paneX, paneY, paneW, paneH)
	m.panes.Add(pane)
	m.panes.Focus("commands")
}

func (m Model) handleRide(ev rideEvent) (tea.Model, tea.Cmd) {
	if ev.err != nil {
		m.connected = false
		m.ready = false

		// If last command was )off, this is intentional shutdown - exit cleanly
		if m.pendingQuit {
			m.log("Session ended with )off")
			return m, tea.Quit
		}

		m.log("Disconnected: %v", ev.err)
		// Append visible disconnect marker to session (with blank line after)
		m.lines = append(m.lines, Line{Text: "⍝ Disconnected"})
		m.lines = append(m.lines, Line{Text: ""})
		m.cursorRow = len(m.lines) - 1
		m.cursorCol = 0
		// Don't quit - keep UI alive so user can view logs, reconnect, etc.
		return m, nil
	}

	if ev.msg == nil {
		if ev.raw != "" {
			m.log("← raw: %s", ev.raw)
		}
		return m, waitForRide(m.msgs)
	}

	msg := ev.msg

	// Log full message for debugging
	if argsJSON, err := json.Marshal(msg.Args); err == nil {
		m.log("← %s %s", msg.Command, string(argsJSON))
	} else {
		m.log("← %s %v", msg.Command, msg.Args)
	}

	switch msg.Command {
	case "AppendSessionOutput":
		// Skip input echo (type 14) only if it matches what we sent
		if t, ok := msg.Args["type"].(float64); ok && int(t) == 14 {
			if result, ok := msg.Args["result"].(string); ok && result == m.lastExecute {
				m.log("  (skipped: our input echo)")
				m.lastExecute = "" // Clear after matching
				return m, waitForRide(m.msgs)
			}
			// Skip internal query echo
			if m.internalQuery != "" {
				if result, ok := msg.Args["result"].(string); ok && result == m.internalQuery+"\n" {
					m.log("  (skipped: internal query echo)")
					return m, waitForRide(m.msgs)
				}
			}
			// Skip )off from external input - just noise before disconnect
			if result, ok := msg.Args["result"].(string); ok && strings.TrimSpace(result) == ")off" {
				m.log("  (skipped: external )off)")
				return m, waitForRide(m.msgs)
			}
			// Input from elsewhere - display it
			m.log("  (external input)")
		}

		// Route output to internal query if one is pending
		if m.internalQuery != "" {
			if result, ok := msg.Args["result"].(string); ok {
				m.internalOutputs = append(m.internalOutputs, result)
				m.log("  (internal query output)")
			}
			return m, waitForRide(m.msgs)
		}

		if result, ok := msg.Args["result"].(string); ok {
			result = strings.TrimSuffix(result, "\n")
			for _, line := range strings.Split(result, "\n") {
				m.lines = append(m.lines, Line{Text: line})
			}
			m.cursorRow = len(m.lines) - 1
			m.cursorCol = 0
		}

	case "SetPromptType":
		if t, ok := msg.Args["type"].(float64); ok {
			wasReady := m.ready
			m.ready = t > 0
			m.log("  ready: %v → %v", wasReady, m.ready)

			// Complete internal query if one was pending
			if m.ready && m.internalQuery != "" {
				m.log("  internal query complete: %d outputs", len(m.internalOutputs))
				oldQuery := m.internalQuery
				if m.internalCallback != nil {
					m.internalCallback(m.internalOutputs)
				}
				// Only clear if callback didn't start a new query
				if m.internalQuery == oldQuery {
					m.internalQuery = ""
					m.internalCallback = nil
					m.internalOutputs = nil
				}
				// Don't add new input line for internal queries
				return m, waitForRide(m.msgs)
			}

			if m.ready {
				// Add new input line with APL indent
				m.lines = append(m.lines, Line{Text: aplIndent})
				m.cursorRow = len(m.lines) - 1
				m.cursorCol = len(aplIndent)
			}
		}

	case "OpenWindow":
		w := NewEditorWindow(msg.Args)
		m.editors[w.Token] = w

		if w.Debugger {
			// Tracer window - add to stack, show single tracer pane
			m.tracerStack = append(m.tracerStack, w.Token)
			m.showTracer(w.Token)
			m.log("  opened tracer: %s (token=%d, stack depth=%d)", w.Name, w.Token, len(m.tracerStack))
		} else {
			// Regular editor - create pane as before
			token := w.Token
			editorPane := NewEditorPane(w, m.config.TracerKeys,
				func() { m.saveEditor(token) },
				func() { m.closeEditor(token) },
			)

			// Position: center of screen
			paneW := min(m.width-4, 60)
			paneH := min(m.height-6, 20)
			if paneW < 30 {
				paneW = 30
			}
			if paneH < 10 {
				paneH = 10
			}
			paneX := (m.width - paneW) / 2
			paneY := (m.height - paneH) / 2

			paneID := fmt.Sprintf("editor:%d", w.Token)
			pane := NewPane(paneID, editorPane, paneX, paneY, paneW, paneH)
			m.panes.Add(pane)
			m.panes.Focus(paneID)
			m.log("  opened editor: %s (token=%d)", w.Name, w.Token)
		}

	case "UpdateWindow":
		token := int(msg.Args["token"].(float64))
		if w, exists := m.editors[token]; exists {
			w.Update(msg.Args)
			m.log("  updated: %s (token=%d)", w.Name, token)
		}

	case "CloseWindow":
		win := int(msg.Args["win"].(float64))

		// Check if this is a tracer window
		if m.isInTracerStack(win) {
			m.removeFromTracerStack(win)
			m.log("  closed tracer: token=%d (stack depth=%d)", win, len(m.tracerStack))
		} else {
			// Regular editor
			paneID := fmt.Sprintf("editor:%d", win)
			m.panes.Remove(paneID)
			m.log("  closed editor: token=%d", win)
		}
		delete(m.editors, win)

	case "ReplySaveChanges":
		win := int(msg.Args["win"].(float64))
		errCode := 0
		if e, ok := msg.Args["err"].(float64); ok {
			errCode = int(e)
		}

		if errCode == 0 {
			m.log("  save succeeded: token=%d", win)
			if w, exists := m.editors[win]; exists {
				w.Modified = false
				// If close was pending, send CloseWindow now
				if w.PendingClose {
					w.PendingClose = false
					m.sendCloseWindow(win)
				}
			}
		} else {
			m.log("  save FAILED: token=%d, err=%d", win, errCode)
			// Clear pending close on failure
			if w, exists := m.editors[win]; exists {
				w.PendingClose = false
			}
		}

	case "SetHighlightLine":
		win := int(msg.Args["win"].(float64))
		line := int(msg.Args["line"].(float64))

		// Store highlight in the window itself
		if w, exists := m.editors[win]; exists {
			w.CurrentRow = line
		}

		// Update pane if this is the current tracer or a regular editor
		if m.isInTracerStack(win) {
			// If this is the current tracer, update the tracer pane
			if win == m.tracerCurrent {
				if pane := m.panes.Get("tracer"); pane != nil {
					if ep, ok := pane.Content.(*EditorPane); ok {
						ep.SetHighlightLine(line)
					}
				}
			}
		} else {
			// Regular editor pane
			paneID := fmt.Sprintf("editor:%d", win)
			if pane := m.panes.Get(paneID); pane != nil {
				if ep, ok := pane.Content.(*EditorPane); ok {
					ep.SetHighlightLine(line)
				}
			}
		}
		m.log("  highlight: token=%d, line=%d", win, line)

		// Refresh variables pane if open and we're in tracer
		if m.isInTracerStack(win) {
			if pane := m.panes.Get("variables"); pane != nil {
				if vp, ok := pane.Content.(*VariablesPane); ok {
					m.fetchVariables(vp)
				}
			}
		}

	case "WindowTypeChanged":
		win := int(msg.Args["win"].(float64))
		tracer := int(msg.Args["tracer"].(float64))
		if w, exists := m.editors[win]; exists {
			w.Debugger = tracer != 0
			m.log("  window type changed: token=%d, tracer=%v", win, w.Debugger)
		}

	case "ReplyGetAutocomplete":
		token := int(msg.Args["token"].(float64))
		skip := 0
		if s, ok := msg.Args["skip"].(float64); ok {
			skip = int(s)
		}
		var options []string
		if opts, ok := msg.Args["options"].([]any); ok {
			for _, o := range opts {
				if s, ok := o.(string); ok {
					options = append(options, s)
				}
			}
		}
		m.log("  autocomplete: token=%d, skip=%d, options=%d", token, skip, len(options))

		// Ignore if we're not waiting for autocomplete
		if !m.acPending {
			m.log("  (no pending request, ignoring)")
			return m, waitForRide(m.msgs)
		}
		m.acPending = false

		if len(options) == 0 {
			// No completions - do nothing
			return m, waitForRide(m.msgs)
		}

		// Get current cursor position for insertion later
		_, triggerCol := m.getAutocompleteContext(token)

		if len(options) == 1 {
			// Single option - auto-insert immediately
			m.acPopup = NewAutocomplete(options, skip, token, triggerCol)
			m.insertAutocomplete()
			return m, waitForRide(m.msgs)
		}

		// Multiple options - show popup
		m.showAutocomplete(options, skip, token, triggerCol)
	}

	return m, waitForRide(m.msgs)
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress any key to exit.\n", m.err)
	}

	w, h := m.width, m.height
	if w < 20 {
		w = 80
	}
	if h < 5 {
		h = 24
	}

	// Reserve space for help line
	helpHeight := 1
	if m.help.ShowAll {
		helpHeight = 4 // More space for full help
	}
	mainH := h - helpHeight

	// Render base session
	base := m.viewSession(w, mainH)

	// Composite floating panes over session
	if m.panes.HasPanes() {
		base = m.panes.Render(base)
	}

	// Render autocomplete overlay if active
	if m.acPopup != nil {
		base = m.renderAutocompleteOverlay(base, w, mainH)
	}

	// Add help at bottom
	m.help.Width = w
	var helpView string
	if m.confirmQuit {
		confirmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		helpView = confirmStyle.Render("Quit? (y/n)")
	} else if m.showQuitHint {
		hintStyle := lipgloss.NewStyle().Foreground(AccentColor)
		helpView = hintStyle.Render("Type C-] q to quit")
	} else if m.leaderActive {
		leaderStyle := lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
		helpView = leaderStyle.Render("C-] ...")
	} else if m.paneMoveMode {
		moveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
		helpView = moveStyle.Render("MOVE: arrows move, shift+arrows resize, esc exit")
	} else if m.savePromptActive {
		promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
		helpView = promptStyle.Render("Save as: ") + m.savePromptFilename + cursorStyle.Render(" ")
	} else if m.backtickActive {
		backtickStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("207")).Bold(true)
		helpView = backtickStyle.Render("` APL symbol...")
	} else if m.isTracerFocused() {
		tracerStyle := lipgloss.NewStyle().Foreground(AccentColor)
		helpView = tracerStyle.Render("n next • i into • o out • c continue • p back • f forward • e edit • esc close")
	} else {
		helpView = m.help.View(m.keys)
	}

	return base + "\n" + helpView
}

func (m Model) viewSession(w, h int) string {
	contentW := w - 2
	contentH := h - 2

	content := m.renderSession(contentW, contentH)

	title := "gritt"
	borderColor := AccentColor
	if !m.connected {
		title = "gritt [disconnected]"
		borderColor = lipgloss.Color("196") // Red
	}

	return m.renderBox(title, content, contentW, contentH, borderColor)
}

func (m Model) renderBox(title, content string, w, h int, borderColor color.Color) string {
	// Build box manually with title in top border
	topBorder := "╭─ " + title + " " + strings.Repeat("─", w-len(title)-3) + "╮"
	bottomBorder := "╰" + strings.Repeat("─", w) + "╯"

	// Style the borders
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(borderColor).Bold(true)

	// Build top border with styled title
	topBorder = borderStyle.Render("╭─ ") + titleStyle.Render(title) + borderStyle.Render(" "+strings.Repeat("─", w-len(title)-3)+"╮")

	// Split content into lines and pad to height
	contentLines := strings.Split(content, "\n")
	for len(contentLines) < h {
		contentLines = append(contentLines, "")
	}

	// Build the box
	var sb strings.Builder
	sb.WriteString(topBorder)
	sb.WriteString("\n")

	for i := 0; i < h; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		sb.WriteString(borderStyle.Render("│"))
		sb.WriteString(line)
		sb.WriteString(borderStyle.Render("│"))
		sb.WriteString("\n")
	}

	sb.WriteString(borderStyle.Render(bottomBorder))

	return sb.String()
}

func (m Model) renderSession(w, h int) string {
	// Calculate viewport - follow cursor
	startLine := 0
	if m.cursorRow >= h {
		startLine = m.cursorRow - h + 1
	}

	lines := make([]string, h)
	for i := 0; i < h; i++ {
		srcIdx := startLine + i
		if srcIdx >= len(m.lines) {
			lines[i] = strings.Repeat(" ", w)
			continue
		}

		lineData := m.lines[srcIdx]
		text := lineData.Text
		runes := []rune(text)

		// Truncate if too wide (leave room for cursor)
		maxLen := w - 1
		if len(runes) > maxLen {
			runes = runes[:maxLen]
		}

		// Render with cursor if this is the current line
		if srcIdx == m.cursorRow {
			col := m.cursorCol
			if col > len(runes) {
				col = len(runes)
			}

			var rendered string
			var visualLen int
			if col < len(runes) {
				// Cursor on a character - visual length unchanged
				rendered = string(runes[:col]) + cursorStyle.Render(string(runes[col])) + string(runes[col+1:])
				visualLen = len(runes)
			} else {
				// Cursor at end - adds a space
				rendered = string(runes) + cursorStyle.Render(" ")
				visualLen = len(runes) + 1
			}
			// Pad to width
			if visualLen < w {
				rendered += strings.Repeat(" ", w-visualLen)
			}
			lines[i] = rendered
		} else {
			// Pad to width
			line := string(runes)
			if len(runes) < w {
				line += strings.Repeat(" ", w-len(runes))
			}
			lines[i] = line
		}
	}

	return strings.Join(lines, "\n")
}

