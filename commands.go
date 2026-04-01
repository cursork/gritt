package main

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// CommandDef defines a single command in the registry.
type CommandDef struct {
	Name    string
	Help    string
	Binding key.Binding
	Leader  bool
	Context string // "", "tracer"
	Action  func(m *Model) (tea.Model, tea.Cmd)
}

// CommandRegistry holds all commands and provides matching.
type CommandRegistry struct {
	leader     key.Binding
	commands   []CommandDef
	byName     map[string]*CommandDef
	leaderCmds []*CommandDef // Leader=true
	directCmds []*CommandDef // Leader=false, Context=""
	tracerCmds []*CommandDef // Context="tracer"
}

func newRegistry(leader key.Binding) *CommandRegistry {
	return &CommandRegistry{
		leader: leader,
		byName: make(map[string]*CommandDef),
	}
}

func (r *CommandRegistry) add(name, help string, leader bool, context string, action func(m *Model) (tea.Model, tea.Cmd)) {
	r.commands = append(r.commands, CommandDef{
		Name:    name,
		Help:    help,
		Leader:  leader,
		Context: context,
		Action:  action,
	})
}

// applyBindings sets key.Binding on each command from config.
func (r *CommandRegistry) applyBindings(bindings map[string]BindingDef) {
	for i := range r.commands {
		cmd := &r.commands[i]
		bd, ok := bindings[cmd.Name]
		if ok {
			// Config overrides leader
			cmd.Leader = bd.Leader
			if bd.Context != "" {
				cmd.Context = bd.Context
			}
			if len(bd.Keys) > 0 {
				helpKey := bd.Keys[0]
				if cmd.Leader {
					leaderHelp := r.leader.Help()
					if leaderHelp.Key != "" {
						helpKey = leaderHelp.Key + " " + helpKey
					}
				}
				cmd.Binding = key.NewBinding(
					key.WithKeys(bd.Keys...),
					key.WithHelp(helpKey, cmd.Help),
				)
			} else {
				cmd.Binding = key.NewBinding(key.WithDisabled())
			}
		} else {
			// No binding config — command exists but unbound
			cmd.Binding = key.NewBinding(key.WithDisabled())
		}
	}
}

// buildIndexes partitions commands into leader/direct/tracer slices.
func (r *CommandRegistry) buildIndexes() {
	r.byName = make(map[string]*CommandDef, len(r.commands))
	r.leaderCmds = nil
	r.directCmds = nil
	r.tracerCmds = nil
	for i := range r.commands {
		cmd := &r.commands[i]
		r.byName[cmd.Name] = cmd
		if cmd.Binding.Enabled() {
			switch {
			case cmd.Context == "tracer":
				r.tracerCmds = append(r.tracerCmds, cmd)
			case cmd.Leader:
				r.leaderCmds = append(r.leaderCmds, cmd)
			default:
				r.directCmds = append(r.directCmds, cmd)
			}
		}
	}
}

// MatchLeader returns the command matching a key press after leader, or nil.
func (r *CommandRegistry) MatchLeader(msg tea.KeyMsg) *CommandDef {
	for _, cmd := range r.leaderCmds {
		if key.Matches(msg, cmd.Binding) {
			return cmd
		}
	}
	return nil
}

// MatchDirect returns the command matching a direct (non-leader) key press, or nil.
func (r *CommandRegistry) MatchDirect(msg tea.KeyMsg) *CommandDef {
	for _, cmd := range r.directCmds {
		if key.Matches(msg, cmd.Binding) {
			return cmd
		}
	}
	return nil
}

// MatchTracer returns the command matching a key press in tracer context, or nil.
func (r *CommandRegistry) MatchTracer(msg tea.KeyMsg) *CommandDef {
	for _, cmd := range r.tracerCmds {
		if key.Matches(msg, cmd.Binding) {
			return cmd
		}
	}
	return nil
}

