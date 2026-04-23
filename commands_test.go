package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Config loading ---

func TestLoadDefaultConfig(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal(defaultConfigJSON, &cfg); err != nil {
		t.Fatalf("embedded default config failed to parse: %v", err)
	}
	if cfg.Bindings == nil {
		t.Fatal("Bindings is nil")
	}
	if len(cfg.Bindings) == 0 {
		t.Fatal("Bindings is empty")
	}

	// Spot-check key entries
	leader := cfg.Bindings["leader"]
	if len(leader.Keys) != 1 || leader.Keys[0] != "ctrl+]" {
		t.Errorf("leader keys = %v, want [ctrl+]]", leader.Keys)
	}

	debug := cfg.Bindings["debug"]
	if !debug.Leader {
		t.Error("debug.Leader = false, want true")
	}
	if len(debug.Keys) != 1 || debug.Keys[0] != "d" {
		t.Errorf("debug keys = %v, want [d]", debug.Keys)
	}

	stepInto := cfg.Bindings["step-into"]
	if stepInto.Context != "tracer" {
		t.Errorf("step-into.Context = %q, want %q", stepInto.Context, "tracer")
	}

	// Palette-only commands have no keys
	symbols := cfg.Bindings["symbols"]
	if len(symbols.Keys) != 0 {
		t.Errorf("symbols.Keys = %v, want empty", symbols.Keys)
	}

	// Navigation
	if len(cfg.Navigation.Up) != 1 || cfg.Navigation.Up[0] != "up" {
		t.Errorf("navigation.Up = %v, want [up]", cfg.Navigation.Up)
	}
	if len(cfg.Navigation.Execute) != 1 || cfg.Navigation.Execute[0] != "enter" {
		t.Errorf("navigation.Execute = %v, want [enter]", cfg.Navigation.Execute)
	}
}

func TestLegacyMigration(t *testing.T) {
	oldJSON := `{
		"keys": {
			"leader": ["ctrl+]"],
			"execute": ["enter"],
			"toggle_debug": ["d"],
			"toggle_stack": ["s"],
			"toggle_locals": ["l"],
			"toggle_breakpoint": ["b"],
			"reconnect": ["r"],
			"command_palette": [":"],
			"pane_move_mode": ["m"],
			"cycle_pane": ["n"],
			"close_pane": ["esc"],
			"quit": ["q"],
			"show_keys": ["?"],
			"autocomplete": ["tab"],
			"doc_help": ["f1"],
			"doc_search": ["/"],
			"history_back": ["ctrl+shift+up"],
			"history_forward": ["ctrl+shift+down"],
			"clear_screen": ["ctrl+l"],
			"focus_mode": ["f"],
			"up": ["up"], "down": ["down"], "left": ["left"], "right": ["right"],
			"home": ["home"], "end": ["end"], "pgup": ["pgup"], "pgdn": ["pgdown"],
			"backspace": ["backspace"], "delete": ["delete"]
		},
		"tracer_keys": {
			"step_over": "n",
			"step_into": "i",
			"step_out": "o",
			"continue": "c",
			"resume_all": "r",
			"backward": "p",
			"forward": "f",
			"edit_mode": "e"
		}
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(oldJSON), &cfg); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Should detect old format and migrate
	if cfg.Keys == nil {
		t.Fatal("Keys should be non-nil before migration")
	}
	cfg.migrateFromLegacy()

	// Legacy fields cleared
	if cfg.Keys != nil {
		t.Error("Keys should be nil after migration")
	}
	if cfg.TracerKeys != nil {
		t.Error("TracerKeys should be nil after migration")
	}

	// Bindings populated
	if cfg.Bindings == nil {
		t.Fatal("Bindings is nil after migration")
	}

	// Check leader commands migrated with leader=true
	for _, name := range []string{"debug", "stack", "variables", "quit", "show-keys", "focus-mode"} {
		bd, ok := cfg.Bindings[name]
		if !ok {
			t.Errorf("missing binding for %q", name)
			continue
		}
		if !bd.Leader {
			t.Errorf("%s.Leader = false, want true", name)
		}
	}

	// Check direct commands migrated without leader
	for _, name := range []string{"close-pane", "autocomplete", "doc-help", "clear"} {
		bd, ok := cfg.Bindings[name]
		if !ok {
			t.Errorf("missing binding for %q", name)
			continue
		}
		if bd.Leader {
			t.Errorf("%s.Leader = true, want false", name)
		}
	}

	// Check tracer keys migrated with context
	for _, name := range []string{"step-into", "step-over", "step-out", "continue", "resume-all", "trace-back", "trace-forward", "edit-mode"} {
		bd, ok := cfg.Bindings[name]
		if !ok {
			t.Errorf("missing tracer binding for %q", name)
			continue
		}
		if bd.Context != "tracer" {
			t.Errorf("%s.Context = %q, want %q", name, bd.Context, "tracer")
		}
	}

	// Check specific key values survived
	if k := cfg.Bindings["step-into"].Keys; len(k) != 1 || k[0] != "i" {
		t.Errorf("step-into.Keys = %v, want [i]", k)
	}
	if k := cfg.Bindings["debug"].Keys; len(k) != 1 || k[0] != "d" {
		t.Errorf("debug.Keys = %v, want [d]", k)
	}

	// Navigation migrated
	if cfg.Navigation.Execute[0] != "enter" {
		t.Errorf("Navigation.Execute = %v, want [enter]", cfg.Navigation.Execute)
	}
	if cfg.Navigation.Up[0] != "up" {
		t.Errorf("Navigation.Up = %v, want [up]", cfg.Navigation.Up)
	}
}

func TestLegacyMigrationTriggeredOnLoad(t *testing.T) {
	oldJSON := `{
		"keys": {
			"leader": ["ctrl+]"],
			"toggle_debug": ["d"]
		}
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(oldJSON), &cfg); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Simulate what loadConfigFile does
	if cfg.Keys != nil && cfg.Bindings == nil {
		cfg.migrateFromLegacy()
	}

	if cfg.Bindings == nil {
		t.Fatal("migration should have created Bindings")
	}
	if _, ok := cfg.Bindings["debug"]; !ok {
		t.Error("toggle_debug should migrate to debug binding")
	}
}

