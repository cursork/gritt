//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// startTermIgnoringProcess starts a process group that survives SIGTERM. The
// shell parent traps and ignores SIGTERM; sleep children get killed but the
// group keeps a process. Returns the cmd plus an "exited" channel that the
// caller-supplied Wait() goroutine closes when the process is reaped.
func startTermIgnoringProcess(t *testing.T) (*exec.Cmd, <-chan struct{}) {
	t.Helper()
	// touch a marker file once the trap is installed so we know the
	// shell has actually executed `trap "" TERM` before we SIGTERM it.
	marker := t.TempDir() + "/ready"
	cmd := exec.Command("sh", "-c", `trap "" TERM; touch "$1"; while :; do sleep 1; done`, "sh", marker)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	exited := make(chan struct{})
	go func() {
		cmd.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-exited
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := exec.Command("test", "-e", marker).CombinedOutput(); err == nil {
			return cmd, exited
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper trap never armed")
	return cmd, exited
}

func TestQuitOpensModalWhenAlive(t *testing.T) {
	initColors(0, "")
	cmd, exited := startTermIgnoringProcess(t)
	m := newKillTestModel(cmd, exited, 5)

	m2, _ := m.quit()
	m = m2.(Model)

	if !m.killWaitActive {
		t.Error("killWaitActive should be true after quit()")
	}
	pane := m.panes.Get("kill-wait")
	if pane == nil {
		t.Fatal("kill-wait pane not added")
	}
	kp, ok := pane.Content.(*KillWaitPane)
	if !ok {
		t.Fatalf("pane content type = %T, want *KillWaitPane", pane.Content)
	}
	if kp.secondsLeft != 5 {
		t.Errorf("secondsLeft = %d, want 5", kp.secondsLeft)
	}
}

func TestKillTickDecrements(t *testing.T) {
	initColors(0, "")
	cmd, exited := startTermIgnoringProcess(t)
	m := newKillTestModel(cmd, exited, 5)
	m, _ = mustModel(m.quit())

	// Simulate a tick — process is still alive, count should drop to 4.
	m, _ = mustModel(m.Update(killTickMsg{}))

	pane := m.panes.Get("kill-wait")
	if pane == nil {
		t.Fatal("modal closed unexpectedly")
	}
	kp := pane.Content.(*KillWaitPane)
	if kp.secondsLeft != 4 {
		t.Errorf("secondsLeft after 1 tick = %d, want 4", kp.secondsLeft)
	}
}

func TestKillTickEscalatesToSIGKILLOnTimeout(t *testing.T) {
	initColors(0, "")
	cmd, exited := startTermIgnoringProcess(t)
	m := newKillTestModel(cmd, exited, 1)
	m, _ = mustModel(m.quit())

	// secondsLeft starts at 1; one tick reaches 0 → reapAndQuit.
	m2, c := m.Update(killTickMsg{})
	m = m2.(Model)

	if !drainsToQuit(c) {
		t.Error("expected tea.Quit after timeout escalation")
	}
	// Wait briefly for SIGKILL to take effect, then verify dead.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(cmd) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if processAlive(cmd) {
		t.Error("process should be killed after timeout escalation")
	}
}

func TestDyalogExitedMsgQuitsImmediately(t *testing.T) {
	initColors(0, "")
	cmd, exited := startTermIgnoringProcess(t)
	m := newKillTestModel(cmd, exited, 10)
	m, _ = mustModel(m.quit())

	// Simulate the wait-goroutine observing the process exit (no tick yet).
	m2, c := m.Update(dyalogExitedMsg{})
	m = m2.(Model)

	if !drainsToQuit(c) {
		t.Error("dyalogExitedMsg should trigger tea.Quit while modal is up")
	}
	if m.killWaitActive {
		t.Error("killWaitActive should be cleared")
	}
}

func TestKillTickReapsWhenProcessExits(t *testing.T) {
	initColors(0, "")
	// `sleep 0.1` exits very quickly — by the time we tick, it's gone.
	cmd := exec.Command("sleep", "0.1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	exited := make(chan struct{})
	go func() {
		cmd.Wait()
		close(exited)
	}()
	m := newKillTestModel(cmd, exited, 5)
	m, _ = mustModel(m.quit())
	<-exited // wait until the goroutine has reaped

	m2, c := m.Update(killTickMsg{})
	m = m2.(Model)

	if !drainsToQuit(c) {
		t.Error("expected tea.Quit when dyalog exits inside the wait window")
	}
	if m.killWaitActive {
		t.Error("killWaitActive should be cleared after reap")
	}
}

func TestEscCancelsKillWait(t *testing.T) {
	initColors(0, "")
	cmd, exited := startTermIgnoringProcess(t)
	m := newKillTestModel(cmd, exited, 5)
	m, _ = mustModel(m.quit())

	// Esc while modal is up.
	m2, c := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = m2.(Model)

	if drainsToQuit(c) {
		t.Error("esc should NOT trigger tea.Quit")
	}
	if m.killWaitActive {
		t.Error("killWaitActive should be false after esc")
	}
	if m.panes.Get("kill-wait") != nil {
		t.Error("kill-wait pane should be removed after esc")
	}
	if !processAlive(cmd) {
		t.Error("process should still be alive after cancel")
	}
}

func TestKKillsImmediately(t *testing.T) {
	initColors(0, "")
	cmd, exited := startTermIgnoringProcess(t)
	m := newKillTestModel(cmd, exited, 5)
	m, _ = mustModel(m.quit())

	m2, c := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = m2.(Model)

	if !drainsToQuit(c) {
		t.Error("'k' should trigger tea.Quit immediately")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(cmd) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if processAlive(cmd) {
		t.Error("process should be killed after 'k'")
	}
}

func TestGracefulKillShortCircuitsWhenAlreadyExited(t *testing.T) {
	cmd := exec.Command("sleep", "0.1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	exited := make(chan struct{})
	go func() {
		cmd.Wait()
		close(exited)
	}()
	<-exited // process already gone before we call

	start := time.Now()
	gracefulKillDyalog(cmd, exited, 5*time.Second)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("should return instantly, took %v", elapsed)
	}
}

func TestGracefulKillReturnsWhenSIGTERMHonored(t *testing.T) {
	// `sleep 60` exits on SIGTERM — graceful path.
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	exited := make(chan struct{})
	go func() {
		cmd.Wait()
		close(exited)
	}()

	start := time.Now()
	gracefulKillDyalog(cmd, exited, 5*time.Second)
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Errorf("SIGTERM-honoring process took %v, expected near-instant", elapsed)
	}
	select {
	case <-exited:
	default:
		t.Error("exited channel not closed after gracefulKillDyalog returned")
	}
}

func TestGracefulKillEscalatesOnTimeout(t *testing.T) {
	cmd, exited := startTermIgnoringProcess(t)

	start := time.Now()
	gracefulKillDyalog(cmd, exited, 500*time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 400*time.Millisecond {
		t.Errorf("escalation returned too fast (%v); SIGKILL path skipped?", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("escalation took too long (%v)", elapsed)
	}
	select {
	case <-exited:
	default:
		t.Error("process not reaped after escalation")
	}
}
