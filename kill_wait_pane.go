package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// KillWaitPane shows a countdown while waiting for a SIGTERM'd Dyalog
// to exit. When the countdown hits zero gritt sends SIGKILL.
type KillWaitPane struct {
	secondsLeft int
}

func (p *KillWaitPane) Title() string { return "Terminating Dyalog" }

func (p *KillWaitPane) Render(w, h int) string {
	count := lipgloss.NewStyle().Foreground(AccentColor).Bold(true).
		Render(fmt.Sprintf("%d", p.secondsLeft))
	keyStyle := lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	hint := keyStyle.Render("[esc]") + " Cancel    " +
		keyStyle.Render("[k]") + " Kill now"

	lines := []string{
		"",
		"  Waiting for Dyalog to terminate. " + count + " seconds...",
		"  " + dim.Render("After that it will be killed."),
		"",
		"  " + hint,
	}
	return strings.Join(lines, "\n")
}

// HandleKey is a no-op; the kill flow is driven from Model.Update so it can
// schedule timer commands and trigger tea.Quit.
func (p *KillWaitPane) HandleKey(msg tea.KeyMsg) bool { return false }

func (p *KillWaitPane) HandleMouse(x, y int, msg tea.MouseMsg) bool { return false }
