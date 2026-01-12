package uitest

import (
	"fmt"
	"testing"
	"time"
)

// Runner provides a test runner with snapshots and reporting
type Runner struct {
	T       *testing.T
	Session *Session
	Report  *Report
}

// NewRunner creates a new test runner
func NewRunner(t *testing.T, sessionName string, width, height int, cmd string, reportDir string) (*Runner, error) {
	session, err := NewSession(sessionName, width, height, cmd)
	if err != nil {
		return nil, err
	}

	return &Runner{
		T:       t,
		Session: session,
		Report:  NewReport(reportDir),
	}, nil
}

// Close cleans up the runner
func (r *Runner) Close() error {
	return r.Session.Close()
}

// Snapshot captures the current screen with a label
func (r *Runner) Snapshot(label string) {
	content, err := r.Session.Capture()
	if err != nil {
		r.T.Logf("Warning: failed to capture snapshot %q: %v", label, err)
		return
	}
	r.Report.AddSnapshot(label, content)
}

// Test runs a test with automatic result recording
func (r *Runner) Test(name string, fn func() bool) bool {
	passed := fn()
	r.Report.AddResult(name, passed)
	if passed {
		r.T.Logf("PASS: %s", name)
	} else {
		r.T.Errorf("FAIL: %s", name)
		// Capture failure state
		r.Snapshot(fmt.Sprintf("Failed: %s", name))
	}
	return passed
}

// SendKeys sends keys to the session
func (r *Runner) SendKeys(keys ...string) {
	if err := r.Session.SendKeys(keys...); err != nil {
		r.T.Fatalf("Failed to send keys: %v", err)
	}
}

// SendLine sends a line of text
func (r *Runner) SendLine(text string) {
	if err := r.Session.SendLine(text); err != nil {
		r.T.Fatalf("Failed to send line: %v", err)
	}
}

// SendText sends literal text (for typing in editors)
func (r *Runner) SendText(text string) {
	if err := r.Session.SendText(text); err != nil {
		r.T.Fatalf("Failed to send text: %v", err)
	}
}

// WaitFor waits for a pattern to appear
func (r *Runner) WaitFor(pattern string, timeout time.Duration) bool {
	err := r.Session.WaitFor(pattern, timeout)
	if err != nil {
		r.T.Logf("WaitFor failed: %v", err)
		return false
	}
	return true
}

// Contains checks if the screen contains a pattern
func (r *Runner) Contains(pattern string) bool {
	found, err := r.Session.Contains(pattern)
	if err != nil {
		r.T.Logf("Contains check failed: %v", err)
		return false
	}
	return found
}

// WaitForNoFocusedPane waits until no pane has double-border focus indicator
func (r *Runner) WaitForNoFocusedPane(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !r.Contains("╔") {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.T.Logf("Timeout waiting for focused pane to close")
	return false
}

// Sleep pauses execution
func (r *Runner) Sleep(d time.Duration) {
	time.Sleep(d)
}

// GenerateReport writes the HTML report
func (r *Runner) GenerateReport() string {
	filename, err := r.Report.Generate()
	if err != nil {
		r.T.Errorf("Failed to generate report: %v", err)
		return ""
	}
	r.T.Logf("Report saved to: %s", filename)
	return filename
}
