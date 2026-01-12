package main

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gritt/ride"
)

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

	// Session state
	lines     []Line
	cursorRow int
	cursorCol int
	ready     bool // Interpreter ready for input

	// Debug log (shared with debug pane)
	debugLog []string

	// Floating panes
	panes     *PaneManager
	debugPane *DebugPane // Keep reference to update log

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
func NewModel(client *ride.Client) Model {
	ch := make(chan rideEvent)
	go func() {
		for {
			msg, raw, err := client.Recv()
			ch <- rideEvent{msg: msg, raw: raw, err: err}
			if err != nil {
				close(ch)
				return
			}
		}
	}()

	m := Model{
		client: client,
		msgs:   ch,
		ready:  true, // Handshake already completed
		lines:  []Line{{Text: aplIndent}},
		panes:  NewPaneManager(80, 24), // Will be updated on WindowSizeMsg
	}
	m.cursorCol = len(aplIndent)
	m.log("Connected, ready for input")
	return m
}

func (m *Model) log(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	m.debugLog = append(m.debugLog, line)
	if len(m.debugLog) > 500 {
		m.debugLog = m.debugLog[len(m.debugLog)-500:]
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
	// Global shortcuts (always work regardless of focus)
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyF12:
		m.toggleDebugPane()
		return m, nil

	case tea.KeyTab:
		if m.panes.HasPanes() {
			m.panes.FocusNext()
		}
		return m, nil

	case tea.KeyEsc:
		// Close focused pane (if any)
		if fp := m.panes.FocusedPane(); fp != nil {
			m.panes.Remove(fp.ID)
		}
		return m, nil
	}

	// Route to focused pane first
	if fp := m.panes.FocusedPane(); fp != nil && fp.Content != nil {
		if fp.Content.HandleKey(msg) {
			return m, nil // Key consumed by pane
		}
	}

	// Session key handling
	return m.handleSessionKey(msg)
}

func (m *Model) toggleDebugPane() {
	if m.panes.Get("debug") != nil {
		m.panes.Remove("debug")
		m.debugPane = nil
	} else {
		// Create debug pane on the right side
		paneW := 50
		paneH := m.height - 4
		if paneH < 10 {
			paneH = 10
		}
		paneX := m.width - paneW - 2
		if paneX < 0 {
			paneX = 0
		}

		m.debugPane = NewDebugPane(&m.debugLog)
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
	m.log("→ Execute %q", code)

	// Send to interpreter
	if err := m.client.Send("Execute", map[string]any{"text": code + "\n", "trace": 0}); err != nil {
		m.err = err
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleRide(ev rideEvent) (tea.Model, tea.Cmd) {
	if ev.err != nil {
		m.err = ev.err
		return m, tea.Quit
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
		// Skip input echo (type 14)
		if t, ok := msg.Args["type"].(float64); ok && int(t) == 14 {
			m.log("  (skipped: input echo)")
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
			if m.ready {
				// Add new input line with APL indent
				m.lines = append(m.lines, Line{Text: aplIndent})
				m.cursorRow = len(m.lines) - 1
				m.cursorCol = len(aplIndent)
			}
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

	// Render base session
	base := m.viewSession(w, h)

	// Composite floating panes over session
	if m.panes.HasPanes() {
		return m.panes.Render(base)
	}
	return base
}

func (m Model) viewSession(w, h int) string {
	contentW := w - 2
	contentH := h - 2

	content := m.renderSession(contentW, contentH)
	return m.renderBox("gritt", content, contentW, contentH, "63")
}

func (m Model) renderBox(title, content string, w, h int, color string) string {
	borderColor := lipgloss.Color(color)

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

