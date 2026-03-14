package main

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
)

//go:embed gritt.default.json
var defaultConfigJSON []byte

// BindingDef defines a key binding in the config file.
type BindingDef struct {
	Keys    []string `json:"keys,omitempty"`
	Leader  bool     `json:"leader,omitempty"`
	Context string   `json:"context,omitempty"` // "", "tracer"
}

// NavConfig defines navigation key bindings.
type NavConfig struct {
	Up        []string `json:"up"`
	Down      []string `json:"down"`
	Left      []string `json:"left"`
	Right     []string `json:"right"`
	Home      []string `json:"home"`
	End       []string `json:"end"`
	PgUp      []string `json:"pgup"`
	PgDn      []string `json:"pgdn"`
	Backspace []string `json:"backspace"`
	Delete    []string `json:"delete"`
	Execute   []string `json:"execute"`
}

// NavKeys holds resolved navigation key bindings.
type NavKeys struct {
	Up, Down, Left, Right key.Binding
	Home, End, PgUp, PgDn key.Binding
	Backspace, Delete      key.Binding
	Execute                key.Binding
}

// Config holds all gritt configuration
type Config struct {
	Accent       string                 `json:"accent"`
	Bindings     map[string]BindingDef  `json:"bindings"`
	Navigation   NavConfig              `json:"navigation"`
	Autolocalise bool                   `json:"autolocalise"`

	// Legacy fields for migration
	Keys       *legacyKeyMapConfig     `json:"keys,omitempty"`
	TracerKeys *legacyTracerKeysConfig `json:"tracer_keys,omitempty"`
}

// legacyKeyMapConfig is the old config format
type legacyKeyMapConfig struct {
	Leader           []string `json:"leader"`
	Execute          []string `json:"execute"`
	ToggleDebug      []string `json:"toggle_debug"`
	ToggleStack      []string `json:"toggle_stack"`
	ToggleLocals     []string `json:"toggle_locals"`
	ToggleBreakpoint []string `json:"toggle_breakpoint"`
	Reconnect        []string `json:"reconnect"`
	CommandPalette   []string `json:"command_palette"`
	PaneMoveMode     []string `json:"pane_move_mode"`
	CyclePane        []string `json:"cycle_pane"`
	ClosePane        []string `json:"close_pane"`
	Quit             []string `json:"quit"`
	ShowKeys         []string `json:"show_keys"`
	Autocomplete     []string `json:"autocomplete"`
	DocHelp          []string `json:"doc_help"`
	DocSearch        []string `json:"doc_search"`
	HistoryBack      []string `json:"history_back"`
	HistoryForward   []string `json:"history_forward"`
	ClearScreen      []string `json:"clear_screen"`
	FocusMode        []string `json:"focus_mode"`

	Up    []string `json:"up"`
	Down  []string `json:"down"`
	Left  []string `json:"left"`
	Right []string `json:"right"`
	Home  []string `json:"home"`
	End   []string `json:"end"`
	PgUp  []string `json:"pgup"`
	PgDn  []string `json:"pgdn"`

	Backspace []string `json:"backspace"`
	Delete    []string `json:"delete"`
}

// legacyTracerKeysConfig is the old tracer_keys format
type legacyTracerKeysConfig struct {
	StepOver  string `json:"step_over"`
	StepInto  string `json:"step_into"`
	StepOut   string `json:"step_out"`
	Continue  string `json:"continue"`
	ResumeAll string `json:"resume_all"`
	Backward  string `json:"backward"`
	Forward   string `json:"forward"`
	EditMode  string `json:"edit_mode"`
}

// LoadConfig loads configuration from first found config file
func LoadConfig() Config {
	paths := []string{
		"gritt.json",
		filepath.Join(os.Getenv("HOME"), ".config", "gritt", "gritt.json"),
		"gritt.default.json",
	}

	for _, path := range paths {
		if cfg, err := loadConfigFile(path); err == nil {
			return cfg
		}
	}

	// Fall back to embedded default config
	var cfg Config
	if err := json.Unmarshal(defaultConfigJSON, &cfg); err != nil {
		panic("embedded default config is invalid: " + err.Error())
	}
	return cfg
}

func loadConfigFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// Migrate old format if detected
	if cfg.Keys != nil && cfg.Bindings == nil {
		cfg.migrateFromLegacy()
	}

	return cfg, nil
}

