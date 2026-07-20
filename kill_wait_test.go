package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKillWaitPaneRender(t *testing.T) {
	initColors(0, "")
	p := &KillWaitPane{secondsLeft: 7}
	out := stripANSI(p.Render(60, 7))

	for _, want := range []string{"Waiting for Dyalog to terminate", "7", "esc", "Cancel", "k", "Kill now"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render missing %q in:\n%s", want, out)
		}
	}
	if got := p.Title(); got != "Terminating Dyalog" {
		t.Errorf("Title() = %q, want %q", got, "Terminating Dyalog")
	}
}

func TestKillTimeoutDefault(t *testing.T) {
	var def Config
	if err := json.Unmarshal(defaultConfigJSON, &def); err != nil {
		t.Fatalf("parse defaultConfigJSON: %v", err)
	}
	if def.KillTimeout != 10 {
		t.Errorf("default kill_timeout = %d, want 10", def.KillTimeout)
	}
}

func newKillTestModel(cmd *exec.Cmd, exited <-chan struct{}, killTimeout int) Model {
	return Model{
		panes:        NewPaneManager(80, 24),
		editors:      make(map[int]*EditorWindow),
		debugLog:     &LogBuffer{},
		width:        80,
		height:       24,
		dyalogCmd:    cmd,
		dyalogExited: exited,
		killTimeout:  killTimeout,
	}
}

// drainsToQuit returns true if running the tea.Cmd (or any of its batched
// children) yields a tea.QuitMsg.
func drainsToQuit(c tea.Cmd) bool {
	if c == nil {
		return false
	}
	msg := c()
	if _, ok := msg.(tea.QuitMsg); ok {
		return true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, b := range batch {
			if drainsToQuit(b) {
				return true
			}
		}
	}
	return false
}

func TestQuitNoCmdGoesStraightToQuit(t *testing.T) {
	initColors(0, "")
	closed := make(chan struct{})
	close(closed)
	m := newKillTestModel(nil, closed, 5)
	_, c := m.quit()
	if !drainsToQuit(c) {
		t.Errorf("quit() with nil dyalogCmd should return tea.Quit")
	}
	if m.killWaitActive {
		t.Error("killWaitActive should remain false")
	}
	if m.panes.Get("kill-wait") != nil {
		t.Error("kill-wait pane should not be added")
	}
}

func TestDyalogExitedMsgIgnoredWhenNotWaiting(t *testing.T) {
	initColors(0, "")
	closed := make(chan struct{})
	close(closed)
	m := newKillTestModel(nil, closed, 5)
	// killWaitActive is false; a stray exit notification should be a no-op.
	_, c := m.Update(dyalogExitedMsg{})
	if drainsToQuit(c) {
		t.Error("dyalogExitedMsg outside wait window should not quit")
	}
}

func mustModel(m tea.Model, c tea.Cmd) (Model, tea.Cmd) {
	mm, ok := m.(Model)
	if !ok {
		panic("not a Model")
	}
	return mm, c
}

func TestGracefulKillNilCmd(t *testing.T) {
	closed := make(chan struct{})
	close(closed)
	// Should not panic with nil cmd.
	gracefulKillDyalog(nil, closed, time.Second)
}
