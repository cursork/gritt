package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/cursork/gritt/aplcart"
)

// APLcart is a searchable APLcart pane
type APLcart struct {
	entries        []aplcart.Entry
	filtered       []aplcart.Entry
	query          string
	selected       int
	scroll         int
	loading        bool
	err            error
	SelectedSyntax string // Set when Enter pressed
}

// NewAPLcart creates an APLcart pane (starts loading)
func NewAPLcart() *APLcart {
	return &APLcart{
		loading: true,
	}
}

// APLcartCacheResult is sent when an APLcart cache fetch/refresh completes.
type APLcartCacheResult struct {
	Entries []aplcart.Entry
	Err     error
}

// RefreshAPLcartCache fetches APLcart from GitHub and updates the cache.
func RefreshAPLcartCache() tea.Msg {
	entries, err := aplcart.RefreshCache()
	return APLcartCacheResult{Entries: entries, Err: err}
}

func (a *APLcart) SetData(entries []aplcart.Entry, err error) {
	a.loading = false
	a.err = err
	a.entries = entries
	a.filtered = entries
}

func (a *APLcart) filter() {
	a.filtered = aplcart.Search(a.entries, a.query)
	if a.selected >= len(a.filtered) {
		a.selected = len(a.filtered) - 1
	}
	if a.selected < 0 {
		a.selected = 0
	}
	a.scroll = 0
}

func (a *APLcart) Title() string {
	return "APLcart"
}

func (a *APLcart) Render(w, h int) string {
	if a.loading {
		loadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		return loadStyle.Render("Loading APLcart...")
	}

	if a.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		return errStyle.Render("Error: " + a.err.Error())
	}

	var sb strings.Builder

	// Query line
	promptStyle := lipgloss.NewStyle().Foreground(AccentColor)
	sb.WriteString(promptStyle.Render("/ "))
	sb.WriteString(a.query)
	sb.WriteString(cursorStyle.Render(" "))
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sb.WriteString(countStyle.Render("  (" + itoa(len(a.filtered)) + ")"))
	sb.WriteString("\n")

	// Separator
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	// Entries list
	selectedStyle := lipgloss.NewStyle().Background(AccentColor).Foreground(lipgloss.Color("0"))
	syntaxStyle := lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	listH := h - 2
	for i := a.scroll; i < len(a.filtered) && i < a.scroll+listH; i++ {
		e := a.filtered[i]

		syntax := e.Syntax
		desc := e.Description

		// Truncate syntax if too long (rune-aware)
		maxSyntax := w / 3
		syntaxRunes := []rune(syntax)
		if len(syntaxRunes) > maxSyntax {
			syntax = string(syntaxRunes[:maxSyntax-1]) + "…"
		}
		syntax = padRight(syntax, maxSyntax)

		// Truncate desc (rune-aware)
		maxDesc := w - maxSyntax - 2
		descRunes := []rune(desc)
		if len(descRunes) > maxDesc {
			desc = string(descRunes[:maxDesc-1]) + "…"
		}

		if i == a.selected {
			sb.WriteString(selectedStyle.Render(syntax) + " " + descStyle.Render(desc))
		} else {
			sb.WriteString(syntaxStyle.Render(syntax) + " " + descStyle.Render(desc))
		}

		if i < len(a.filtered)-1 && i < a.scroll+listH-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func (a *APLcart) HandleKey(msg tea.KeyMsg) bool {
	if a.loading || a.err != nil {
		return false
	}

	switch msg.Type {
	case tea.KeyUp:
		if a.selected > 0 {
			a.selected--
			if a.selected < a.scroll {
				a.scroll = a.selected
			}
		}
		return true

	case tea.KeyDown:
		if a.selected < len(a.filtered)-1 {
			a.selected++
			if a.selected >= a.scroll+15 {
				a.scroll = a.selected - 14
			}
		}
		return true

	case tea.KeyEnter:
		if a.selected >= 0 && a.selected < len(a.filtered) {
			a.SelectedSyntax = a.filtered[a.selected].Syntax
		}
		return true

	case tea.KeyBackspace:
		if len(a.query) > 0 {
			a.query = a.query[:len(a.query)-1]
			a.filter()
		}
		return true

	default:
		if len(msg.Runes) > 0 {
			a.query += string(msg.Runes)
			a.filter()
			return true
		}
	}

	return false
}

func (a *APLcart) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	if a.loading || a.err != nil {
		return false
	}

	if msg.Type == tea.MouseLeft && y >= 2 {
		idx := y - 2 + a.scroll
		if idx >= 0 && idx < len(a.filtered) {
			a.selected = idx
			a.SelectedSyntax = a.filtered[a.selected].Syntax
			return true
		}
	}
	return false
}