// migrateFromLegacy converts old keys+tracer_keys format to new bindings+navigation.
func (c *Config) migrateFromLegacy() {
	k := c.Keys
	c.Bindings = map[string]BindingDef{
		"leader":          {Keys: k.Leader},
		"debug":           {Keys: k.ToggleDebug, Leader: true},
		"stack":           {Keys: k.ToggleStack, Leader: true},
		"variables":       {Keys: k.ToggleLocals, Leader: true},
		"breakpoint":      {Keys: k.ToggleBreakpoint, Leader: true},
		"reconnect":       {Keys: k.Reconnect, Leader: true},
		"command-palette": {Keys: k.CommandPalette, Leader: true},
		"pane-move":       {Keys: k.PaneMoveMode, Leader: true},
		"cycle-pane":      {Keys: k.CyclePane, Leader: true},
		"close-pane":      {Keys: k.ClosePane},
		"quit":            {Keys: k.Quit, Leader: true},
		"show-keys":       {Keys: k.ShowKeys, Leader: true},
		"autocomplete":    {Keys: k.Autocomplete},
		"doc-help":        {Keys: k.DocHelp},
		"doc-search":      {Keys: k.DocSearch, Leader: true},
		"history-back":    {Keys: k.HistoryBack},
		"history-forward": {Keys: k.HistoryForward},
		"clear":           {Keys: k.ClearScreen},
		"focus-mode":      {Keys: k.FocusMode, Leader: true},
	}

	c.Navigation = NavConfig{
		Up:        k.Up,
		Down:      k.Down,
		Left:      k.Left,
		Right:     k.Right,
		Home:      k.Home,
		End:       k.End,
		PgUp:      k.PgUp,
		PgDn:      k.PgDn,
		Backspace: k.Backspace,
		Delete:    k.Delete,
		Execute:   k.Execute,
	}

	// Migrate tracer keys
	if t := c.TracerKeys; t != nil {
		if t.StepInto != "" {
			c.Bindings["step-into"] = BindingDef{Keys: []string{t.StepInto}, Context: "tracer"}
		}
		if t.StepOver != "" {
			c.Bindings["step-over"] = BindingDef{Keys: []string{t.StepOver}, Context: "tracer"}
		}
		if t.StepOut != "" {
			c.Bindings["step-out"] = BindingDef{Keys: []string{t.StepOut}, Context: "tracer"}
		}
		if t.Continue != "" {
			c.Bindings["continue"] = BindingDef{Keys: []string{t.Continue}, Context: "tracer"}
		}
		if t.ResumeAll != "" {
			c.Bindings["resume-all"] = BindingDef{Keys: []string{t.ResumeAll}, Context: "tracer"}
		}
		if t.Backward != "" {
			c.Bindings["trace-back"] = BindingDef{Keys: []string{t.Backward}, Context: "tracer"}
		}
		if t.Forward != "" {
			c.Bindings["trace-forward"] = BindingDef{Keys: []string{t.Forward}, Context: "tracer"}
		}
		if t.EditMode != "" {
			c.Bindings["edit-mode"] = BindingDef{Keys: []string{t.EditMode}, Context: "tracer"}
		}
	}

	// Clear legacy fields
	c.Keys = nil
	c.TracerKeys = nil
}

// LeaderBinding returns the leader key.Binding from config.
func (c *Config) LeaderBinding() key.Binding {
	bd, ok := c.Bindings["leader"]
	if !ok || len(bd.Keys) == 0 {
		return key.NewBinding(key.WithDisabled())
	}
	return key.NewBinding(
		key.WithKeys(bd.Keys...),
		key.WithHelp(bd.Keys[0], "leader"),
	)
}

// ToNavKeys converts config to NavKeys.
func (c *Config) ToNavKeys() NavKeys {
	return NavKeys{
		Up:        navBinding(c.Navigation.Up, "up"),
		Down:      navBinding(c.Navigation.Down, "down"),
		Left:      navBinding(c.Navigation.Left, "left"),
		Right:     navBinding(c.Navigation.Right, "right"),
		Home:      navBinding(c.Navigation.Home, "line start"),
		End:       navBinding(c.Navigation.End, "line end"),
		PgUp:      navBinding(c.Navigation.PgUp, "page up"),
		PgDn:      navBinding(c.Navigation.PgDn, "page down"),
		Backspace: navBinding(c.Navigation.Backspace, "delete back"),
		Delete:    navBinding(c.Navigation.Delete, "delete forward"),
		Execute:   navBinding(c.Navigation.Execute, "execute"),
	}
}

func navBinding(keys []string, help string) key.Binding {
	if len(keys) == 0 {
		return key.NewBinding(key.WithDisabled())
	}
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(keys[0], help),
	)
}
