package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLoadConfigNil(t *testing.T) {
	// nil → search default hierarchy, ultimately fall back to embedded defaults
	// Run from a temp dir so there's no gritt.json to accidentally pick up
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	cfg := LoadConfig(nil)
	assertValidDefaultConfig(t, cfg)
}

func TestLoadConfigEmptyString(t *testing.T) {
	// *cfgFile == "" → embedded defaults only
	empty := ""
	cfg := LoadConfig(&empty)
	assertValidDefaultConfig(t, cfg)
}

func TestLoadConfigSpecificFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "custom.json")

	customCfg := Config{
		Accent: "#FF0000",
		Bindings: map[string]BindingDef{
			"leader": {Keys: []string{"ctrl+x"}},
			"quit":   {Keys: []string{"q"}, Leader: true},
		},
		Navigation: NavConfig{
			Up:      []string{"k"},
			Down:    []string{"j"},
			Execute: []string{"enter"},
		},
	}

	data, err := json.MarshalIndent(customCfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadConfig(&cfgPath)

	if cfg.Accent != "#FF0000" {
		t.Errorf("Accent = %q, want %q", cfg.Accent, "#FF0000")
	}
	if len(cfg.Bindings) != 2 {
		t.Errorf("len(Bindings) = %d, want 2", len(cfg.Bindings))
	}
	leader, ok := cfg.Bindings["leader"]
	if !ok {
		t.Fatal("missing 'leader' binding")
	}
	if len(leader.Keys) != 1 || leader.Keys[0] != "ctrl+x" {
		t.Errorf("leader keys = %v, want [ctrl+x]", leader.Keys)
	}
	if cfg.Navigation.Up[0] != "k" {
		t.Errorf("Navigation.Up = %v, want [k]", cfg.Navigation.Up)
	}
	if cfg.Navigation.Down[0] != "j" {
		t.Errorf("Navigation.Down = %v, want [j]", cfg.Navigation.Down)
	}
}

func TestLoadConfigMissingFileFatals(t *testing.T) {
	// log.Fatalf calls os.Exit(1), so we re-exec the test binary in a subprocess
	if os.Getenv("GRITT_TEST_FATAL") == "1" {
		bogus := "/no/such/file/gritt-test-config.json"
		LoadConfig(&bogus)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestLoadConfigMissingFileFatals$")
	cmd.Env = append(os.Environ(), "GRITT_TEST_FATAL=1")
	err := cmd.Run()

	if err == nil {
		t.Fatal("expected subprocess to exit with non-zero status")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 0 {
			t.Fatal("expected non-zero exit code")
		}
	}
}

func TestLoadConfigLegacyMigration(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "legacy.json")

	legacy := `{
		"keys": {
			"leader": ["ctrl+]"],
			"execute": ["enter"],
			"toggle_debug": ["ctrl+d"],
			"command_palette": [":"],
			"quit": ["ctrl+q"],
			"up": ["up"],
			"down": ["down"],
			"left": ["left"],
			"right": ["right"],
			"home": ["home"],
			"end": ["end"],
			"pgup": ["pgup"],
			"pgdn": ["pgdown"],
			"backspace": ["backspace"],
			"delete": ["delete"]
		},
		"tracer_keys": {
			"step_over": "n",
			"step_into": "i",
			"continue": "c"
		}
	}`

	if err := os.WriteFile(cfgPath, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadConfig(&cfgPath)

	// Legacy fields should be cleared
	if cfg.Keys != nil {
		t.Error("Keys should be nil after migration")
	}
	if cfg.TracerKeys != nil {
		t.Error("TracerKeys should be nil after migration")
	}

	// Bindings should have been migrated
	if _, ok := cfg.Bindings["leader"]; !ok {
		t.Error("missing migrated 'leader' binding")
	}
	if _, ok := cfg.Bindings["debug"]; !ok {
		t.Error("missing migrated 'debug' binding")
	}
	if _, ok := cfg.Bindings["step-over"]; !ok {
		t.Error("missing migrated 'step-over' binding")
	}
	if _, ok := cfg.Bindings["step-into"]; !ok {
		t.Error("missing migrated 'step-into' binding")
	}
	if _, ok := cfg.Bindings["continue"]; !ok {
		t.Error("missing migrated 'continue' binding")
	}

	// Navigation should have been migrated
	if len(cfg.Navigation.Up) == 0 || cfg.Navigation.Up[0] != "up" {
		t.Errorf("Navigation.Up = %v, want [up]", cfg.Navigation.Up)
	}
	if len(cfg.Navigation.Execute) == 0 || cfg.Navigation.Execute[0] != "enter" {
		t.Errorf("Navigation.Execute = %v, want [enter]", cfg.Navigation.Execute)
	}
}

// assertValidDefaultConfig checks that a config loaded from embedded defaults
// has the expected structure.
func assertValidDefaultConfig(t *testing.T, cfg Config) {
	t.Helper()

	// Bindings should be populated
	if cfg.Bindings == nil {
		t.Fatal("Bindings is nil")
	}

	expectedBindings := []string{
		"leader", "debug", "stack", "variables", "quit",
		"command-palette", "autocomplete", "close-pane",
		"step-into", "step-over", "continue",
	}
	for _, name := range expectedBindings {
		if _, ok := cfg.Bindings[name]; !ok {
			t.Errorf("missing expected binding %q", name)
		}
	}

	// Leader should have keys
	leader := cfg.Bindings["leader"]
	if len(leader.Keys) == 0 {
		t.Error("leader binding has no keys")
	}

	// Navigation should be populated
	nav := cfg.Navigation
	if len(nav.Up) == 0 {
		t.Error("Navigation.Up is empty")
	}
	if len(nav.Down) == 0 {
		t.Error("Navigation.Down is empty")
	}
	if len(nav.Left) == 0 {
		t.Error("Navigation.Left is empty")
	}
	if len(nav.Right) == 0 {
		t.Error("Navigation.Right is empty")
	}
	if len(nav.Execute) == 0 {
		t.Error("Navigation.Execute is empty")
	}
	if len(nav.Backspace) == 0 {
		t.Error("Navigation.Backspace is empty")
	}
	if len(nav.Delete) == 0 {
		t.Error("Navigation.Delete is empty")
	}

	// Tracer bindings should have context set
	stepInto, ok := cfg.Bindings["step-into"]
	if ok && stepInto.Context != "tracer" {
		t.Errorf("step-into context = %q, want %q", stepInto.Context, "tracer")
	}
}
