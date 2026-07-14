package uitest

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Runner provides a test runner with snapshots and reporting
type Runner struct {
	T       *testing.T
	Session *Session
	Report  *Report

	// SkipAliveCheck disables the IsAlive gate inside Test(). Set to true
	// for runners driving non-TUI interactions (e.g. a CLI-only session
	// running `./gritt -history`), where the gritt TUI's top border won't
	// be rendered.
	SkipAliveCheck bool
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

// IsAlive returns true if the gritt TUI appears to be rendering normally.
// A dead/broken state (Dyalog disconnected, gritt error screen) shows up as
// the absence of gritt's top border ("╭─ gritt") and presence of fatal
// error markers like "connection refused" or "Press any key to exit".
//
// Many test predicates use `!runner.Contains("X")` and trivially evaluate
// true when nothing is rendering. Gating Test() on IsAlive() turns those
// silent false-positives into honest failures.
//
// Recognized "alive" UI states:
//   - normal pane mode: top border "╭─ gritt" is visible
//   - focus mode: border is hidden by design; "focus mode" hint shows
//     in the status line instead
func (r *Runner) IsAlive() bool {
	content, err := r.Session.Capture()
	if err != nil {
		return false
	}
	plain := stripANSI(content)
	// Fatal markers — gritt's stderr/exit screen
	deadMarkers := []string{
		"connection refused",
		"Press any key to exit",
		"Error: dial",
	}
	for _, m := range deadMarkers {
		if strings.Contains(plain, m) {
			return false
		}
	}
	// Positive markers — at least one must be visible for us to treat
	// gritt as rendering normally. ╭─ gritt is the standard top border;
	// "focus mode" is shown as a hint when the border is intentionally
	// hidden.
	if strings.Contains(plain, "╭─ gritt") {
		return true
	}
	if strings.Contains(plain, "focus mode") {
		return true
	}
	return false
}

// Test runs a test with automatic result recording. Before running the
// predicate it asserts the gritt UI is alive (unless SkipAliveCheck is
// set) — without this check, tests whose predicates merely assert
// ABSENCE of state silently pass on a broken UI (e.g. `!Contains("X")`
// is true when nothing is rendering).
func (r *Runner) Test(name string, fn func() bool) bool {
	if !r.SkipAliveCheck && !r.IsAlive() {
		r.Report.AddResult(name, false)
		r.T.Errorf("FAIL: %s (system not alive — gritt not rendering)", name)
		r.Snapshot(fmt.Sprintf("Failed (system dead): %s", name))
		return false
	}
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

// Skip records a test that wasn't run because a required capability is
// unavailable in this environment (e.g. the Dyalog interpreter in the
// `dyalog/dyalog` container declines to emit OpenWindow{debugger:1}, so
// tracer-dependent tests have nothing to assert against). The test counts
// as neither pass nor fail; the report surfaces it distinctly so a
// green-with-skips run is visibly different from all-green.
//
// Capability detection MUST be runtime (probe the actual behaviour); never
// gate by environment variable. See FACIENDA for the current sanctioned
// skip — Dyalog tracer in the official Docker image.
func (r *Runner) Skip(name, reason string) {
	r.Report.AddSkip(name, reason)
	r.T.Logf("SKIP: %s — %s", name, reason)
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

// WaitForLine snapshots the screen, then waits for a new line containing
// pattern to appear (ignoring lines already on screen, like input echo).
func (r *Runner) WaitForLine(pattern string, timeout time.Duration) bool {
	err := r.Session.WaitForLine(pattern, timeout)
	if err != nil {
		r.T.Logf("WaitForLine failed: %v", err)
		return false
	}
	return true
}

// WaitForNot waits for a pattern to disappear
func (r *Runner) WaitForNot(pattern string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !r.Contains(pattern) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.T.Logf("Timeout waiting for %q to disappear", pattern)
	return false
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

// WaitForIdle waits until the interpreter is no longer busy.
// Detects absence of all spinner braille frames from the screen.
func (r *Runner) WaitForIdle(timeout time.Duration) bool {
	spinnerFrames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		busy := false
		for _, frame := range spinnerFrames {
			if r.Contains(string(frame)) {
				busy = true
				break
			}
		}
		if !busy {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.T.Logf("Timeout waiting for idle")
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
