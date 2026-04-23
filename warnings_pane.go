package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

type messageLevel int

const (
	levelWarning messageLevel = iota
	levelError
)

type paneMessage struct {
	level messageLevel
	text  string
}

// WarningsPane displays warning and error messages. Dismissed with Escape.
type WarningsPane struct {
	messages []paneMessage
	scroll   int
}

func (w *WarningsPane) Title() string {
	return "⚠ messages"
}

func (w *WarningsPane) Render(width, height int) string {
	var sb strings.Builder

	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	sb.WriteString(dimStyle.Render("Esc to dismiss"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", width))
	sb.WriteString("\n")

	listH := height - 2
	if listH < 1 {
		listH = 1
	}

	for i := w.scroll; i < len(w.messages) && i < w.scroll+listH; i++ {
		msg := w.messages[i]
		text := msg.text
		// Truncate long lines
		runes := []rune(text)
		if len(runes) > width-2 {
			text = string(runes[:width-3]) + "…"
		}
		style := warnStyle
		if msg.level == levelError {
			style = errStyle
		}
		sb.WriteString(style.Render("  " + text))
		if i < len(w.messages)-1 && i < w.scroll+listH-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (w *WarningsPane) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if w.scroll > 0 {
			w.scroll--
		}
		return true
	case tea.KeyDown:
		w.scroll++
		if w.scroll >= len(w.messages) {
			w.scroll = len(w.messages) - 1
		}
		if w.scroll < 0 {
			w.scroll = 0
		}
		return true
	case tea.KeyEscape:
		return false // let parent close
	}
	return false
}

func (w *WarningsPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	return false
}
