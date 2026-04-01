package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// HistoryPane is a searchable command history overlay.
type HistoryPane struct {
	history  []string // Full history (shared reference, not copied)
	filtered []int    // Indices into history that match query
	query    string
	selected int
	scroll   int
	listH    int    // Last known list height for scroll calc
	Selected string // Set when Enter pressed — checked by tui.go
}

func NewHistoryPane(history []string) *HistoryPane {
	hp := &HistoryPane{
		history: history,
	}
	hp.filter()
	return hp
}

func (h *HistoryPane) filter() {
	h.filtered = nil
	q := strings.ToLower(h.query)
	seen := make(map[string]bool)
	for i, entry := range h.history {
		text := strings.TrimSpace(entry)
		if text == "" {
			continue
		}
		// Deduplicate in display
		if seen[text] {
			continue
		}
		seen[text] = true
		if q == "" || strings.Contains(strings.ToLower(entry), q) {
			h.filtered = append(h.filtered, i)
		}
	}
	if h.selected >= len(h.filtered) {
		h.selected = len(h.filtered) - 1
	}
	if h.selected < 0 {
		h.selected = 0
	}
	h.scroll = 0
}

func (h *HistoryPane) Title() string {
	return "History"
}

func (h *HistoryPane) Render(w, h2 int) string {
	var sb strings.Builder

	// Query line
	promptStyle := lipgloss.NewStyle().Foreground(AccentColor)
	sb.WriteString(promptStyle.Render("/ "))
	sb.WriteString(h.query)
	sb.WriteString(cursorStyle.Render(" "))
	sb.WriteString("\n")

	// Separator
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	if len(h.filtered) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		if h.query != "" {
			sb.WriteString(dimStyle.Render("  no matches"))
		} else {
			sb.WriteString(dimStyle.Render("  no history"))
		}
		return sb.String()
	}

	selectedStyle := lipgloss.NewStyle().Background(AccentColor).Foreground(lipgloss.Color("0"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	listH := h2 - 2
	if listH < 1 {
		listH = 1
	}
	h.listH = listH

	// Adjust scroll
	if h.selected >= h.scroll+listH {
		h.scroll = h.selected - listH + 1
	}
	if h.selected < h.scroll {
		h.scroll = h.selected
	}

	for i := h.scroll; i < len(h.filtered) && i < h.scroll+listH; i++ {
		entry := strings.TrimSpace(h.history[h.filtered[i]])

		// Truncate if needed (rune-aware)
		entryRunes := []rune(entry)
		if len(entryRunes) > w-2 {
			entry = string(entryRunes[:w-3]) + "…"
		}

		if i == h.selected {
			// Pad to full width for highlight bar
			padded := " " + entry
			padRunes := []rune(padded)
			for len(padRunes) < w {
				padRunes = append(padRunes, ' ')
			}
			sb.WriteString(selectedStyle.Render(string(padRunes)))
		} else {
			sb.WriteString(normalStyle.Render(" " + entry))
		}

		if i < len(h.filtered)-1 && i < h.scroll+listH-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (h *HistoryPane) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if h.selected > 0 {
			h.selected--
		}
		return true

	case tea.KeyDown:
		if h.selected < len(h.filtered)-1 {
			h.selected++
		}
		return true

	case tea.KeyPgUp:
		h.selected -= h.listH
		if h.selected < 0 {
			h.selected = 0
		}
		return true

	case tea.KeyPgDown:
		h.selected += h.listH
		if h.selected >= len(h.filtered) {
			h.selected = len(h.filtered) - 1
		}
		if h.selected < 0 {
			h.selected = 0
		}
		return true

	case tea.KeyEnter:
		if h.selected >= 0 && h.selected < len(h.filtered) {
			h.Selected = h.history[h.filtered[h.selected]]
		}
		return true

	case tea.KeyBackspace:
		if len(h.query) > 0 {
			runes := []rune(h.query)
			h.query = string(runes[:len(runes)-1])
			h.filter()
		}
		return true

	case tea.KeyEscape:
		return false // Let parent handle (close pane)

	default:
		if len(msg.Runes) > 0 {
			h.query += string(msg.Runes)
			h.filter()
			return true
		}
	}

	return false
}

func (h *HistoryPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	if msg.Type == tea.MouseLeft && y >= 2 {
		idx := h.scroll + (y - 2)
		if idx >= 0 && idx < len(h.filtered) {
			h.selected = idx
			h.Selected = h.history[h.filtered[h.selected]]
			return true
		}
	}
	return false
}
