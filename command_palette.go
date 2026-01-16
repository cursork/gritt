package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// Command represents an executable command in the palette
type Command struct {
	Name string
	Help string
}

// CommandPalette is a searchable command list
type CommandPalette struct {
	commands       []Command
	filtered       []Command
	query          string
	selected       int
	SelectedAction string // Set when Enter pressed
}

// NewCommandPalette creates a command palette with the given commands
func NewCommandPalette(commands []Command) *CommandPalette {
	cp := &CommandPalette{
		commands: commands,
		filtered: commands,
	}
	return cp
}

func (c *CommandPalette) filter() {
	if c.query == "" {
		c.filtered = c.commands
		return
	}

	q := strings.ToLower(c.query)
	c.filtered = nil
	for _, cmd := range c.commands {
		if strings.Contains(strings.ToLower(cmd.Name), q) ||
			strings.Contains(strings.ToLower(cmd.Help), q) {
			c.filtered = append(c.filtered, cmd)
		}
	}

	// Reset selection if out of bounds
	if c.selected >= len(c.filtered) {
		c.selected = len(c.filtered) - 1
	}
	if c.selected < 0 {
		c.selected = 0
	}
}

func (c *CommandPalette) Title() string {
	return "Commands"
}

func (c *CommandPalette) Render(w, h int) string {
	var sb strings.Builder

	// Query line
	promptStyle := lipgloss.NewStyle().Foreground(DyalogOrange)
	sb.WriteString(promptStyle.Render(": "))
	sb.WriteString(c.query)
	sb.WriteString(cursorStyle.Render(" "))
	sb.WriteString("\n")

	// Separator
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	// Commands list
	selectedStyle := lipgloss.NewStyle().Background(DyalogOrange).Foreground(lipgloss.Color("0"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	listH := h - 2 // Account for query line and separator
	for i, cmd := range c.filtered {
		if i >= listH {
			break
		}

		name := cmd.Name
		help := cmd.Help

		// Truncate if needed (rune-aware)
		maxName := w / 3
		nameRunes := []rune(name)
		if len(nameRunes) > maxName {
			name = string(nameRunes[:maxName-1]) + "…"
		}

		line := name + " " + helpStyle.Render(help)
		lineRunes := []rune(line)
		if len(lineRunes) > w {
			line = string(lineRunes[:w])
		}

		if i == c.selected {
			// Render selected line with highlight
			line = selectedStyle.Render(padRight(name, maxName)) + " " + helpStyle.Render(help)
		}

		sb.WriteString(line)
		if i < len(c.filtered)-1 && i < listH-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func (c *CommandPalette) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if c.selected > 0 {
			c.selected--
		}
		return true

	case tea.KeyDown:
		if c.selected < len(c.filtered)-1 {
			c.selected++
		}
		return true

	case tea.KeyEnter:
		if c.selected >= 0 && c.selected < len(c.filtered) {
			c.SelectedAction = c.filtered[c.selected].Name
		}
		return true

	case tea.KeyBackspace:
		if len(c.query) > 0 {
			c.query = c.query[:len(c.query)-1]
			c.filter()
		}
		return true

	default:
		if len(msg.Runes) > 0 {
			c.query += string(msg.Runes)
			c.filter()
			return true
		}
	}

	return false
}

func (c *CommandPalette) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	if msg.Type == tea.MouseLeft && y >= 2 {
		idx := y - 2 // Account for query and separator
		if idx >= 0 && idx < len(c.filtered) {
			c.selected = idx
			c.SelectedAction = c.filtered[c.selected].Name
			return true
		}
	}
	return false
}