// ByName returns a command by name, or nil.
func (r *CommandRegistry) ByName(name string) *CommandDef {
	return r.byName[name]
}

// Leader returns the leader key binding.
func (r *CommandRegistry) Leader() key.Binding {
	return r.leader
}

// ShortHelp implements help.KeyMap for the status bar.
func (r *CommandRegistry) ShortHelp() []key.Binding {
	var bindings []key.Binding
	for _, name := range []string{"doc-help", "clear", "quit"} {
		if cmd := r.byName[name]; cmd != nil && cmd.Binding.Enabled() {
			bindings = append(bindings, cmd.Binding)
		}
	}
	return bindings
}

// FullHelp implements help.KeyMap for the full help view.
func (r *CommandRegistry) FullHelp() [][]key.Binding {
	var leader, direct, tracer []key.Binding
	for _, cmd := range r.leaderCmds {
		leader = append(leader, cmd.Binding)
	}
	for _, cmd := range r.directCmds {
		direct = append(direct, cmd.Binding)
	}
	for _, cmd := range r.tracerCmds {
		tracer = append(tracer, cmd.Binding)
	}
	return [][]key.Binding{leader, direct, tracer}
}

// PaletteCommands returns Command entries for the command palette.
func (r *CommandRegistry) PaletteCommands(m *Model) []Command {
	var cmds []Command
	for i := range r.commands {
		cmd := &r.commands[i]
		hint := ""
		if cmd.Binding.Enabled() {
			h := cmd.Binding.Help()
			if h.Key != "" {
				hint = " (" + h.Key + ")"
			}
		}
		help := cmd.Help
		// Dynamic labels
		switch cmd.Name {
		case "autolocalise":
			if m.autolocalise {
				help += " [on]"
			} else {
				help += " [off]"
			}
		}
		cmds = append(cmds, Command{
			Name: cmd.Name,
			Help: help + hint,
		})
	}
	return cmds
}

