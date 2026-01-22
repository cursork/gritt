package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
)

// Autocomplete holds state for the completion popup overlay.
// This is NOT a pane - it's rendered as an overlay while the session/editor stays focused.
type Autocomplete struct {
	Options  []string // Completion options from Dyalog
	Selected int      // Currently selected index
	Skip     int      // Characters to replace before cursor
	Token    int      // Window token (0 for session, >0 for editor)
	TriggerCol int    // Cursor column when autocomplete was triggered
}

// NewAutocomplete creates autocomplete state
func NewAutocomplete(options []string, skip, token, triggerCol int) *Autocomplete {
	return &Autocomplete{
		Options:    options,
		Selected:   0,
		Skip:       skip,
		Token:      token,
		TriggerCol: triggerCol,
	}
}

// CycleNext moves selection to next option (wraps around)
func (a *Autocomplete) CycleNext() {
	if len(a.Options) == 0 {
		return
	}
	a.Selected = (a.Selected + 1) % len(a.Options)
}

// CyclePrev moves selection to previous option (wraps around)
func (a *Autocomplete) CyclePrev() {
	if len(a.Options) == 0 {
		return
	}
	a.Selected = (a.Selected - 1 + len(a.Options)) % len(a.Options)
}

// SelectedOption returns the currently selected option
func (a *Autocomplete) SelectedOption() string {
	if a.Selected >= 0 && a.Selected < len(a.Options) {
		return a.Options[a.Selected]
	}
	return ""
}

// Render returns the popup content for overlay rendering
func (a *Autocomplete) Render(maxW, maxH int) string {
	if len(a.Options) == 0 {
		return ""
	}

	selectedStyle := lipgloss.NewStyle().Background(DyalogOrange).Foreground(lipgloss.Color("0"))
	borderStyle := lipgloss.NewStyle().Foreground(DyalogOrange)

	// Calculate dimensions
	contentW := 0
	for _, opt := range a.Options {
		if len(opt) > contentW {
			contentW = len(opt)
		}
	}
	if contentW > maxW-4 {
		contentW = maxW - 4
	}
	if contentW < 10 {
		contentW = 10
	}

	contentH := len(a.Options)
	if contentH > maxH-2 {
		contentH = maxH - 2
	}

	// Calculate scroll offset to keep selection visible
	scrollOffset := 0
	if a.Selected >= contentH {
		scrollOffset = a.Selected - contentH + 1
	}

	var lines []string

	// Top border
	lines = append(lines, borderStyle.Render("┌"+strings.Repeat("─", contentW)+"┐"))

	// Options
	for i := scrollOffset; i < len(a.Options) && i < scrollOffset+contentH; i++ {
		opt := a.Options[i]
		optRunes := []rune(opt)
		if len(optRunes) > contentW {
			opt = string(optRunes[:contentW-1]) + "…"
		}

		padded := opt + strings.Repeat(" ", contentW-len([]rune(opt)))

		if i == a.Selected {
			lines = append(lines, borderStyle.Render("│")+selectedStyle.Render(padded)+borderStyle.Render("│"))
		} else {
			lines = append(lines, borderStyle.Render("│")+padded+borderStyle.Render("│"))
		}
	}

	// Bottom border
	lines = append(lines, borderStyle.Render("└"+strings.Repeat("─", contentW)+"┘"))

	return strings.Join(lines, "\n")
}

// Width returns the rendered width of the popup
func (a *Autocomplete) Width() int {
	w := 10
	for _, opt := range a.Options {
		if len(opt) > w {
			w = len(opt)
		}
	}
	return w + 2 // borders
}

// Height returns the rendered height of the popup
func (a *Autocomplete) Height(maxH int) int {
	h := len(a.Options)
	if h > maxH-2 {
		h = maxH - 2
	}
	return h + 2 // borders
}
