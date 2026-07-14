// Package uitest provides TUI testing via tmux
package uitest

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Session wraps a tmux session for TUI testing
type Session struct {
	Name   string
	Width  int
	Height int
}

// NewSession creates a new tmux session running the given command
func NewSession(name string, width, height int, cmd string) (*Session, error) {
	s := &Session{Name: name, Width: width, Height: height}

	// Kill any existing session with this name
	exec.Command("tmux", "kill-session", "-t", name).Run()

	// Create new session (tmux may ignore -x/-y and inherit outer terminal size)
	// Set COLORTERM=truecolor so gritt outputs ANSI colors
	// Set HOME=/tmp to avoid loading user's personal config
	args := []string{
		"new-session", "-d",
		"-s", name,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
		"-e", "COLORTERM=truecolor",
		"-e", "HOME=/tmp",
		cmd,
	}
	if err := exec.Command("tmux", args...).Run(); err != nil {
		return nil, fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Force resize to ensure correct dimensions (sends SIGWINCH to app)
	exec.Command("tmux", "resize-window", "-t", name, "-x", fmt.Sprintf("%d", width), "-y", fmt.Sprintf("%d", height)).Run()

	return s, nil
}

// Close kills the tmux session
func (s *Session) Close() error {
	return exec.Command("tmux", "kill-session", "-t", s.Name).Run()
}

// SendKeys sends keys to the tmux session
func (s *Session) SendKeys(keys ...string) error {
	args := append([]string{"send-keys", "-t", s.Name}, keys...)
	return exec.Command("tmux", args...).Run()
}

// SendLine sends text followed by Enter
func (s *Session) SendLine(text string) error {
	if err := s.SendKeys(text); err != nil {
		return err
	}
	return s.SendKeys("Enter")
}

// SendText sends literal text (for typing in editors)
func (s *Session) SendText(text string) error {
	// Use -l flag for literal text (no key name interpretation)
	return exec.Command("tmux", "send-keys", "-t", s.Name, "-l", text).Run()
}

// Capture returns the current pane content (with ANSI escape codes for reports)
func (s *Session) Capture() (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", s.Name, "-p", "-e").Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture pane: %w", err)
	}
	return string(out), nil
}

// WaitFor waits until the output contains the pattern or timeout.
// Matches against plain text (ANSI codes stripped).
func (s *Session) WaitFor(pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := s.Capture()
		if err != nil {
			return err
		}
		if strings.Contains(stripANSI(content), pattern) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	content, _ := s.Capture()
	return fmt.Errorf("timeout waiting for %q\nCurrent content:\n%s", pattern, stripANSI(content))
}

