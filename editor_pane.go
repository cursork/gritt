package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// EditorPane implements PaneContent for editing APL functions
type EditorPane struct {
	window *EditorWindow

	// View state
	scrollY  int  // First visible line
	editMode bool // True when tracer is in edit mode (Shift+Enter to enable)

	// Tracer key bindings (single characters)
	tracerKeys TracerKeysConfig

	// Callbacks
	onSave  func()
	onClose func()

	// Tracer control callbacks (only used when window.Debugger is true)
	onStepInto   func()
	onStepOver   func()
	onStepOut    func()
	onContinue   func()
	onResumeAll  func()
	onBackward   func()
	onForward    func()

	// Styles
	cursorStyle      lipgloss.Style
	lineNumStyle     lipgloss.Style
	breakpointStyle  lipgloss.Style
	tracerLineStyle  lipgloss.Style // Bold for current line in tracer
	highlightLine    int            // -1 = none, otherwise 0-based line for tracer highlight
}

// NewEditorPane creates an editor pane for the given window
func NewEditorPane(w *EditorWindow, tracerKeys TracerKeysConfig, onSave, onClose func()) *EditorPane {
	return &EditorPane{
		window:     w,
		tracerKeys: tracerKeys,
		onSave:     onSave,
		onClose:    onClose,
		cursorStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("255")).
			Foreground(lipgloss.Color("0")),
		lineNumStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		breakpointStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("9")), // Red
		tracerLineStyle: lipgloss.NewStyle().Foreground(DyalogOrange),
		highlightLine:   -1,
	}
}

// SetWindow switches the pane to display a different window (for tracer switching)
func (e *EditorPane) SetWindow(w *EditorWindow) {
	e.window = w
	e.scrollY = 0
	e.highlightLine = -1
	e.editMode = false
	// Position cursor at highlighted line if set
	if w.CurrentRow >= 0 && w.CurrentRow < len(w.Text) {
		e.window.CursorRow = w.CurrentRow
		e.window.CursorCol = 0
	}
}

func (e *EditorPane) Title() string {
	prefix := ""
	if e.window.Modified {
		prefix = "* "
	}
	suffix := ""
	if e.window.Debugger {
		if e.editMode {
			suffix = " [edit]"
		} else {
			suffix = " [tracer]"
		}
	} else {
		suffix = " [edit]"
	}
	return prefix + e.window.Name + suffix
}

// InTracerMode returns true if this is a tracer in trace mode (not edit mode)
func (e *EditorPane) InTracerMode() bool {
	return e.window.Debugger && !e.editMode
}

func (e *EditorPane) Render(w, h int) string {
	if len(e.window.Text) == 0 {
		e.window.Text = []string{""}
	}

	// Calculate line number width: [0], [1], ..., [99], [100]
	maxLine := len(e.window.Text) - 1
	numWidth := len(fmt.Sprintf("[%d]", maxLine))

	// Adjust scroll to keep cursor visible
	if e.window.CursorRow < e.scrollY {
		e.scrollY = e.window.CursorRow
	}
	if e.window.CursorRow >= e.scrollY+h {
		e.scrollY = e.window.CursorRow - h + 1
	}

	var lines []string
	for i := 0; i < h; i++ {
		lineIdx := e.scrollY + i
		if lineIdx >= len(e.window.Text) {
			// Empty line below content
			lines = append(lines, strings.Repeat(" ", w))
			continue
		}

		// Breakpoint indicator
		bp := " "
		if e.window.HasStop(lineIdx) {
			bp = e.breakpointStyle.Render("●")
		}

		// Line number
		lineNum := e.lineNumStyle.Render(fmt.Sprintf("[%*d]", numWidth-2, lineIdx))

		// Line content
		text := e.window.Text[lineIdx]
		textRunes := []rune(text)

		// Content width after breakpoint, line number and spaces
		contentW := w - numWidth - 3 // 1 for bp, 1 space after bp, 1 space after linenum
		if contentW < 1 {
			contentW = 1
		}

		// Render line with cursor if on this line
		var lineContent string
		isCurrentLine := lineIdx == e.window.CursorRow
		if isCurrentLine {
			// Pass tracer style if in tracer mode
			var lineStyle *lipgloss.Style
			if e.InTracerMode() {
				lineStyle = &e.tracerLineStyle
			}
			lineContent = e.renderLineWithCursor(textRunes, e.window.CursorCol, contentW, lineStyle)
		} else {
			lineContent = e.renderLine(textRunes, contentW)
		}

		// In tracer mode, highlight entire current line including line number
		if isCurrentLine && e.InTracerMode() {
			lineNum = e.tracerLineStyle.Render(fmt.Sprintf("[%*d]", numWidth-2, lineIdx))
		}

		lines = append(lines, bp+" "+lineNum+" "+lineContent)
	}

	return strings.Join(lines, "\n")
}

