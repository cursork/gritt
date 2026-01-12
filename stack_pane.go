package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// StackFrame represents one frame in the SI stack
type StackFrame struct {
	Token   int
	Name    string
	Line    int    // CurrentRow
	Code    string // Line of code at that position
	Current bool   // Is this the currently displayed frame?
}

// StackPane displays the tracer stack and allows navigation
type StackPane struct {
	getStack func() []StackFrame
	onSelect func(token int)
	selected int // Index in stack (0 = bottom, len-1 = top)

	// Styles
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	currentStyle  lipgloss.Style
}

// NewStackPane creates a stack pane with callbacks
func NewStackPane(getStack func() []StackFrame, onSelect func(token int)) *StackPane {
	return &StackPane{
		getStack:      getStack,
		onSelect:      onSelect,
		normalStyle:   lipgloss.NewStyle(),
		selectedStyle: lipgloss.NewStyle().Background(lipgloss.Color("240")),
		currentStyle:  lipgloss.NewStyle().Foreground(DyalogOrange).Bold(true),
	}
}

func (s *StackPane) Title() string {
	stack := s.getStack()
	return fmt.Sprintf("stack (%d)", len(stack))
}

func (s *StackPane) Render(w, h int) string {
	stack := s.getStack()
	if len(stack) == 0 {
		return "  (no stack)"
	}

	// Clamp selected to valid range
	if s.selected >= len(stack) {
		s.selected = len(stack) - 1
	}
	if s.selected < 0 {
		s.selected = 0
	}

	var lines []string

	// Render stack in reverse order (top of stack first)
	for i := len(stack) - 1; i >= 0; i-- {
		frame := stack[i]

		// Format: "name[line] code"
		// Truncate code to fit
		codeWidth := w - len(frame.Name) - 6 // "[nn] "
		code := frame.Code
		if len(code) > codeWidth && codeWidth > 3 {
			code = code[:codeWidth-3] + "..."
		}

		line := fmt.Sprintf("%s[%d] %s", frame.Name, frame.Line, code)

		// Pad to width
		if len(line) < w {
			line = line + strings.Repeat(" ", w-len(line))
		} else if len(line) > w {
			line = line[:w]
		}

		// Apply styles
		displayIdx := len(stack) - 1 - i // Display index (0 = top)
		if displayIdx == s.selected {
			if frame.Current {
				line = s.currentStyle.Render("►") + s.selectedStyle.Render(line[1:])
			} else {
				line = s.selectedStyle.Render(line)
			}
		} else if frame.Current {
			line = s.currentStyle.Render("►" + line[1:])
		}

		lines = append(lines, line)
	}

	// Pad remaining height
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}

	return strings.Join(lines[:h], "\n")
}

func (s *StackPane) HandleKey(msg tea.KeyMsg) bool {
	stack := s.getStack()
	if len(stack) == 0 {
		return false
	}

	switch msg.Type {
	case tea.KeyUp:
		if s.selected > 0 {
			s.selected--
		}
		return true
	case tea.KeyDown:
		if s.selected < len(stack)-1 {
			s.selected++
		}
		return true
	case tea.KeyEnter:
		// Select this frame - convert display index to stack index
		stackIdx := len(stack) - 1 - s.selected
		if stackIdx >= 0 && stackIdx < len(stack) {
			s.onSelect(stack[stackIdx].Token)
		}
		return true
	}
	return false
}

func (s *StackPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	stack := s.getStack()
	if len(stack) == 0 {
		return false
	}

	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		// Click to select
		if y >= 0 && y < len(stack) {
			s.selected = y
			// Also trigger selection
			stackIdx := len(stack) - 1 - s.selected
			if stackIdx >= 0 && stackIdx < len(stack) {
				s.onSelect(stack[stackIdx].Token)
			}
		}
		return true
	}
	return false
}
