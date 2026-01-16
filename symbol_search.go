package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// SymbolSearch is a searchable APL symbol list
type SymbolSearch struct {
	filtered       []APLSymbol
	query          string
	selected       int
	scroll         int  // Scroll offset
	SelectedSymbol rune // Set when Enter pressed
}

// NewSymbolSearch creates a symbol search pane
func NewSymbolSearch() *SymbolSearch {
	return &SymbolSearch{
		filtered: aplSymbols,
	}
}

func (s *SymbolSearch) filter() {
	if s.query == "" {
		s.filtered = aplSymbols
		s.selected = 0
		s.scroll = 0
		return
	}

	q := strings.ToLower(s.query)
	s.filtered = nil
	for _, sym := range aplSymbols {
		// Match against names
		for _, name := range sym.Names {
			if strings.Contains(name, q) {
				s.filtered = append(s.filtered, sym)
				break
			}
		}
	}

	if s.selected >= len(s.filtered) {
		s.selected = len(s.filtered) - 1
	}
	if s.selected < 0 {
		s.selected = 0
	}
	s.scroll = 0
}

func (s *SymbolSearch) Title() string {
	return "APL Symbols"
}

func (s *SymbolSearch) Render(w, h int) string {
	var sb strings.Builder

	// Query line
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("207"))
	sb.WriteString(promptStyle.Render("/ "))
	sb.WriteString(s.query)
	sb.WriteString(cursorStyle.Render(" "))
	sb.WriteString("\n")

	// Separator
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	// Symbols list
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("207")).Foreground(lipgloss.Color("0"))
	symStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("207")).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	listH := h - 2
	for i := s.scroll; i < len(s.filtered) && i < s.scroll+listH; i++ {
		sym := s.filtered[i]

		char := string(sym.Char)
		keycode := sym.Keycode
		if keycode == "" {
			keycode = "   "
		} else {
			keycode = padRight(keycode, 3)
		}
		desc := sym.Desc

		// Truncate desc if needed (rune-aware)
		maxDesc := w - 8
		descRunes := []rune(desc)
		if len(descRunes) > maxDesc {
			desc = string(descRunes[:maxDesc-1]) + "…"
		}

		if i == s.selected {
			line := selectedStyle.Render(" "+char+" ") + " " + keyStyle.Render(keycode) + " " + descStyle.Render(desc)
			sb.WriteString(line)
		} else {
			line := symStyle.Render(" "+char+" ") + " " + keyStyle.Render(keycode) + " " + descStyle.Render(desc)
			sb.WriteString(line)
		}

		if i < len(s.filtered)-1 && i < s.scroll+listH-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (s *SymbolSearch) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if s.selected > 0 {
			s.selected--
			if s.selected < s.scroll {
				s.scroll = s.selected
			}
		}
		return true

	case tea.KeyDown:
		if s.selected < len(s.filtered)-1 {
			s.selected++
			// Scroll down if needed (assume ~15 visible lines)
			if s.selected >= s.scroll+15 {
				s.scroll = s.selected - 14
			}
		}
		return true

	case tea.KeyEnter:
		if s.selected >= 0 && s.selected < len(s.filtered) {
			s.SelectedSymbol = s.filtered[s.selected].Char
		}
		return true

	case tea.KeyBackspace:
		if len(s.query) > 0 {
			s.query = s.query[:len(s.query)-1]
			s.filter()
		}
		return true

	default:
		if len(msg.Runes) > 0 {
			s.query += string(msg.Runes)
			s.filter()
			return true
		}
	}

	return false
}

func (s *SymbolSearch) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	if msg.Type == tea.MouseLeft && y >= 2 {
		idx := y - 2
		if idx >= 0 && idx < len(s.filtered) {
			s.selected = idx
			s.SelectedSymbol = s.filtered[s.selected].Char
			return true
		}
	}
	return false
}
