package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testRebindPane() *RebindPane {
	reg := testRegistry() // from commands_test.go
	bindings := map[string]BindingDef{
		"debug":     {Keys: []string{"d"}, Leader: true},
		"stack":     {Keys: []string{"s"}, Leader: true},
		"clear":     {Keys: []string{"ctrl+l"}},
		"doc-help":  {Keys: []string{"f1"}},
		"step-into": {Keys: []string{"i"}, Context: "tracer"},
		"step-over": {Keys: []string{"n"}, Context: "tracer"},
		"symbols":   {},
		"unbound":   {},
	}
	return NewRebindPane(reg, bindings)
}

func TestRebindPaneCreation(t *testing.T) {
	rp := testRebindPane()
	if len(rp.entries) == 0 {
		t.Fatal("no entries")
	}
	// "leader" should be excluded
	for _, e := range rp.entries {
		if e.name == "leader" {
			t.Error("leader should not appear in rebind pane")
		}
	}
}

func TestRebindPaneTitle(t *testing.T) {
	rp := testRebindPane()
	if rp.Title() != "rebind keys" {
		t.Errorf("Title() = %q, want %q", rp.Title(), "rebind keys")
	}
}

func TestRebindPaneTitleCapturing(t *testing.T) {
	rp := testRebindPane()
	rp.capturing = true
	title := rp.Title()
	if !strings.Contains(title, "press key for") {
		t.Errorf("Title() during capture = %q, want 'press key for: ...'", title)
	}
}

func TestRebindPaneNavigation(t *testing.T) {
	rp := testRebindPane()
	if rp.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", rp.selected)
	}

	rp.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if rp.selected != 1 {
		t.Errorf("after down, selected = %d, want 1", rp.selected)
	}

	rp.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if rp.selected != 0 {
		t.Errorf("after up, selected = %d, want 0", rp.selected)
	}

	// Can't go above 0
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if rp.selected != 0 {
		t.Errorf("should stay at 0, got %d", rp.selected)
	}
}

func TestRebindPaneCaptureKey(t *testing.T) {
	rp := testRebindPane()

	// Enter capture mode
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !rp.capturing {
		t.Fatal("should be in capture mode after Enter")
	}

	// Press 'x' to bind
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if rp.capturing {
		t.Error("should exit capture mode after key press")
	}
	if !rp.PendingApply {
		t.Fatal("PendingApply should be true")
	}
	if rp.PendingName != rp.entries[0].name {
		t.Errorf("PendingName = %q, want %q", rp.PendingName, rp.entries[0].name)
	}
	if len(rp.PendingBinding.Keys) != 1 || rp.PendingBinding.Keys[0] != "x" {
		t.Errorf("PendingBinding.Keys = %v, want [x]", rp.PendingBinding.Keys)
	}
}

func TestRebindPaneCaptureEscape(t *testing.T) {
	rp := testRebindPane()
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !rp.capturing {
		t.Fatal("should be capturing")
	}

	rp.HandleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if rp.capturing {
		t.Error("should exit capture on Escape")
	}
	if rp.PendingApply {
		t.Error("no change should be pending after cancel")
	}
}

func TestRebindPaneToggleLeader(t *testing.T) {
	rp := testRebindPane()

	// Find the entry and check initial leader state
	initial := rp.entries[rp.selected].leader

	rp.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	if !rp.PendingApply {
		t.Fatal("PendingApply should be true after Tab")
	}
	if rp.PendingBinding.Leader == initial {
		t.Error("leader should have toggled")
	}

	// Toggle back
	rp.PendingApply = false
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	if rp.PendingBinding.Leader != initial {
		t.Error("leader should have toggled back")
	}
}

func TestRebindPaneToggleLeaderBlockedForTracer(t *testing.T) {
	rp := testRebindPane()

	// Navigate to a tracer entry
	for i, e := range rp.entries {
		if e.context == "tracer" {
			rp.selected = i
			break
		}
	}

	wasLeader := rp.entries[rp.selected].leader
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	// Should NOT change for tracer context
	if rp.entries[rp.selected].leader != wasLeader {
		t.Error("leader should not toggle for tracer commands")
	}
}

func TestRebindPaneUnbind(t *testing.T) {
	rp := testRebindPane()

	// Verify the first entry has keys
	if len(rp.entries[0].keys) == 0 {
		// Navigate to one that does
		for i, e := range rp.entries {
			if len(e.keys) > 0 {
				rp.selected = i
				break
			}
		}
	}

	rp.HandleKey(tea.KeyMsg{Type: tea.KeyDelete})
	if !rp.PendingApply {
		t.Fatal("PendingApply should be true after Delete")
	}
	if len(rp.PendingBinding.Keys) != 0 {
		t.Errorf("Keys should be empty after unbind, got %v", rp.PendingBinding.Keys)
	}
}

func TestRebindPaneCaptureFunctionKey(t *testing.T) {
	rp := testRebindPane()
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})

	rp.HandleKey(tea.KeyMsg{Type: tea.KeyF5})
	if !rp.PendingApply {
		t.Fatal("PendingApply should be true")
	}
	if len(rp.PendingBinding.Keys) != 1 || rp.PendingBinding.Keys[0] != "f5" {
		t.Errorf("Keys = %v, want [f5]", rp.PendingBinding.Keys)
	}
}

func TestRebindPaneAutoAdvance(t *testing.T) {
	rp := testRebindPane()
	initial := rp.selected

	// Capture a key
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})

	if rp.selected != initial+1 {
		t.Errorf("selected = %d, want %d (should auto-advance)", rp.selected, initial+1)
	}
}

func TestRebindPaneRender(t *testing.T) {
	rp := testRebindPane()
	out := rp.Render(50, 10)
	if out == "" {
		t.Error("Render returned empty string")
	}
	// Should contain hint line
	if !strings.Contains(out, "Enter:bind") {
		t.Error("should contain hint text")
	}
}

func TestRebindPanePreservesContext(t *testing.T) {
	rp := testRebindPane()

	// Navigate to tracer entry
	for i, e := range rp.entries {
		if e.context == "tracer" {
			rp.selected = i
			break
		}
	}

	// Rebind it
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rp.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})

	if rp.PendingBinding.Context != "tracer" {
		t.Errorf("Context = %q, want %q (should preserve)", rp.PendingBinding.Context, "tracer")
	}
}
