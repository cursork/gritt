// Package uitest provides TUI testing via tmux
package uitest

import (
	"bytes"
	"fmt"
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
	args := []string{
		"new-session", "-d",
		"-s", name,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
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

// Capture returns the current pane content (with ANSI escape codes)
func (s *Session) Capture() (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", s.Name, "-p", "-e").Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture pane: %w", err)
	}
	return string(out), nil
}

// WaitFor waits until the output contains the pattern or timeout
func (s *Session) WaitFor(pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := s.Capture()
		if err != nil {
			return err
		}
		if strings.Contains(content, pattern) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	content, _ := s.Capture()
	return fmt.Errorf("timeout waiting for %q\nCurrent content:\n%s", pattern, content)
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

// Contains checks if the current output contains the pattern
func (s *Session) Contains(pattern string) (bool, error) {
	content, err := s.Capture()
	if err != nil {
		return false, err
	}
	return strings.Contains(content, pattern), nil
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
func RequireDyalog(port int) error {
	addr := fmt.Sprintf("localhost:%d", port)
	cmd := exec.Command("nc", "-z", addr[:strings.Index(addr, ":")], fmt.Sprintf("%d", port))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Dyalog not running on port %d. Start with: RIDE_INIT=SERVE:*:%d dyalog +s -q", port, port)
	}
	return nil
}

// StartDyalog starts Dyalog in the background
func StartDyalog(port int) (*exec.Cmd, error) {
	cmd := exec.Command("dyalog", "+s", "-q")
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("RIDE_INIT=SERVE:*:%d", port))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Dyalog: %w", err)
	}
	// Wait for it to be ready
	time.Sleep(3 * time.Second)
	return cmd, nil
}
