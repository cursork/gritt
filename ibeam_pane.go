package main

import (
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/cursork/gritt/ibeam"
)

// IBeamSearch is a searchable I-beam (⌶) lookup pane.
type IBeamSearch struct {
	db           *sql.DB
	query        string
	results      []ibeam.Entry
	selected     int
	scrollOffset int
	SelectedEntry *ibeam.Entry // Set when Enter pressed — TUI opens doc

	// Detail view for private entries (no public docs)
	detail      *ibeam.Entry
	detailScroll int
}

func NewIBeamSearch(db *sql.DB) *IBeamSearch {
	ib := &IBeamSearch{db: db}
	ib.results = ibeam.All(db) // Show all on open
	return ib
}

func (ib *IBeamSearch) search() {
	if ib.query == "" {
		ib.results = ibeam.All(ib.db)
	} else {
		ib.results = ibeam.Search(ib.db, ib.query)
	}
	if ib.selected >= len(ib.results) {
		ib.selected = len(ib.results) - 1
	}
	if ib.selected < 0 {
		ib.selected = 0
	}
	ib.scrollOffset = 0
}

func (ib *IBeamSearch) Title() string {
	return "I-Beam ⌶ Lookup"
}

func (ib *IBeamSearch) Render(w, h int) string {
	if ib.detail != nil {
		return ib.renderDetail(w, h)
	}

	var sb strings.Builder

	promptStyle := lipgloss.NewStyle().Foreground(AccentColor)
	sb.WriteString(promptStyle.Render("⌶ "))
	sb.WriteString(ib.query)
	sb.WriteString(cursorStyle.Render(" "))
	sb.WriteString("\n")

	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	selectedStyle := lipgloss.NewStyle().Background(AccentColor).Foreground(lipgloss.Color("0"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	privateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	listH := h - 2
	if listH < 1 {
		listH = 1
	}

	ib.adjustScroll(listH)

	if len(ib.results) == 0 && ib.query != "" {
		sb.WriteString(dimStyle.Render("No results"))
		return sb.String()
	}

	visibleCount := 0
	for i := ib.scrollOffset; i < len(ib.results) && visibleCount < listH; i++ {
		e := ib.results[i]

		// Format: "220  Serialise/Deserialise Array  R←X(220⌶)Y"
		num := fmt.Sprintf("%5d", e.Number)
		name := e.Name
		sig := e.Signature

		// Truncate name to fit
		maxName := w - 8 - len([]rune(sig)) - 2
		if maxName < 10 {
			maxName = 10
		}
		nameRunes := []rune(name)
		if len(nameRunes) > maxName {
			name = string(nameRunes[:maxName-1]) + "…"
		}

		var line string
		if e.Source == "private" {
			line = num + "  " + privateStyle.Render(name) + "  " + dimStyle.Render(sig)
		} else {
			line = num + "  " + name + "  " + dimStyle.Render(sig)
		}

		if i == ib.selected {
			// Re-render as selected (plain text, highlighted)
			plain := fmt.Sprintf("%5d  %s  %s", e.Number, name, sig)
			plainRunes := []rune(plain)
			if len(plainRunes) > w {
				plain = string(plainRunes[:w])
			}
			line = selectedStyle.Render(plain)
		}

		sb.WriteString(line)
		visibleCount++
		if visibleCount < listH {
			sb.WriteString("\n")
		}
	}

	for visibleCount < listH {
		sb.WriteString(strings.Repeat(" ", w))
		visibleCount++
		if visibleCount < listH {
			sb.WriteString("\n")
		}
	}

	if len(ib.results) > 0 {
		count := fmt.Sprintf(" %d I-beams ", len(ib.results))
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render(count))
	}

	return sb.String()
}

func (ib *IBeamSearch) adjustScroll(listH int) {
	if ib.selected < ib.scrollOffset {
		ib.scrollOffset = ib.selected
	}
	if ib.selected >= ib.scrollOffset+listH {
		ib.scrollOffset = ib.selected - listH + 1
	}
}

func (ib *IBeamSearch) renderDetail(w, h int) string {
	e := ib.detail
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	sb.WriteString(titleStyle.Render(fmt.Sprintf("%d⌶ — %s", e.Number, e.Name)))
	sb.WriteString("\n")
	if e.Signature != "" {
		sb.WriteString(dimStyle.Render(e.Signature))
		sb.WriteString("\n")
	}
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	// Word-wrap description
	desc := e.Description
	if desc == "" {
		desc = "(no description available)"
	}
	lines := wordWrap(desc, w)

	listH := h - 3
	if listH < 1 {
		listH = 1
	}

	for i := ib.detailScroll; i < len(lines) && i-ib.detailScroll < listH; i++ {
		sb.WriteString(lines[i])
		if i-ib.detailScroll < listH-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func wordWrap(s string, w int) []string {
	if w < 10 {
		w = 10
	}
	words := strings.Fields(s)
	var lines []string
	var line string
	for _, word := range words {
		if line == "" {
			line = word
		} else if len(line)+1+len(word) <= w {
			line += " " + word
		} else {
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (ib *IBeamSearch) HandleKey(msg tea.KeyMsg) bool {
	// Detail view: Escape/Backspace returns to list
	if ib.detail != nil {
		switch msg.Type {
		case tea.KeyEscape, tea.KeyBackspace:
			ib.detail = nil
			return true
		case tea.KeyUp:
			if ib.detailScroll > 0 {
				ib.detailScroll--
			}
			return true
		case tea.KeyDown:
			ib.detailScroll++
			return true
		}
		return true
	}

	switch msg.Type {
	case tea.KeyUp:
		if ib.selected > 0 {
			ib.selected--
		}
		return true
	case tea.KeyDown:
		if ib.selected < len(ib.results)-1 {
			ib.selected++
		}
		return true
	case tea.KeyEnter:
		if ib.selected >= 0 && ib.selected < len(ib.results) {
			e := ib.results[ib.selected]
			ib.SelectedEntry = &e
		}
		return true
	case tea.KeyBackspace:
		if len(ib.query) > 0 {
			runes := []rune(ib.query)
			ib.query = string(runes[:len(runes)-1])
			ib.search()
		}
		return true
	case tea.KeyRunes:
		ib.query += string(msg.Runes)
		ib.search()
		return true
	}
	return false
}

func (ib *IBeamSearch) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	return false
}

func (ib *IBeamSearch) showDescription(entry *ibeam.Entry) {
	ib.detail = entry
	ib.detailScroll = 0
}