// renderLine renders a line without cursor, padded/truncated to width
func (e *EditorPane) renderLine(runes []rune, w int) string {
	if len(runes) >= w {
		return string(runes[:w])
	}
	return string(runes) + strings.Repeat(" ", w-len(runes))
}

// renderLineWithCursor renders a line with cursor highlight at col position
// If lineStyle is non-nil, it's applied to the non-cursor parts (for tracer highlight)
func (e *EditorPane) renderLineWithCursor(runes []rune, col, w int, lineStyle *lipgloss.Style) string {
	// Clamp cursor to valid range
	if col > len(runes) {
		col = len(runes)
	}
	if col < 0 {
		col = 0
	}

	// Build parts: before cursor, cursor char, after cursor
	before := string(runes[:col])

	var cursorChar string
	if col < len(runes) {
		cursorChar = e.cursorStyle.Render(string(runes[col]))
	} else {
		cursorChar = e.cursorStyle.Render(" ")
	}

	var after string
	if col+1 < len(runes) {
		after = string(runes[col+1:])
	}

	// Apply line style to non-cursor parts if provided
	if lineStyle != nil {
		before = lineStyle.Render(before)
		after = lineStyle.Render(after)
	}

	line := before + cursorChar + after

	// Pad to width (approximate due to ANSI codes)
	visibleLen := len(runes)
	if visibleLen < w {
		pad := strings.Repeat(" ", w-visibleLen-1)
		if lineStyle != nil {
			pad = lineStyle.Render(pad)
		}
		return line + pad
	}
	return line
}

func (e *EditorPane) HandleKey(msg tea.KeyMsg) bool {
	// Tracer mode - navigation, tracer controls, and close
	// Tracer windows (Debugger=true) are read-only unless edit mode enabled
	if e.window.Debugger && !e.editMode {
		switch msg.Type {
		case tea.KeyUp:
			e.cursorUp()
		case tea.KeyDown:
			e.cursorDown()
		case tea.KeyLeft:
			e.cursorLeft()
		case tea.KeyRight:
			e.cursorRight()
		case tea.KeyHome:
			e.window.CursorCol = 0
		case tea.KeyEnd:
			e.window.CursorCol = len([]rune(e.currentLine()))
		case tea.KeyEnter:
			// Enter = Step over (run current line)
			if e.onStepOver != nil {
				e.onStepOver()
			}
		case tea.KeyEscape:
			if e.onClose != nil {
				e.onClose()
			}
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				r := msg.Runes[0]
				switch {
				case e.matchKey(r, e.tracerKeys.StepInto):
					if e.onStepInto != nil {
						e.onStepInto()
					}
				case e.matchKey(r, e.tracerKeys.StepOver):
					if e.onStepOver != nil {
						e.onStepOver()
					}
				case e.matchKey(r, e.tracerKeys.StepOut):
					if e.onStepOut != nil {
						e.onStepOut()
					}
				case e.matchKey(r, e.tracerKeys.Continue):
					if e.onContinue != nil {
						e.onContinue()
					}
				case e.matchKey(r, e.tracerKeys.ResumeAll):
					if e.onResumeAll != nil {
						e.onResumeAll()
					}
				case e.matchKey(r, e.tracerKeys.Backward):
					if e.onBackward != nil {
						e.onBackward()
					}
				case e.matchKey(r, e.tracerKeys.Forward):
					if e.onForward != nil {
						e.onForward()
					}
				case e.matchKey(r, e.tracerKeys.EditMode):
					e.editMode = true
				default:
					return false
				}
			} else {
				return false
			}
		default:
			return false
		}
		return true
	}

	// Read-only mode (non-tracer read-only windows, but NOT tracer in edit mode)
	if e.window.ReadOnly && !e.editMode {
		switch msg.Type {
		case tea.KeyUp:
			e.cursorUp()
		case tea.KeyDown:
			e.cursorDown()
		case tea.KeyLeft:
			e.cursorLeft()
		case tea.KeyRight:
			e.cursorRight()
		case tea.KeyHome:
			e.window.CursorCol = 0
		case tea.KeyEnd:
			e.window.CursorCol = len([]rune(e.currentLine()))
		case tea.KeyEscape:
			if e.onClose != nil {
				e.onClose()
			}
		default:
			return false
		}
		return true
	}

	// Editable mode
	switch msg.Type {
	case tea.KeyUp:
		e.cursorUp()
	case tea.KeyDown:
		e.cursorDown()
	case tea.KeyLeft:
		e.cursorLeft()
	case tea.KeyRight:
		e.cursorRight()
	case tea.KeyHome:
		e.window.CursorCol = 0
	case tea.KeyEnd:
		e.window.CursorCol = len([]rune(e.currentLine()))
	case tea.KeyEnter:
		e.insertNewline()
	case tea.KeyBackspace:
		e.deleteCharBack()
	case tea.KeyDelete:
		e.deleteCharForward()
	case tea.KeyCtrlS:
		if e.onSave != nil {
			e.onSave()
		}
	case tea.KeyEscape:
		// If in edit mode of a tracer, just exit edit mode (don't save yet)
		// Changes stay pending until the window actually closes
		if e.editMode && e.window.Debugger {
			e.editMode = false
		} else if e.onClose != nil {
			e.onClose()
		}
	case tea.KeySpace:
		e.insertChar(' ')
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			e.insertChar(r)
		}
	default:
		return false
	}
	return true
}

