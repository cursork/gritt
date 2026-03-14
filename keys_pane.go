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
	commands *CommandRegistry
	nav      NavKeys
}

// NewKeysPane creates a key mappings pane
func NewKeysPane(commands *CommandRegistry, nav NavKeys) *KeysPane {
	vp := viewport.New(0, 0)
	return &KeysPane{
		viewport: vp,
		commands: commands,
		nav:      nav,
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

	// Leader commands
	sb.WriteString("--- Leader commands ---\n")
	for _, cmd := range k.commands.leaderCmds {
		h := cmd.Binding.Help()
		sb.WriteString(fmt.Sprintf("  %-16s %s\n", h.Key, cmd.Help))
	}
	sb.WriteString("\n")

	// Direct commands
	sb.WriteString("--- Direct commands ---\n")
	for _, cmd := range k.commands.directCmds {
		h := cmd.Binding.Help()
		sb.WriteString(fmt.Sprintf("  %-16s %s\n", h.Key, cmd.Help))
	}
	sb.WriteString("\n")

	// Tracer commands
	sb.WriteString("--- Tracer commands ---\n")
	for _, cmd := range k.commands.tracerCmds {
		h := cmd.Binding.Help()
		sb.WriteString(fmt.Sprintf("  %-16s %s\n", h.Key, cmd.Help))
	}
	sb.WriteString("\n")

	// Navigation
	navBindings := []key.Binding{
		k.nav.Up, k.nav.Down, k.nav.Left, k.nav.Right,
		k.nav.Home, k.nav.End, k.nav.PgUp, k.nav.PgDn,
		k.nav.Execute, k.nav.Backspace, k.nav.Delete,
	}
	sb.WriteString("--- Navigation ---\n")
	for _, b := range navBindings {
		if b.Enabled() {
			h := b.Help()
			sb.WriteString(fmt.Sprintf("  %-16s %s\n", h.Key, h.Desc))
		}
	}

	sb.WriteString("\nPress Esc to close")
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
