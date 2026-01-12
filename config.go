package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
)

// Config holds all gritt configuration
type Config struct {
	Keys KeyMapConfig `json:"keys"`
}

// KeyMapConfig defines key bindings in config file format
type KeyMapConfig struct {
	Leader      []string `json:"leader"`
	Execute     []string `json:"execute"`
	ToggleDebug []string `json:"toggle_debug"`
	CyclePane   []string `json:"cycle_pane"`
	ClosePane   []string `json:"close_pane"`
	Quit        []string `json:"quit"`
	ShowKeys    []string `json:"show_keys"`

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

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Keys: KeyMapConfig{
			Leader:      []string{"ctrl+]"},
			Execute:     []string{"enter"},
			ToggleDebug: []string{"d"},      // After leader: C-] d
			CyclePane:   []string{"tab"},
			ClosePane:   []string{"esc"},
			Quit:        []string{"q"},      // After leader: C-] q
			ShowKeys:    []string{"?"},      // After leader: C-] ?

			Up:    []string{"up"},
			Down:  []string{"down"},
			Left:  []string{"left"},
			Right: []string{"right"},
			Home:  []string{"home"},
			End:   []string{"end"},
			PgUp:  []string{"pgup"},
			PgDn:  []string{"pgdown"},

			Backspace: []string{"backspace"},
			Delete:    []string{"delete"},
		},
	}
}

// LoadConfig loads configuration from file or returns defaults
func LoadConfig() Config {
	paths := []string{
		"config.json",
		filepath.Join(os.Getenv("HOME"), ".config", "gritt", "config.json"),
	}

	for _, path := range paths {
		if cfg, err := loadConfigFile(path); err == nil {
			return cfg
		}
	}

	return DefaultConfig()
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

	// Merge with defaults for any missing values
	return mergeWithDefaults(cfg), nil
}

func mergeWithDefaults(cfg Config) Config {
	def := DefaultConfig()

	if len(cfg.Keys.Leader) == 0 {
		cfg.Keys.Leader = def.Keys.Leader
	}
	if len(cfg.Keys.Execute) == 0 {
		cfg.Keys.Execute = def.Keys.Execute
	}
	if len(cfg.Keys.ToggleDebug) == 0 {
		cfg.Keys.ToggleDebug = def.Keys.ToggleDebug
	}
	if len(cfg.Keys.CyclePane) == 0 {
		cfg.Keys.CyclePane = def.Keys.CyclePane
	}
	if len(cfg.Keys.ClosePane) == 0 {
		cfg.Keys.ClosePane = def.Keys.ClosePane
	}
	if len(cfg.Keys.Quit) == 0 {
		cfg.Keys.Quit = def.Keys.Quit
	}
	if len(cfg.Keys.ShowKeys) == 0 {
		cfg.Keys.ShowKeys = def.Keys.ShowKeys
	}
	if len(cfg.Keys.Up) == 0 {
		cfg.Keys.Up = def.Keys.Up
	}
	if len(cfg.Keys.Down) == 0 {
		cfg.Keys.Down = def.Keys.Down
	}
	if len(cfg.Keys.Left) == 0 {
		cfg.Keys.Left = def.Keys.Left
	}
	if len(cfg.Keys.Right) == 0 {
		cfg.Keys.Right = def.Keys.Right
	}
	if len(cfg.Keys.Home) == 0 {
		cfg.Keys.Home = def.Keys.Home
	}
	if len(cfg.Keys.End) == 0 {
		cfg.Keys.End = def.Keys.End
	}
	if len(cfg.Keys.PgUp) == 0 {
		cfg.Keys.PgUp = def.Keys.PgUp
	}
	if len(cfg.Keys.PgDn) == 0 {
		cfg.Keys.PgDn = def.Keys.PgDn
	}
	if len(cfg.Keys.Backspace) == 0 {
		cfg.Keys.Backspace = def.Keys.Backspace
	}
	if len(cfg.Keys.Delete) == 0 {
		cfg.Keys.Delete = def.Keys.Delete
	}

	return cfg
}

// ToKeyMap converts config to KeyMap
func (c *Config) ToKeyMap() KeyMap {
	return KeyMap{
		Leader: key.NewBinding(
			key.WithKeys(c.Keys.Leader...),
			key.WithHelp(c.Keys.Leader[0], "leader"),
		),
		Execute: key.NewBinding(
			key.WithKeys(c.Keys.Execute...),
			key.WithHelp(c.Keys.Execute[0], "execute"),
		),
		ToggleDebug: key.NewBinding(
			key.WithKeys(c.Keys.ToggleDebug...),
			key.WithHelp(c.Keys.Leader[0]+" "+c.Keys.ToggleDebug[0], "debug"),
		),
		CyclePane: key.NewBinding(
			key.WithKeys(c.Keys.CyclePane...),
			key.WithHelp(c.Keys.CyclePane[0], "cycle pane"),
		),
		ClosePane: key.NewBinding(
			key.WithKeys(c.Keys.ClosePane...),
			key.WithHelp(c.Keys.ClosePane[0], "close pane"),
		),
		Quit: key.NewBinding(
			key.WithKeys(c.Keys.Quit...),
			key.WithHelp(c.Keys.Leader[0]+" "+c.Keys.Quit[0], "quit"),
		),
		ShowKeys: key.NewBinding(
			key.WithKeys(c.Keys.ShowKeys...),
			key.WithHelp(c.Keys.Leader[0]+" "+c.Keys.ShowKeys[0], "show keys"),
		),
		Up: key.NewBinding(
			key.WithKeys(c.Keys.Up...),
			key.WithHelp(c.Keys.Up[0], "up"),
		),
		Down: key.NewBinding(
			key.WithKeys(c.Keys.Down...),
			key.WithHelp(c.Keys.Down[0], "down"),
		),
		Left: key.NewBinding(
			key.WithKeys(c.Keys.Left...),
			key.WithHelp(c.Keys.Left[0], "left"),
		),
		Right: key.NewBinding(
			key.WithKeys(c.Keys.Right...),
			key.WithHelp(c.Keys.Right[0], "right"),
		),
		Home: key.NewBinding(
			key.WithKeys(c.Keys.Home...),
			key.WithHelp(c.Keys.Home[0], "line start"),
		),
		End: key.NewBinding(
			key.WithKeys(c.Keys.End...),
			key.WithHelp(c.Keys.End[0], "line end"),
		),
		PgUp: key.NewBinding(
			key.WithKeys(c.Keys.PgUp...),
			key.WithHelp(c.Keys.PgUp[0], "page up"),
		),
		PgDn: key.NewBinding(
			key.WithKeys(c.Keys.PgDn...),
			key.WithHelp(c.Keys.PgDn[0], "page down"),
		),
		Backspace: key.NewBinding(
			key.WithKeys(c.Keys.Backspace...),
			key.WithHelp(c.Keys.Backspace[0], "delete back"),
		),
		Delete: key.NewBinding(
			key.WithKeys(c.Keys.Delete...),
			key.WithHelp(c.Keys.Delete[0], "delete forward"),
		),
	}
}