func (e *EditorPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if e.scrollY > 0 {
			e.scrollY--
		}
		return true
	case tea.MouseButtonWheelDown:
		if e.scrollY < len(e.window.Text)-1 {
			e.scrollY++
		}
		return true
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress {
			// Click to position cursor
			// x is relative to content area, need to account for line numbers
			// For now, just set row based on y
			targetRow := e.scrollY + y
			if targetRow >= 0 && targetRow < len(e.window.Text) {
				e.window.CursorRow = targetRow
				// Approximate col from x (subtract line number width estimate)
				e.window.CursorCol = x - 5 // rough estimate
				if e.window.CursorCol < 0 {
					e.window.CursorCol = 0
				}
				lineLen := len([]rune(e.currentLine()))
				if e.window.CursorCol > lineLen {
					e.window.CursorCol = lineLen
				}
			}
			return true
		}
	}
	return false
}

// currentLine returns the current line text
func (e *EditorPane) currentLine() string {
	if e.window.CursorRow >= 0 && e.window.CursorRow < len(e.window.Text) {
		return e.window.Text[e.window.CursorRow]
	}
	return ""
}

// Cursor movement
func (e *EditorPane) cursorUp() {
	if e.window.CursorRow > 0 {
		e.window.CursorRow--
		e.clampCol()
	}
}

func (e *EditorPane) cursorDown() {
	if e.window.CursorRow < len(e.window.Text)-1 {
		e.window.CursorRow++
		e.clampCol()
	}
}

func (e *EditorPane) cursorLeft() {
	if e.window.CursorCol > 0 {
		e.window.CursorCol--
	} else if e.window.CursorRow > 0 {
		// Wrap to end of previous line
		e.window.CursorRow--
		e.window.CursorCol = len([]rune(e.currentLine()))
	}
}

func (e *EditorPane) cursorRight() {
	lineLen := len([]rune(e.currentLine()))
	if e.window.CursorCol < lineLen {
		e.window.CursorCol++
	} else if e.window.CursorRow < len(e.window.Text)-1 {
		// Wrap to start of next line
		e.window.CursorRow++
		e.window.CursorCol = 0
	}
}

func (e *EditorPane) clampCol() {
	lineLen := len([]rune(e.currentLine()))
	if e.window.CursorCol > lineLen {
		e.window.CursorCol = lineLen
	}
}

