package main

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for gritt
type KeyMap struct {
	// Leader key (prefix for gritt commands)
	Leader key.Binding

	// Actions (some require leader prefix)
	Execute          key.Binding
	ToggleDebug      key.Binding // After leader
	ToggleStack      key.Binding // After leader
	ToggleLocals     key.Binding // After leader - show local variables in tracer
	ToggleBreakpoint key.Binding // After leader - toggle breakpoint in editor/tracer
	Reconnect        key.Binding // After leader
	CommandPalette   key.Binding // After leader
	PaneMoveMode     key.Binding // After leader
	CyclePane        key.Binding
	ClosePane        key.Binding
	Quit             key.Binding
	ShowKeys         key.Binding // After leader
	Autocomplete     key.Binding // Trigger code completion
	DocHelp          key.Binding // Context-sensitive documentation

	// Navigation
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Home  key.Binding
	End   key.Binding
	PgUp  key.Binding
	PgDn  key.Binding

	// Editing
	Backspace key.Binding
	Delete    key.Binding
}

// ShortHelp returns keybindings for the short help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Execute, k.ToggleDebug, k.ToggleStack, k.ShowKeys, k.Quit}
}

// FullHelp returns keybindings for the full help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Execute, k.ToggleDebug, k.CyclePane, k.ClosePane},
		{k.Up, k.Down, k.Left, k.Right},
		{k.Home, k.End, k.PgUp, k.PgDn},
		{k.ShowKeys, k.Quit},
	}
}