// --- NavKeys ---

func TestToNavKeys(t *testing.T) {
	cfg := Config{
		Navigation: NavConfig{
			Up:      []string{"up"},
			Down:    []string{"down"},
			Execute: []string{"enter"},
		},
	}
	nav := cfg.ToNavKeys()

	if !nav.Up.Enabled() {
		t.Error("Up should be enabled")
	}
	if !nav.Execute.Enabled() {
		t.Error("Execute should be enabled")
	}
	// Unset keys → disabled
	if nav.Home.Enabled() {
		t.Error("Home should be disabled (no keys set)")
	}
}

// --- CommandRegistry ---

func testRegistry() *CommandRegistry {
	cfg := Config{
		Bindings: map[string]BindingDef{
			"leader":          {Keys: []string{"ctrl+]"}},
			"debug":           {Keys: []string{"d"}, Leader: true},
			"stack":           {Keys: []string{"s"}, Leader: true},
			"clear":           {Keys: []string{"ctrl+l"}},
			"doc-help":        {Keys: []string{"f1"}},
			"command-palette": {Keys: []string{":"}, Leader: true},
			"quit":            {Keys: []string{"q"}, Leader: true},
			"history-back":    {Keys: []string{"ctrl+shift+up"}},
			"step-into":      {Keys: []string{"i"}, Context: "tracer"},
			"step-over":      {Keys: []string{"n"}, Context: "tracer"},
			"symbols":        {}, // palette only
		},
	}
	reg := newRegistry(cfg.LeaderBinding())

	// Minimal registrations — just enough to test dispatch
	noop := func(m *Model) (tea.Model, tea.Cmd) { return *m, nil }
	reg.add("debug", "Toggle debug pane", true, "", noop)
	reg.add("stack", "Toggle stack pane", true, "", noop)
	reg.add("clear", "Clear session screen", false, "", noop)
	reg.add("doc-help", "Documentation", false, "", noop)
	reg.add("command-palette", "Open command palette", true, "", noop)
	reg.add("quit", "Quit gritt", true, "", noop)
	reg.add("history-back", "History", false, "", noop)
	reg.add("step-into", "Step into", false, "tracer", noop)
	reg.add("step-over", "Step over", false, "tracer", noop)
	reg.add("symbols", "Search symbols", false, "", noop)
	reg.add("unbound", "Never bound", false, "", noop)

	reg.applyBindings(cfg.Bindings)
	reg.buildIndexes()
	return reg
}

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func TestByName(t *testing.T) {
	reg := testRegistry()

	if cmd := reg.ByName("debug"); cmd == nil {
		t.Error("ByName(debug) returned nil")
	} else if cmd.Name != "debug" {
		t.Errorf("ByName(debug).Name = %q", cmd.Name)
	}

	if cmd := reg.ByName("nonexistent"); cmd != nil {
		t.Errorf("ByName(nonexistent) should be nil, got %q", cmd.Name)
	}
}

