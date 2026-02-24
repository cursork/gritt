package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// LocalVar represents a variable in the current scope
type LocalVar struct {
	Name    string
	Value   string // Preview value (may be truncated)
	IsLocal bool   // True if declared as local in function header
}

// VarsMode determines which variables are shown
type VarsMode int

const (
	VarsModeLocals VarsMode = iota // Only variables assigned in current function
	VarsModeAll                    // All visible variables (includes inherited)
)

// VariablesPane displays variables when in tracer
type VariablesPane struct {
	vars     []LocalVar
	selected int
	mode     VarsMode
	loading  bool                 // True while fetching variables
	onOpen   func(name string)    // Called when user wants to open variable with )ed
	onToggle func(mode VarsMode)  // Called when user toggles mode

	// Styles
	selectedStyle lipgloss.Style // Orange for selected line
	normalStyle   lipgloss.Style // Gray for non-selected lines
}

// NewVariablesPane creates a variables pane
func NewVariablesPane(onOpen func(name string), onToggle func(mode VarsMode)) *VariablesPane {
	return &VariablesPane{
		onOpen:        onOpen,
		onToggle:      onToggle,
		mode:          VarsModeLocals,
		selectedStyle: lipgloss.NewStyle().Foreground(AccentColor),
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

// SetVars updates the list of variables and clears loading state
func (v *VariablesPane) SetVars(vars []LocalVar) {
	v.vars = vars
	v.loading = false
	// Clamp selection
	if v.selected >= len(vars) {
		v.selected = len(vars) - 1
	}
	if v.selected < 0 {
		v.selected = 0
	}
}

// SetLoading sets the loading state
func (v *VariablesPane) SetLoading(loading bool) {
	v.loading = loading
}

// Clear removes all variables and clears loading state
func (v *VariablesPane) Clear() {
	v.vars = nil
	v.selected = 0
	v.loading = false
}

// Mode returns the current display mode
func (v *VariablesPane) Mode() VarsMode {
	return v.mode
}

// SetMode changes the display mode
func (v *VariablesPane) SetMode(mode VarsMode) {
	v.mode = mode
}

func (v *VariablesPane) Title() string {
	if v.mode == VarsModeAll {
		return "variables [all]"
	}
	return "variables [local]"
}

func (v *VariablesPane) Render(w, h int) string {
	if v.loading {
		return v.normalStyle.Render("  Loading...")
	}
	if len(v.vars) == 0 {
		return v.normalStyle.Render("  (no variables)")
	}

	var lines []string

	// Calculate max name width for alignment (rune-safe for APL characters)
	maxNameWidth := 0
	for _, vr := range v.vars {
		if len([]rune(vr.Name)) > maxNameWidth {
			maxNameWidth = len([]rune(vr.Name))
		}
	}
	if maxNameWidth > w/3 {
		maxNameWidth = w / 3
	}

	for i, vr := range v.vars {
		// Format: "• name = value" (• for locals in all mode)
		prefix := "  " // 2 spaces by default
		if v.mode == VarsModeAll && vr.IsLocal {
			prefix = "• " // bullet for locals
		}

		name := vr.Name
		nameRunes := []rune(name)
		if len(nameRunes) > maxNameWidth {
			name = string(nameRunes[:maxNameWidth])
		}

		// Pad name for alignment (rune-safe)
		namePadded := name + strings.Repeat(" ", maxNameWidth-len([]rune(name)))

		// Calculate available space for value (account for prefix)
		valueWidth := w - maxNameWidth - 3 - len(prefix) // prefix + " = "
		value := vr.Value
		valueRunes := []rune(value)
		if len(valueRunes) > valueWidth && valueWidth > 3 {
			value = string(valueRunes[:valueWidth-3]) + "..."
		}

		// Build plain text line
		plainLine := prefix + namePadded + " = " + value
		plainLen := len([]rune(plainLine))
		if plainLen < w {
			plainLine = plainLine + strings.Repeat(" ", w-plainLen)
		}

		// Apply style based on selection
		var line string
		if i == v.selected {
			line = v.selectedStyle.Render(plainLine)
		} else {
			line = v.normalStyle.Render(plainLine)
		}

		lines = append(lines, line)
	}

	// Pad remaining height
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}

	// Only show what fits - scroll to keep selected visible
	if len(lines) > h {
		start := 0
		if v.selected >= h {
			start = v.selected - h + 1
		}
		lines = lines[start : start+h]
	}

	return strings.Join(lines[:h], "\n")
}

func (v *VariablesPane) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if v.selected > 0 {
			v.selected--
		}
		return true
	case tea.KeyDown:
		if v.selected < len(v.vars)-1 {
			v.selected++
		}
		return true
	case tea.KeyEnter:
		// Open selected variable with )ed
		if v.selected >= 0 && v.selected < len(v.vars) && v.onOpen != nil {
			v.onOpen(v.vars[v.selected].Name)
		}
		return true
	case tea.KeyRunes:
		// '~' toggles mode - but DON'T call onToggle callback here
		// The TUI handles the refresh to avoid stale Model reference
		if len(msg.Runes) == 1 && msg.Runes[0] == '~' {
			if v.mode == VarsModeLocals {
				v.mode = VarsModeAll
			} else {
				v.mode = VarsModeLocals
			}
			// Set loading state - TUI will trigger the actual fetch
			v.loading = true
			return true
		}
	}
	return false
}

func (v *VariablesPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	if len(v.vars) == 0 {
		return false
	}

	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		if y >= 0 && y < len(v.vars) {
			v.selected = y
		}
		return true
	}

	return false
}