// buildCommands creates the full command registry with all actions.
func buildCommands(cfg *Config) *CommandRegistry {
	leaderBinding := cfg.LeaderBinding()
	reg := newRegistry(leaderBinding)

	// --- Leader commands (default) ---
	reg.add("debug", "Toggle debug pane", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.toggleDebugPane()
		return *m, nil
	})
	reg.add("stack", "Toggle stack pane", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.toggleStackPane()
		return *m, nil
	})
	reg.add("variables", "Toggle variables pane", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.toggleVariablesPane()
		return *m, nil
	})
	reg.add("breakpoint", "Toggle breakpoint on current line", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.toggleBreakpoint()
		return *m, nil
	})
	reg.add("reconnect", "Reconnect to Dyalog", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		return m.reconnect()
	})
	reg.add("command-palette", "Open command palette", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.openCommandPalette()
		return *m, nil
	})
	reg.add("pane-move", "Pane move mode", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		if m.panes.FocusedPane() != nil {
			m.paneMoveMode = true
		}
		return *m, nil
	})
	reg.add("show-keys", "Show key bindings", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.toggleKeysPane()
		return *m, nil
	})
	reg.add("cycle-pane", "Cycle pane focus", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		if m.panes.HasPanes() {
			m.panes.FocusNext()
		}
		return *m, nil
	})
	reg.add("quit", "Quit gritt", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.confirmQuit = true
		return *m, nil
	})
	reg.add("doc-search", "Search documentation", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		return m.openDocSearch()
	})
	reg.add("ibeam", "I-beam ⌶ lookup", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		return m.openIBeamSearch()
	})
	reg.add("focus-mode", "Toggle focus mode", true, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.focusMode = !m.focusMode
		return *m, nil
	})

	// --- Direct commands (no leader) ---
	reg.add("doc-help", "Context-sensitive documentation", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		return m.openDocHelp()
	})
	reg.add("clear", "Clear session screen", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.clearScreen()
		return *m, nil
	})
	reg.add("autocomplete", "Trigger code completion", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		if fp := m.panes.FocusedPane(); fp != nil {
			if ep, ok := fp.Content.(*EditorPane); ok && !ep.InTracerMode() {
				m.requestAutocomplete(ep.window.Token)
				return *m, nil
			}
			// Other pane focused - autocomplete not applicable
		} else {
			m.requestAutocomplete(0)
			return *m, nil
		}
		return *m, nil
	})
	reg.add("close-pane", "Close pane / exit mode", false, "", nil) // complex — handled inline
	reg.add("history-back", "Previous command", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.historyBack()
		return *m, nil
	})
	reg.add("history-forward", "Next command", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.historyForward()
		return *m, nil
	})
	reg.add("reverse-search", "Search command history", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.openHistorySearch()
		return *m, nil
	})

	// --- Palette-only commands (no default binding) ---
	reg.add("symbols", "Search APL symbols", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.openSymbolSearch()
		return *m, nil
	})
	reg.add("aplcart", "Search APLcart idioms", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		return m.openAPLcart()
	})
	reg.add("cache-refresh", "Re-download docs and APLcart caches", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.log("Refreshing caches...")
		return *m, tea.Batch(RefreshAPLcartCache, RefreshDocsCache)
	})
	reg.add("save", "Save session to file", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.saveSession()
		return *m, nil
	})
	reg.add("load", "Load session from file", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.loadSession()
		return *m, nil
	})
	reg.add("save-config", "Save current config to disk", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.saveConfig()
		return *m, nil
	})
	reg.add("format", "Format code in focused editor", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.formatFocusedEditor()
		return *m, nil
	})
	reg.add("toggle-local", "Toggle localisation of word under cursor", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.toggleLocalisation()
		return *m, nil
	})
	reg.add("localise", "Clean up locals: add missing, remove stale", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.localiseEditor()
		return *m, nil
	})
	reg.add("autolocalise", "Toggle autolocalise mode", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.autolocalise = !m.autolocalise
		return *m, nil
	})
	reg.add("close-all-windows", "Close all editors/tracers", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.closeAllWindows()
		return *m, nil
	})
	reg.add("rebind", "Change key bindings", false, "", func(m *Model) (tea.Model, tea.Cmd) {
		m.openRebindPane()
		return *m, nil
	})

	// --- Tracer commands ---
	reg.add("step-into", "Tracer: step into", false, "tracer", func(m *Model) (tea.Model, tea.Cmd) {
		m.tracerStepInto()
		return *m, nil
	})
	reg.add("step-over", "Tracer: step over", false, "tracer", func(m *Model) (tea.Model, tea.Cmd) {
		m.tracerStepOver()
		return *m, nil
	})
	reg.add("step-out", "Tracer: step out", false, "tracer", func(m *Model) (tea.Model, tea.Cmd) {
		m.tracerStepOut()
		return *m, nil
	})
	reg.add("continue", "Tracer: continue execution", false, "tracer", func(m *Model) (tea.Model, tea.Cmd) {
		m.tracerContinue()
		return *m, nil
	})
	reg.add("resume-all", "Tracer: resume all threads", false, "tracer", func(m *Model) (tea.Model, tea.Cmd) {
		m.tracerResumeAll()
		return *m, nil
	})
	reg.add("trace-back", "Tracer: move backward", false, "tracer", func(m *Model) (tea.Model, tea.Cmd) {
		m.tracerBackward()
		return *m, nil
	})
	reg.add("trace-forward", "Tracer: move forward", false, "tracer", func(m *Model) (tea.Model, tea.Cmd) {
		m.tracerForward()
		return *m, nil
	})
	reg.add("edit-mode", "Tracer: enter edit mode", false, "tracer", nil) // handled in EditorPane

	reg.applyBindings(cfg.Bindings)
	reg.buildIndexes()
	return reg
}