func TestMatchLeader(t *testing.T) {
	reg := testRegistry()

	cmd := reg.MatchLeader(keyMsg("d"))
	if cmd == nil {
		t.Fatal("MatchLeader(d) returned nil")
	}
	if cmd.Name != "debug" {
		t.Errorf("MatchLeader(d).Name = %q, want debug", cmd.Name)
	}

	cmd = reg.MatchLeader(keyMsg("s"))
	if cmd == nil {
		t.Fatal("MatchLeader(s) returned nil")
	}
	if cmd.Name != "stack" {
		t.Errorf("MatchLeader(s).Name = %q, want stack", cmd.Name)
	}

	// Direct key should NOT match as leader
	cmd = reg.MatchLeader(keyMsg("i"))
	if cmd != nil {
		t.Errorf("MatchLeader(i) should be nil (tracer context), got %q", cmd.Name)
	}
}

func TestMatchDirect(t *testing.T) {
	reg := testRegistry()

	// Leader commands should NOT match as direct
	cmd := reg.MatchDirect(keyMsg("d"))
	if cmd != nil {
		t.Errorf("MatchDirect(d) should be nil (leader cmd), got %q", cmd.Name)
	}

	// Tracer commands should NOT match as direct
	cmd = reg.MatchDirect(keyMsg("i"))
	if cmd != nil {
		t.Errorf("MatchDirect(i) should be nil (tracer cmd), got %q", cmd.Name)
	}
}

func TestMatchTracer(t *testing.T) {
	reg := testRegistry()

	cmd := reg.MatchTracer(keyMsg("i"))
	if cmd == nil {
		t.Fatal("MatchTracer(i) returned nil")
	}
	if cmd.Name != "step-into" {
		t.Errorf("MatchTracer(i).Name = %q, want step-into", cmd.Name)
	}

	cmd = reg.MatchTracer(keyMsg("n"))
	if cmd == nil {
		t.Fatal("MatchTracer(n) returned nil")
	}
	if cmd.Name != "step-over" {
		t.Errorf("MatchTracer(n).Name = %q, want step-over", cmd.Name)
	}

	// Non-tracer key
	cmd = reg.MatchTracer(keyMsg("d"))
	if cmd != nil {
		t.Errorf("MatchTracer(d) should be nil, got %q", cmd.Name)
	}
}

func TestIndexPartitioning(t *testing.T) {
	reg := testRegistry()

	// Leader commands
	leaderNames := map[string]bool{}
	for _, cmd := range reg.leaderCmds {
		leaderNames[cmd.Name] = true
	}
	if !leaderNames["debug"] || !leaderNames["stack"] {
		t.Errorf("leaderCmds = %v, want debug and stack", leaderNames)
	}

	// Direct commands
	directNames := map[string]bool{}
	for _, cmd := range reg.directCmds {
		directNames[cmd.Name] = true
	}
	if !directNames["clear"] || !directNames["doc-help"] {
		t.Errorf("directCmds = %v, want clear and doc-help", directNames)
	}

	// Tracer commands
	tracerNames := map[string]bool{}
	for _, cmd := range reg.tracerCmds {
		tracerNames[cmd.Name] = true
	}
	if !tracerNames["step-into"] || !tracerNames["step-over"] {
		t.Errorf("tracerCmds = %v, want step-into and step-over", tracerNames)
	}

	// Unbound commands should not appear in any index
	for _, cmd := range reg.leaderCmds {
		if cmd.Name == "symbols" || cmd.Name == "unbound" {
			t.Errorf("unbound command %q should not be in leaderCmds", cmd.Name)
		}
	}
	for _, cmd := range reg.directCmds {
		if cmd.Name == "symbols" || cmd.Name == "unbound" {
			t.Errorf("unbound command %q should not be in directCmds", cmd.Name)
		}
	}
}

func TestUnboundCommandStillAccessibleByName(t *testing.T) {
	reg := testRegistry()

	cmd := reg.ByName("symbols")
	if cmd == nil {
		t.Fatal("symbols should be accessible by name")
	}
	if cmd.Binding.Enabled() {
		t.Error("symbols binding should be disabled (no keys)")
	}

	cmd = reg.ByName("unbound")
	if cmd == nil {
		t.Fatal("unbound should be accessible by name")
	}
	if cmd.Binding.Enabled() {
		t.Error("unbound binding should be disabled")
	}
}

