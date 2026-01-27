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

// Config holds all gritt configuration
type Config struct {
	Accent     string           `json:"accent"`
	Keys       KeyMapConfig     `json:"keys"`
	TracerKeys TracerKeysConfig `json:"tracer_keys"`
}

// TracerKeysConfig defines single-key bindings for tracer mode
type TracerKeysConfig struct {
	StepOver  string `json:"step_over"`
	StepInto  string `json:"step_into"`
	StepOut   string `json:"step_out"`
	Continue  string `json:"continue"`
	ResumeAll string `json:"resume_all"`
	Backward  string `json:"backward"`
	Forward   string `json:"forward"`
	EditMode  string `json:"edit_mode"`
}

// KeyMapConfig defines key bindings in config file format
type KeyMapConfig struct {
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

	return cfg, nil
}

// ToKeyMap converts config to KeyMap
func (c *Config) ToKeyMap() KeyMap {
	return KeyMap{
		Leader:           c.binding(c.Keys.Leader, "", "leader"),
		Execute:          c.binding(c.Keys.Execute, "", "execute"),
		ToggleDebug:      c.bindingWithLeader(c.Keys.ToggleDebug, "debug"),
		ToggleStack:      c.bindingWithLeader(c.Keys.ToggleStack, "stack"),
		ToggleLocals:     c.bindingWithLeader(c.Keys.ToggleLocals, "locals"),
		ToggleBreakpoint: c.bindingWithLeader(c.Keys.ToggleBreakpoint, "breakpoint"),
		Reconnect:        c.bindingWithLeader(c.Keys.Reconnect, "reconnect"),
		CommandPalette:   c.bindingWithLeader(c.Keys.CommandPalette, "commands"),
		PaneMoveMode:     c.bindingWithLeader(c.Keys.PaneMoveMode, "move pane"),
		CyclePane:        c.bindingWithLeader(c.Keys.CyclePane, "cycle pane"),
		ClosePane:        c.binding(c.Keys.ClosePane, "", "close pane"),
		Quit:             c.bindingWithLeader(c.Keys.Quit, "quit"),
		ShowKeys:         c.bindingWithLeader(c.Keys.ShowKeys, "show keys"),
		Autocomplete:     c.binding(c.Keys.Autocomplete, "", "autocomplete"),
		DocHelp:          c.binding(c.Keys.DocHelp, "", "doc help"),
		Up:          c.binding(c.Keys.Up, "", "up"),
		Down:        c.binding(c.Keys.Down, "", "down"),
		Left:        c.binding(c.Keys.Left, "", "left"),
		Right:       c.binding(c.Keys.Right, "", "right"),
		Home:        c.binding(c.Keys.Home, "", "line start"),
		End:         c.binding(c.Keys.End, "", "line end"),
		PgUp:        c.binding(c.Keys.PgUp, "", "page up"),
		PgDn:        c.binding(c.Keys.PgDn, "", "page down"),
		Backspace:   c.binding(c.Keys.Backspace, "", "delete back"),
		Delete:      c.binding(c.Keys.Delete, "", "delete forward"),
	}
}

// binding creates a key binding, returning disabled binding if keys is empty
func (c *Config) binding(keys []string, prefix, help string) key.Binding {
	if len(keys) == 0 {
		return key.NewBinding(key.WithDisabled())
	}
	helpText := help
	if prefix != "" {
		helpText = prefix + " " + keys[0]
	} else {
		helpText = keys[0]
	}
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(helpText, help),
	)
}

// bindingWithLeader creates a key binding with leader prefix in help text
func (c *Config) bindingWithLeader(keys []string, help string) key.Binding {
	if len(keys) == 0 {
		return key.NewBinding(key.WithDisabled())
	}
	prefix := ""
	if len(c.Keys.Leader) > 0 {
		prefix = c.Keys.Leader[0]
	}
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(prefix+" "+keys[0], help),
	)
}