// Text editing
func (e *EditorPane) insertChar(r rune) {
	line := e.currentLine()
	runes := []rune(line)
	col := e.window.CursorCol
	if col > len(runes) {
		col = len(runes)
	}

	newRunes := make([]rune, 0, len(runes)+1)
	newRunes = append(newRunes, runes[:col]...)
	newRunes = append(newRunes, r)
	newRunes = append(newRunes, runes[col:]...)

	e.window.Text[e.window.CursorRow] = string(newRunes)
	e.window.CursorCol++
	e.window.Modified = true
}

func (e *EditorPane) deleteCharBack() {
	if e.window.CursorCol > 0 {
		// Delete within line
		line := e.currentLine()
		runes := []rune(line)
		col := e.window.CursorCol

		newRunes := make([]rune, 0, len(runes)-1)
		newRunes = append(newRunes, runes[:col-1]...)
		newRunes = append(newRunes, runes[col:]...)

		e.window.Text[e.window.CursorRow] = string(newRunes)
		e.window.CursorCol--
		e.window.Modified = true
	} else if e.window.CursorRow > 0 {
		// Join with previous line
		prevLine := e.window.Text[e.window.CursorRow-1]
		currLine := e.currentLine()
		newCol := len([]rune(prevLine))

		e.window.Text[e.window.CursorRow-1] = prevLine + currLine
		e.window.Text = append(e.window.Text[:e.window.CursorRow], e.window.Text[e.window.CursorRow+1:]...)
		e.window.CursorRow--
		e.window.CursorCol = newCol
		e.window.Modified = true
	}
}

func (e *EditorPane) deleteCharForward() {
	line := e.currentLine()
	runes := []rune(line)
	col := e.window.CursorCol

	if col < len(runes) {
		// Delete at cursor
		newRunes := make([]rune, 0, len(runes)-1)
		newRunes = append(newRunes, runes[:col]...)
		newRunes = append(newRunes, runes[col+1:]...)

		e.window.Text[e.window.CursorRow] = string(newRunes)
		e.window.Modified = true
	} else if e.window.CursorRow < len(e.window.Text)-1 {
		// Join with next line
		nextLine := e.window.Text[e.window.CursorRow+1]
		e.window.Text[e.window.CursorRow] = line + nextLine
		e.window.Text = append(e.window.Text[:e.window.CursorRow+1], e.window.Text[e.window.CursorRow+2:]...)
		e.window.Modified = true
	}
}

func (e *EditorPane) insertNewline() {
	line := e.currentLine()
	runes := []rune(line)
	col := e.window.CursorCol
	if col > len(runes) {
		col = len(runes)
	}

	// Split line at cursor
	before := string(runes[:col])
	after := string(runes[col:])

	e.window.Text[e.window.CursorRow] = before

	// Insert new line after
	newText := make([]string, 0, len(e.window.Text)+1)
	newText = append(newText, e.window.Text[:e.window.CursorRow+1]...)
	newText = append(newText, after)
	newText = append(newText, e.window.Text[e.window.CursorRow+1:]...)

	e.window.Text = newText
	e.window.CursorRow++
	e.window.CursorCol = 0
	e.window.Modified = true
}

// SetHighlightLine sets the tracer highlight line (for SetHighlightLine message)
func (e *EditorPane) SetHighlightLine(line int) {
	e.highlightLine = line
	// Jump to highlighted line
	if line >= 0 && line < len(e.window.Text) {
		e.window.CursorRow = line
		e.window.CursorCol = 0
	}
}

// TracerCallbacks holds all tracer control callbacks
type TracerCallbacks struct {
	StepInto  func()
	StepOver  func()
	StepOut   func()
	Continue  func()
	ResumeAll func()
	Backward  func()
	Forward   func()
}

// SetTracerCallbacks sets all tracer control callbacks at once
func (e *EditorPane) SetTracerCallbacks(cb TracerCallbacks) {
	e.onStepInto = cb.StepInto
	e.onStepOver = cb.StepOver
	e.onStepOut = cb.StepOut
	e.onContinue = cb.Continue
	e.onResumeAll = cb.ResumeAll
	e.onBackward = cb.Backward
	e.onForward = cb.Forward
}


// matchKey checks if a rune matches a single-character config key
func (e *EditorPane) matchKey(r rune, configKey string) bool {
	if configKey == "" {
		return false
	}
	return string(r) == configKey
}
