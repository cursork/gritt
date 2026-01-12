package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// KeysPane displays all key mappings
type KeysPane struct {
	viewport viewport.Model
	keys     KeyMap
}

// NewKeysPane creates a key mappings pane
func NewKeysPane(keys KeyMap) *KeysPane {
	vp := viewport.New(0, 0)
	return &KeysPane{
		viewport: vp,
		keys:     keys,
	}
}

func (k *KeysPane) Title() string {
	return "key mappings"
}

func (k *KeysPane) Render(w, h int) string {
	k.viewport.Width = w
	k.viewport.Height = h
	k.viewport.SetContent(k.buildContent(w))
	return k.viewport.View()
}

func (k *KeysPane) buildContent(width int) string {
	var sb strings.Builder

	bindings := []struct {
		category string
		keys     []key.Binding
	}{
		{"Actions", []key.Binding{
			k.keys.Execute,
			k.keys.ToggleDebug,
			k.keys.CyclePane,
			k.keys.ClosePane,
			k.keys.ShowKeys,
			k.keys.Quit,
		}},
		{"Navigation", []key.Binding{
			k.keys.Up,
			k.keys.Down,
			k.keys.Left,
			k.keys.Right,
			k.keys.Home,
			k.keys.End,
			k.keys.PgUp,
			k.keys.PgDn,
		}},
		{"Editing", []key.Binding{
			k.keys.Backspace,
			k.keys.Delete,
		}},
	}

	for _, cat := range bindings {
		sb.WriteString(fmt.Sprintf("─── %s ───\n", cat.category))
		for _, b := range cat.keys {
			help := b.Help()
			keyStr := help.Key
			desc := help.Desc
			// Pad for alignment
			line := fmt.Sprintf("  %-12s %s\n", keyStr, desc)
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Press Esc to close")
	return sb.String()
}

func (k *KeysPane) HandleKey(msg tea.KeyMsg) bool {
	var cmd tea.Cmd
	k.viewport, cmd = k.viewport.Update(msg)
	return cmd != nil
}

func (k *KeysPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	var cmd tea.Cmd
	k.viewport, cmd = k.viewport.Update(msg)
	return cmd != nil
}