func TestLeaderHelpPrefix(t *testing.T) {
	reg := testRegistry()

	cmd := reg.ByName("debug")
	if cmd == nil {
		t.Fatal("debug not found")
	}

	h := cmd.Binding.Help()
	if h.Key != "ctrl+] d" {
		t.Errorf("debug help key = %q, want %q", h.Key, "ctrl+] d")
	}
}

func TestDirectHelpNoPrefix(t *testing.T) {
	reg := testRegistry()

	cmd := reg.ByName("doc-help")
	if cmd == nil {
		t.Fatal("doc-help not found")
	}

	h := cmd.Binding.Help()
	if h.Key != "f1" {
		t.Errorf("doc-help help key = %q, want %q", h.Key, "f1")
	}
}

// --- buildCommands with real config ---

func TestBuildCommandsFromDefaults(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal(defaultConfigJSON, &cfg); err != nil {
		t.Fatalf("parse default config: %v", err)
	}

	reg := buildCommands(&cfg)

	// All commands should be in byName
	for _, name := range []string{
		"debug", "stack", "variables", "breakpoint", "reconnect",
		"command-palette", "pane-move", "show-keys", "cycle-pane", "quit",
		"doc-search", "focus-mode",
		"doc-help", "clear", "autocomplete", "close-pane",
		"history-back", "history-forward",
		"symbols", "aplcart", "cache-refresh", "save", "load", "save-config",
		"format", "toggle-local", "localise", "autolocalise", "close-all-windows",
		"step-into", "step-over", "step-out", "continue", "resume-all",
		"trace-back", "trace-forward", "edit-mode",
	} {
		if reg.ByName(name) == nil {
			t.Errorf("missing command %q", name)
		}
	}

	// Leader key should be set
	if !reg.Leader().Enabled() {
		t.Error("leader binding should be enabled")
	}
	if h := reg.Leader().Help(); h.Key != "ctrl+]" {
		t.Errorf("leader key = %q, want ctrl+]", h.Key)
	}

	// Check partitioning counts are reasonable
	if len(reg.leaderCmds) < 5 {
		t.Errorf("expected at least 5 leader commands, got %d", len(reg.leaderCmds))
	}
	if len(reg.directCmds) < 3 {
		t.Errorf("expected at least 3 direct commands, got %d", len(reg.directCmds))
	}
	if len(reg.tracerCmds) < 5 {
		t.Errorf("expected at least 5 tracer commands, got %d", len(reg.tracerCmds))
	}

	// Spot-check: debug should be leader, step-into should be tracer
	debug := reg.ByName("debug")
	if !debug.Leader {
		t.Error("debug should be a leader command")
	}
	stepInto := reg.ByName("step-into")
	if stepInto.Context != "tracer" {
		t.Error("step-into should have tracer context")
	}

	// Palette-only commands should exist but be disabled
	symbols := reg.ByName("symbols")
	if symbols.Binding.Enabled() {
		t.Error("symbols should have no keybinding (palette only)")
	}
}

