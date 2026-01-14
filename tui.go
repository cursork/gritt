package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss/v2"
	"gritt/ride"
)

// DyalogOrange returns the brand color adapted to terminal capabilities
// RGB (242, 167, 79) = #F2A74F
var DyalogOrange color.Color

func initColors(profile colorprofile.Profile) {
	complete := lipgloss.Complete(profile)
	DyalogOrange = complete(
		lipgloss.Color("3"),       // ANSI: yellow
		lipgloss.Color("215"),     // ANSI256: #ffaf5f
		lipgloss.Color("#F2A74F"), // TrueColor: exact RGB
	)
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
	help help.Model
	keys KeyMap

	// Leader key state
	leaderActive bool
	showQuitHint bool
	confirmQuit  bool

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
	initColors(profile)

	cfg := LoadConfig()
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
		help:      help.New(),
		keys:      cfg.ToKeyMap(),
	}
	m.cursorCol = len(aplIndent)
	m.msgs = m.startRecvLoop()
	m.log("Connected to %s", addr)
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

	case rideEvent:
		return m.handleRide(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit confirmation
	if m.confirmQuit {
		m.confirmQuit = false
		if msg.String() == "y" || msg.String() == "Y" {
			return m, tea.Quit
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
		case key.Matches(msg, m.keys.Reconnect):
			return m.reconnect()
		case key.Matches(msg, m.keys.CommandPalette):
			m.openCommandPalette()
			return m, nil
		case key.Matches(msg, m.keys.ShowKeys):
			m.toggleKeysPane()
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

	// Global shortcuts (always work regardless of focus)
	switch {

	case key.Matches(msg, m.keys.CyclePane):
		if m.panes.HasPanes() {
			m.panes.FocusNext()
		}
		return m, nil

	case key.Matches(msg, m.keys.ClosePane):
		if fp := m.panes.FocusedPane(); fp != nil {
			if fp.ID == "tracer" {
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
		editorPane := NewEditorPane(w,
			func() { m.saveEditor(m.tracerCurrent) },
			func() { m.closeEditor(m.tracerCurrent) },
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

func (m *Model) openCommandPalette() {
	// Remove existing palette if open
	if m.panes.Get("commands") != nil {
		m.panes.Remove("commands")
		return
	}

	// Build command list
	commands := []Command{
		{Name: "debug", Help: "Toggle debug pane", Action: func() {
			m.panes.Remove("commands")
			m.toggleDebugPane()
		}},
		{Name: "stack", Help: "Toggle stack pane", Action: func() {
			m.panes.Remove("commands")
			m.toggleStackPane()
		}},
		{Name: "keys", Help: "Show key bindings", Action: func() {
			m.panes.Remove("commands")
			m.toggleKeysPane()
		}},
		{Name: "reconnect", Help: "Reconnect to Dyalog", Action: func() {
			m.panes.Remove("commands")
			// Note: reconnect returns tea.Cmd but we can't use it here
			// Just close old client and try to connect
			if !m.connected {
				m.log("Reconnecting to %s...", m.addr)
				if m.client != nil {
					m.client.Close()
				}
				if client, err := ride.Connect(m.addr); err == nil {
					m.client = client
					m.connected = true
					m.ready = true
					m.msgs = m.startRecvLoop()
					m.log("Reconnected to %s", m.addr)
				} else {
					m.log("Reconnect failed: %v", err)
				}
			}
		}},
		{Name: "quit", Help: "Quit gritt", Action: func() {
			m.panes.Remove("commands")
			m.confirmQuit = true
		}},
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
			// Skip )off from external input - just noise before disconnect
			if result, ok := msg.Args["result"].(string); ok && strings.TrimSpace(result) == ")off" {
				m.log("  (skipped: external )off)")
				return m, waitForRide(m.msgs)
			}
			// Input from elsewhere - display it
			m.log("  (external input)")
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
			editorPane := NewEditorPane(w,
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

	case "WindowTypeChanged":
		win := int(msg.Args["win"].(float64))
		tracer := int(msg.Args["tracer"].(float64))
		if w, exists := m.editors[win]; exists {
			w.Debugger = tracer != 0
			m.log("  window type changed: token=%d, tracer=%v", win, w.Debugger)
		}
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

	// Add help at bottom
	m.help.Width = w
	var helpView string
	if m.confirmQuit {
		confirmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		helpView = confirmStyle.Render("Quit? (y/n)")
	} else if m.showQuitHint {
		hintStyle := lipgloss.NewStyle().Foreground(DyalogOrange)
		helpView = hintStyle.Render("Type C-] q to quit")
	} else if m.leaderActive {
		leaderStyle := lipgloss.NewStyle().Foreground(DyalogOrange).Bold(true)
		helpView = leaderStyle.Render("C-] ...")
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
	borderColor := DyalogOrange
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

