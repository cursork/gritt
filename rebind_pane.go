package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

// rebindEntry represents one command in the rebind pane.
type rebindEntry struct {
	name    string
	help    string
	keys    []string
	leader  bool
	context string
}

// RebindPane allows interactive keybinding changes.
type RebindPane struct {
	entries      []rebindEntry
	selected     int
	scrollOffset int
	capturing    bool // waiting for next key press

	// Pending change — checked by tui.go after HandleKey
	PendingName    string
	PendingBinding BindingDef
	PendingApply   bool
}

// NewRebindPane creates a rebind pane from the current command registry and config.
func NewRebindPane(reg *CommandRegistry, bindings map[string]BindingDef) *RebindPane {
	var entries []rebindEntry
	for i := range reg.commands {
		cmd := &reg.commands[i]
		if cmd.Name == "leader" {
			continue // leader is not rebindable through this UI
		}
		bd := bindings[cmd.Name]
		entries = append(entries, rebindEntry{
			name:    cmd.Name,
			help:    cmd.Help,
			keys:    bd.Keys,
			leader:  bd.Leader,
			context: bd.Context,
		})
	}
	return &RebindPane{entries: entries}
}

func (r *RebindPane) Title() string {
	if r.capturing {
		e := r.entries[r.selected]
		return fmt.Sprintf("press key for: %s", e.name)
	}
	return "rebind keys"
}

func (r *RebindPane) Render(w, h int) string {
	var sb strings.Builder

	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	captureStyle := lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))

	// Hint line
	if r.capturing {
		sb.WriteString(captureStyle.Render(" Press any key to bind (Esc cancel) "))
	} else {
		sb.WriteString(dimStyle.Render("Enter:bind Tab:leader Del:unbind"))
	}
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	listH := h - 2
	if listH < 1 {
		listH = 1
	}

	r.adjustScroll(listH)

	visibleCount := 0
	for i := r.scrollOffset; i < len(r.entries) && visibleCount < listH; i++ {
		e := r.entries[i]

		// Key display
		var keyStr string
		if len(e.keys) > 0 {
			keyStr = strings.Join(e.keys, ",")
			if e.leader {
				keyStr = "L+" + keyStr
			}
			if e.context == "tracer" {
				keyStr = "[T] " + keyStr
			}
		} else {
			keyStr = "---"
		}

		// Format: keyStr right-aligned, name left
		nameW := w - 16 // reserve space for key column
		if nameW < 10 {
			nameW = 10
		}
		nameRunes := []rune(e.name)
		name := e.name
		if len(nameRunes) > nameW {
			name = string(nameRunes[:nameW-1]) + "…"
		}

		keyColW := w - nameW - 1
		if keyColW < 5 {
			keyColW = 5
		}

		var line string
		paddedName := padRight(name, nameW)
		keyDisplay := padRight(keyStr, keyColW)

		if i == r.selected {
			if r.capturing {
				line = captureStyle.Render(paddedName) + " " + keyStyle.Render(keyDisplay)
			} else {
				line = selectedStyle.Render(paddedName) + " " + keyStyle.Render(keyDisplay)
			}
		} else {
			line = paddedName + " " + dimStyle.Render(keyDisplay)
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

	return sb.String()
}

func (r *RebindPane) HandleKey(msg tea.KeyMsg) bool {
	if r.capturing {
		return r.handleCapture(msg)
	}

	switch msg.Type {
	case tea.KeyUp:
		if r.selected > 0 {
			r.selected--
		}
		return true

	case tea.KeyDown:
		if r.selected < len(r.entries)-1 {
			r.selected++
		}
		return true

	case tea.KeyEnter:
		// Start capturing
		r.capturing = true
		return true

	case tea.KeyTab:
		// Toggle leader for selected command
		e := &r.entries[r.selected]
		if e.context == "tracer" {
			return true // tracer commands can't be leader
		}
		e.leader = !e.leader
		r.applyChange(e)
		return true

	case tea.KeyDelete, tea.KeyBackspace:
		// Unbind selected command
		e := &r.entries[r.selected]
		e.keys = nil
		r.applyChange(e)
		return true

	case tea.KeyEscape:
		return false // let parent close

	default:
		return false
	}
}

func (r *RebindPane) handleCapture(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyEscape {
		r.capturing = false
		return true
	}

	// Capture the key
	var keyName string
	switch {
	case msg.Type == tea.KeyF1:
		keyName = "f1"
	case msg.Type == tea.KeyF2:
		keyName = "f2"
	case msg.Type == tea.KeyF3:
		keyName = "f3"
	case msg.Type == tea.KeyF4:
		keyName = "f4"
	case msg.Type == tea.KeyF5:
		keyName = "f5"
	case msg.Type == tea.KeyF6:
		keyName = "f6"
	case msg.Type == tea.KeyF7:
		keyName = "f7"
	case msg.Type == tea.KeyF8:
		keyName = "f8"
	case msg.Type == tea.KeyF9:
		keyName = "f9"
	case msg.Type == tea.KeyF10:
		keyName = "f10"
	case msg.Type == tea.KeyF11:
		keyName = "f11"
	case msg.Type == tea.KeyF12:
		keyName = "f12"
	case msg.Type == tea.KeyTab:
		keyName = "tab"
	case msg.Type == tea.KeyEnter:
		keyName = "enter"
	case msg.Type == tea.KeyBackspace:
		keyName = "backspace"
	case msg.Type == tea.KeyDelete:
		keyName = "delete"
	case msg.Type == tea.KeyUp:
		keyName = "up"
	case msg.Type == tea.KeyDown:
		keyName = "down"
	case msg.Type == tea.KeyLeft:
		keyName = "left"
	case msg.Type == tea.KeyRight:
		keyName = "right"
	case msg.Type == tea.KeyHome:
		keyName = "home"
	case msg.Type == tea.KeyEnd:
		keyName = "end"
	case msg.Type == tea.KeyPgUp:
		keyName = "pgup"
	case msg.Type == tea.KeyPgDown:
		keyName = "pgdown"
	default:
		// Use the key string from bubbletea
		keyName = msg.String()
	}

	if keyName == "" {
		r.capturing = false
		return true
	}

	e := &r.entries[r.selected]
	e.keys = []string{keyName}
	r.applyChange(e)
	r.capturing = false

	// Auto-advance to next entry
	if r.selected < len(r.entries)-1 {
		r.selected++
	}
	return true
}

func (r *RebindPane) applyChange(e *rebindEntry) {
	r.PendingName = e.name
	r.PendingBinding = BindingDef{
		Keys:    e.keys,
		Leader:  e.leader,
		Context: e.context,
	}
	r.PendingApply = true
}

func (r *RebindPane) adjustScroll(listH int) {
	if r.selected >= r.scrollOffset+listH {
		r.scrollOffset = r.selected - listH + 1
	}
	if r.selected < r.scrollOffset {
		r.scrollOffset = r.selected
	}
}

func (r *RebindPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonLeft && y >= 2 {
		idx := r.scrollOffset + y - 2
		if idx >= 0 && idx < len(r.entries) {
			r.selected = idx
			return true
		}
	}
	return false
}