// WaitForRegex waits until the output matches the regex or timeout
func (s *Session) WaitForRegex(pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := s.Capture()
		if err != nil {
			return err
		}
		matched, _ := regexpMatch(pattern, content)
		if matched {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	content, _ := s.Capture()
	return fmt.Errorf("timeout waiting for pattern %q\nCurrent content:\n%s", pattern, content)
}

func regexpMatch(pattern, content string) (bool, error) {
	// Simple contains check - can be extended to full regex
	return strings.Contains(content, pattern), nil
}

// Contains checks if the current output contains the pattern.
// Matches against plain text (ANSI codes stripped).
func (s *Session) Contains(pattern string) (bool, error) {
	content, err := s.Capture()
	if err != nil {
		return false, err
	}
	return strings.Contains(stripANSI(content), pattern), nil
}

// lines returns the non-empty, trimmed, ANSI-stripped lines on screen.
func (s *Session) lines() ([]string, error) {
	content, err := s.Capture()
	if err != nil {
		return nil, err
	}
	raw := strings.Split(stripANSI(content), "\n")
	var out []string
	for _, l := range raw {
		l = strings.TrimRight(l, " \t")
		if l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

// WaitForLine snapshots the current screen, then waits for a NEW line
// (one not present in the snapshot) that contains pattern. This avoids
// false matches from input echo or stale output already on screen.
func (s *Session) WaitForLine(pattern string, timeout time.Duration) error {
	// Snapshot current lines as a set
	before, err := s.lines()
	if err != nil {
		return err
	}
	seen := make(map[string]int, len(before))
	for _, l := range before {
		seen[l]++
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		now, err := s.lines()
		if err != nil {
			return err
		}
		// Build counts for current screen
		cur := make(map[string]int, len(now))
		for _, l := range now {
			cur[l]++
		}
		// Check for new lines (or additional occurrences) containing pattern
		for l, count := range cur {
			if count > seen[l] && strings.Contains(l, pattern) {
				return nil
			}
		}
	}
	now, _ := s.lines()
	return fmt.Errorf("timeout waiting for new line containing %q\nBefore (%d lines): %v\nNow (%d lines): %v",
		pattern, len(before), before, len(now), now)
}

// ContainsRegex checks if the current output matches the regex
func (s *Session) ContainsRegex(pattern string) (bool, error) {
	content, err := s.Capture()
	if err != nil {
		return false, err
	}
	return regexpMatch(pattern, content)
}

// Sleep pauses for the given duration
func (s *Session) Sleep(d time.Duration) {
	time.Sleep(d)
}

// RequireDyalog checks if Dyalog is running on the given port
// RequireDyalog verifies a working RIDE server is listening on `port`.
// A bare `nc -z` (port-LISTEN check) is insufficient — a half-dead Dyalog
// or a stale TIME_WAIT/orphan process can hold the port without speaking
// the protocol, leaving us connecting to a corpse. We instead open a TCP
// connection and read the RIDE handshake greeting (`SupportedProtocols=`)
// that a live Dyalog sends within ~1s of accepting a connection.
func RequireDyalog(port int) error {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return fmt.Errorf("Dyalog not running on port %d (dial: %v). Start with: RIDE_INIT=SERVE:*:%d dyalog +s -q", port, err, port)
	}
	defer conn.Close()
	// RIDE protocol: server sends a frame within ~1s. The first frame is
	// `<4-byte length> "RIDE" <payload>` where payload starts with the
	// ASCII string "SupportedProtocols=".
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("port %d is open but no RIDE greeting received (read: %v) — likely a dead/orphan process holding the port", port, err)
	}
	if !bytes.Contains(buf[:n], []byte("SupportedProtocols=")) {
		return fmt.Errorf("port %d responded but greeting doesn't look like RIDE (got %q)", port, buf[:n])
	}
	return nil
}

// StartDyalog starts Dyalog in the background and waits until the RIDE
// port is actually accepting connections. A blind sleep races with slow
// startup (cold-start, locked workspace files, etc.) and silently leaves
// the test connecting to a port nothing's listening on yet, which
// presents to the runner as a fatal "connection refused" screen.
func StartDyalog(port int) (*exec.Cmd, error) {
	cmd := exec.Command("dyalog", "+s", "-q")
	cmd.Env = append(cmd.Environ(),
		fmt.Sprintf("RIDE_INIT=SERVE:*:%d", port),
		// RIDE_SPAWNED tells Dyalog "I'm being run with a RIDE GUI
		// client attached" — without it, breakpoints and errors get
		// inline "Name[Line]" session output instead of OpenWindow
		// {debugger:1}, and the tracer pane never opens. mapl sets
		// this on the macOS app; bare `dyalog +s -q` in a container
		// doesn't, so we set it explicitly.
		"RIDE_SPAWNED=1",
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Dyalog: %w", err)
	}
	// Poll until the port accepts a connection (or give up after 30s).
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if RequireDyalog(port) == nil {
			return cmd, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	// Port never came up — kill the orphan and fail.
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
	return nil, fmt.Errorf("Dyalog started but port %d never accepted connections within 30s", port)
}