func TestSingleLetterDirectBindingWarning(t *testing.T) {
	// Bind "x" as a direct (non-leader) command — should trigger a warning
	cfg := Config{
		Bindings: map[string]BindingDef{
			"leader":   {Keys: []string{"ctrl+]"}},
			"debug":    {Keys: []string{"x"}}, // bare letter, no leader
			"doc-help": {Keys: []string{"f1"}},
		},
	}
	reg := newRegistry(cfg.LeaderBinding())
	noop := func(m *Model) (tea.Model, tea.Cmd) { return *m, nil }
	reg.add("debug", "Toggle debug pane", false, "", noop)
	reg.add("doc-help", "Documentation", false, "", noop)
	reg.applyBindings(cfg.Bindings)

	warnings := reg.buildIndexes()

	if len(warnings) == 0 {
		t.Fatal("expected warning about single-letter direct binding 'x'")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, `"debug"`) && strings.Contains(w, `"x"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("warning should mention command name and key, got: %v", warnings)
	}
}

// --- ShortHelp / FullHelp ---

func TestShortHelp(t *testing.T) {
	reg := testRegistry()

	bindings := reg.ShortHelp()
	names := map[string]bool{}
	for _, b := range bindings {
		h := b.Help()
		names[h.Desc] = true
	}
	if !names["Documentation"] {
		t.Error("ShortHelp should include doc-help")
	}
	if !names["Open command palette"] {
		t.Error("ShortHelp should include command-palette")
	}
	if !names["Quit gritt"] {
		t.Error("ShortHelp should include quit")
	}
	if !names["History"] {
		t.Error("ShortHelp should include history-back")
	}
}

func TestFullHelp(t *testing.T) {
	reg := testRegistry()

	groups := reg.FullHelp()
	if len(groups) != 3 {
		t.Fatalf("FullHelp returned %d groups, want 3", len(groups))
	}
	// group[0] = leader, group[1] = direct, group[2] = tracer
	if len(groups[0]) != 4 { // debug, stack, command-palette, quit
		t.Errorf("leader group has %d bindings, want 4", len(groups[0]))
	}
	if len(groups[2]) != 2 { // step-into, step-over
		t.Errorf("tracer group has %d bindings, want 2", len(groups[2]))
	}
}

// --- Config roundtrip ---

func TestConfigRoundtrip(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal(defaultConfigJSON, &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var cfg2 Config
	if err := json.Unmarshal(data, &cfg2); err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	// Spot check — leader should survive roundtrip
	if len(cfg2.Bindings["leader"].Keys) != 1 || cfg2.Bindings["leader"].Keys[0] != "ctrl+]" {
		t.Errorf("leader didn't roundtrip: %v", cfg2.Bindings["leader"])
	}

	// leader=true should survive
	if !cfg2.Bindings["debug"].Leader {
		t.Error("debug.Leader didn't roundtrip")
	}

	// context should survive
	if cfg2.Bindings["step-into"].Context != "tracer" {
		t.Error("step-into.Context didn't roundtrip")
	}

	// Legacy fields should NOT appear in output
	if cfg2.Keys != nil {
		t.Error("legacy Keys should not appear after roundtrip")
	}
}

// --- Rebinding at runtime ---

func TestRebindAtRuntime(t *testing.T) {
	cfg := Config{
		Bindings: map[string]BindingDef{
			"leader": {Keys: []string{"ctrl+]"}},
			"debug":  {Keys: []string{"d"}, Leader: true},
			"clear":  {Keys: []string{"ctrl+l"}},
		},
	}
	reg := newRegistry(cfg.LeaderBinding())
	noop := func(m *Model) (tea.Model, tea.Cmd) { return *m, nil }
	reg.add("debug", "Toggle debug pane", true, "", noop)
	reg.add("clear", "Clear session screen", false, "", noop)
	reg.applyBindings(cfg.Bindings)
	reg.buildIndexes()

	// Verify initial state
	if cmd := reg.MatchLeader(keyMsg("d")); cmd == nil || cmd.Name != "debug" {
		t.Fatal("debug should match leader d initially")
	}

	// Rebind debug to 'x' and make it direct
	cfg.Bindings["debug"] = BindingDef{Keys: []string{"x"}, Leader: false}
	reg.applyBindings(cfg.Bindings)
	reg.buildIndexes()

	// Old binding should no longer match as leader
	if cmd := reg.MatchLeader(keyMsg("d")); cmd != nil {
		t.Error("d should no longer match after rebind")
	}
	if cmd := reg.MatchLeader(keyMsg("x")); cmd != nil {
		t.Error("x should not match as leader (now direct)")
	}

	// New binding should match as direct
	if cmd := reg.MatchDirect(keyMsg("x")); cmd == nil || cmd.Name != "debug" {
		t.Fatal("x should match as direct after rebind")
	}
}

func TestLeaderBinding(t *testing.T) {
	cfg := Config{
		Bindings: map[string]BindingDef{
			"leader": {Keys: []string{"ctrl+]"}},
		},
	}
	lb := cfg.LeaderBinding()
	if !lb.Enabled() {
		t.Error("leader should be enabled")
	}

	// No leader
	cfg2 := Config{Bindings: map[string]BindingDef{}}
	lb2 := cfg2.LeaderBinding()
	if lb2.Enabled() {
		t.Error("leader should be disabled when not configured")
	}
}

func TestEmptyBindingsConfig(t *testing.T) {
	cfg := Config{
		Bindings: map[string]BindingDef{},
	}
	reg := newRegistry(key.NewBinding(key.WithDisabled()))
	noop := func(m *Model) (tea.Model, tea.Cmd) { return *m, nil }
	reg.add("debug", "Toggle debug pane", true, "", noop)
	reg.applyBindings(cfg.Bindings)
	reg.buildIndexes()

	// Command exists but is unbound
	cmd := reg.ByName("debug")
	if cmd == nil {
		t.Fatal("debug should exist")
	}
	if cmd.Binding.Enabled() {
		t.Error("debug should be disabled with empty config")
	}
	if len(reg.leaderCmds) != 0 {
		t.Error("no commands should be in leader index")
	}
}
