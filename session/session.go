// Package session provides a headless API for Dyalog APL via the RIDE protocol.
//
// Basic usage:
//
//	sess, err := session.Launch(context.Background())
//	if err != nil { ... }
//	defer sess.Close()
//
//	result, err := sess.Eval(ctx, "2+2")
//	// result == "4"
package session

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cursork/gritt/ride"
)

// Session is a connection to a Dyalog APL interpreter.
// Methods are safe for sequential use. For concurrent access,
// the session serialises internally with a mutex.
type Session struct {
	client *ride.Client
	cmd    *exec.Cmd // nil if we didn't launch the process
	mu     sync.Mutex

	// Stored for relaunch on crash (Launch mode only).
	launchOpts *LaunchOptions
}

// LaunchOptions configures how Dyalog is spawned.
type LaunchOptions struct {
	Version string            // "20.0", "/path/to/dyalog", or "" for auto-discover
	Timeout time.Duration     // handshake timeout (default 15s)
	Env     map[string]string // extra environment variables
}

// ConnectOptions configures connection to a running interpreter.
type ConnectOptions struct {
	Addr    string        // "host:port" (default "localhost:4502")
	Timeout time.Duration // connection timeout (default 10s)
}

// Launch spawns a new Dyalog interpreter and connects via RIDE.
// The interpreter is killed when the session is closed.
// Retries up to 3 times if the RIDE handshake fails.
func Launch(ctx context.Context, opts ...LaunchOptions) (*Session, error) {
	var opt LaunchOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Timeout == 0 {
		opt.Timeout = 15 * time.Second
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}

		client, cmd, err := launchOnce(ctx, opt)
		if err == nil {
			return &Session{client: client, cmd: cmd, launchOpts: &opt}, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// launchOnce performs a single attempt to spawn Dyalog and complete the RIDE handshake.
// Uses SERVE mode: Dyalog listens on a random port, we connect to it.
func launchOnce(ctx context.Context, opt LaunchOptions) (*ride.Client, *exec.Cmd, error) {
	exe, err := FindDyalog(opt.Version)
	if err != nil {
		return nil, nil, err
	}

	port := 10000 + rand.Intn(50000)
	cmd := exec.Command(exe, "+s", "-q")
	cmd.Env = append(os.Environ(), fmt.Sprintf("RIDE_INIT=SERVE:*:%d", port))
	cmd.Env = append(cmd.Env, "RIDE_SPAWNED=1")
	cmd.Env = append(cmd.Env, "DYALOG_LINEEDITOR_MODE=1")
	cmd.Env = append(cmd.Env, DyalogEnv(exe)...)
	for k, v := range opt.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start dyalog (%s): %w", exe, err)
	}

	// Poll for RIDE to be ready
	addr := fmt.Sprintf("localhost:%d", port)
	deadline := time.After(opt.Timeout)
	for {
		select {
		case <-ctx.Done():
			kill(cmd)
			return nil, nil, ctx.Err()
		case <-deadline:
			kill(cmd)
			return nil, nil, fmt.Errorf("dyalog did not start on port %d within %s", port, opt.Timeout)
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	client, err := ride.Connect(addr)
	if err != nil {
		kill(cmd)
		return nil, nil, fmt.Errorf("ride connect: %w", err)
	}

	return client, cmd, nil
}

// Connect connects to an already-running Dyalog interpreter in SERVE mode.
func Connect(ctx context.Context, opts ...ConnectOptions) (*Session, error) {
	var opt ConnectOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Addr == "" {
		opt.Addr = "localhost:4502"
	}

	client, err := ride.Connect(opt.Addr)
	if err != nil {
		return nil, err
	}

	return &Session{client: client}, nil
}

// Close shuts down the session. If the session owns the Dyalog process, it is killed.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.client.Close()
	if s.cmd != nil {
		kill(s.cmd)
	}
	return err
}

// Alive reports whether the connection is still open and, if we own the process,
// whether it is still running.
func (s *Session) Alive() bool {
	if s.cmd != nil {
		return s.cmd.ProcessState == nil
	}
	return true
}

// --- Execution ---

// APLError represents an error from the APL interpreter.
type APLError struct {
	Message string   // e.g. "DOMAIN ERROR"
	Lines   []string // all error output lines
}

func (e *APLError) Error() string {
	return e.Message
}

// ErrNotLaunched is returned when Relaunch is called on a Connect-mode session.
var ErrNotLaunched = fmt.Errorf("session was not launched; cannot relaunch")

// ErrSessionRestarted indicates the interpreter crashed and was relaunched.
var ErrSessionRestarted = fmt.Errorf("interpreter crashed and was restarted; workspace state lost")

// Eval executes APL code and returns the output as a single string.
// Input echo (type 14) is filtered. APL errors return *APLError.
func (s *Session) Eval(ctx context.Context, code string) (string, error) {
	lines, err := s.execCollect(ctx, code)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

// EvalAll executes APL code and returns all output lines.
func (s *Session) EvalAll(ctx context.Context, code string) ([]string, error) {
	return s.execCollect(ctx, code)
}

// Batch executes multiple expressions sequentially in the same session.
// Returns one result per expression. Stops on first error.
func (s *Session) Batch(ctx context.Context, exprs []string) ([]string, error) {
	results := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		r, err := s.Eval(ctx, expr)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}

// --- Workspace operations ---

// NS specifies a target namespace for Link.
type NS string

// Link links a filesystem directory into the APL workspace.
// With no namespace argument, links to root (#).
func (s *Session) Link(ctx context.Context, dir string, ns ...NS) error {
	target := "#"
	if len(ns) > 0 {
		target = string(ns[0])
	}
	code := fmt.Sprintf("⎕SE.Link.Create '%s' '%s'", target, dir)
	_, err := s.Eval(ctx, code)
	return err
}

// Fix loads an APL source file into the workspace via ⎕FIX.
func (s *Session) Fix(ctx context.Context, path string) error {
	code := fmt.Sprintf("⎕FIX 'file://%s'", path)
	_, err := s.Eval(ctx, code)
	return err
}

// Names returns names in the given namespace (default "#").
func (s *Session) Names(ctx context.Context, ns ...NS) ([]string, error) {
	target := "#"
	if len(ns) > 0 {
		target = string(ns[0])
	}
	code := fmt.Sprintf("↑' '(≠⊆⊢)∊' ',¨(%s.⎕NL ¯2 ¯3 ¯4 ¯9)", target)
	raw, err := s.Eval(ctx, code)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	return strings.Fields(raw), nil
}

// Get retrieves the display form of a variable.
func (s *Session) Get(ctx context.Context, name string) (string, error) {
	return s.Eval(ctx, name)
}

// Format reformats APL source files in place using Dyalog's formatter.
func (s *Session) Format(ctx context.Context, paths ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Lazily opened dummy windows — one for functions, one for namespaces
	fnToken := -1
	nsToken := -1

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		content := strings.TrimRight(string(data), "\n")
		lines := strings.Split(content, "\n")

		// Detect namespace files by first non-blank line
		isNamespace := false
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				isNamespace = strings.HasPrefix(trimmed, ":Namespace") ||
					strings.HasPrefix(trimmed, ":Class") ||
					strings.HasPrefix(trimmed, ":Interface")
				break
			}
		}

		var token int
		if isNamespace {
			if nsToken < 0 {
				var err error
				nsToken, err = s.openDummyNamespace()
				if err != nil {
					return err
				}
			}
			token = nsToken
		} else {
			if fnToken < 0 {
				var err error
				fnToken, err = s.openDummyEditor()
				if err != nil {
					return err
				}
			}
			token = fnToken
		}

		formatted, err := s.formatCode(token, lines)
		if err != nil {
			return fmt.Errorf("format %s: %w", path, err)
		}

		// Check if anything changed
		changed := len(lines) != len(formatted)
		if !changed {
			for i := range lines {
				if lines[i] != formatted[i] {
					changed = true
					break
				}
			}
		}

		if changed {
			out := strings.Join(formatted, "\n") + "\n"
			if err := os.WriteFile(path, []byte(out), 0644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}
	}

	// Close dummy windows
	if fnToken >= 0 {
		s.client.Send("CloseWindow", map[string]any{"win": fnToken})
	}
	if nsToken >= 0 {
		s.client.Send("CloseWindow", map[string]any{"win": nsToken})
	}

	return nil
}

// Relaunch kills the current interpreter and starts a fresh one.
// Only works for sessions created with Launch (not Connect).
func (s *Session) Relaunch(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.relaunchLocked(ctx)
}

func (s *Session) relaunchLocked(ctx context.Context) error {
	if s.launchOpts == nil {
		return ErrNotLaunched
	}

	s.client.Close()
	if s.cmd != nil {
		kill(s.cmd)
	}

	client, cmd, err := launchOnce(ctx, *s.launchOpts)
	if err != nil {
		for attempt := 2; attempt <= 3; attempt++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
			client, cmd, err = launchOnce(ctx, *s.launchOpts)
			if err == nil {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("relaunch failed: %w", err)
		}
	}

	s.client = client
	s.cmd = cmd
	return nil
}

// --- Internal ---

var fmtCounter int

func (s *Session) openDummyEditor() (int, error) {
	fmtCounter++
	name := fmt.Sprintf("gritt∆fmt%d", fmtCounter)
	if err := s.client.Send("Edit", map[string]any{
		"win":  0,
		"text": name,
		"pos":  0,
	}); err != nil {
		return 0, fmt.Errorf("send Edit: %w", err)
	}
	return s.waitForOpenWindow()
}

func (s *Session) openDummyNamespace() (int, error) {
	fmtCounter++
	name := fmt.Sprintf("gritt∆fmt%d", fmtCounter)
	// Create the namespace first
	if err := s.execPrintLocked(fmt.Sprintf("⎕FIX ':Namespace %s' ':EndNamespace'", name)); err != nil {
		return 0, err
	}
	if err := s.client.Send("Edit", map[string]any{
		"win":  0,
		"text": name,
		"pos":  0,
	}); err != nil {
		return 0, fmt.Errorf("send Edit: %w", err)
	}
	return s.waitForOpenWindow()
}

func (s *Session) waitForOpenWindow() (int, error) {
	for {
		msg, _, err := s.client.Recv()
		if err != nil {
			return 0, fmt.Errorf("recv waiting for OpenWindow: %w", err)
		}
		if msg != nil && msg.Command == "OpenWindow" {
			if t, ok := msg.Args["token"].(float64); ok {
				return int(t), nil
			}
		}
	}
}

func (s *Session) formatCode(win int, lines []string) ([]string, error) {
	text := make([]any, len(lines))
	for i, l := range lines {
		text[i] = l
	}

	if err := s.client.Send("FormatCode", map[string]any{
		"win":  win,
		"text": text,
	}); err != nil {
		return nil, fmt.Errorf("send FormatCode: %w", err)
	}

	for {
		msg, _, err := s.client.Recv()
		if err != nil {
			return nil, fmt.Errorf("recv ReplyFormatCode: %w", err)
		}
		if msg != nil && msg.Command == "ReplyFormatCode" {
			if result, ok := msg.Args["text"].([]any); ok {
				out := make([]string, len(result))
				for i, l := range result {
					out[i], _ = l.(string)
				}
				return out, nil
			}
		}
	}
}

// execCollect sends Execute, collects output, separates errors.
// If the connection dies and the session was launched, it automatically
// relaunches the interpreter and returns ErrSessionRestarted.
func (s *Session) execCollect(ctx context.Context, code string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.client.Send("Execute", map[string]any{
		"text":  code + "\n",
		"trace": 0,
	}); err != nil {
		if rerr := s.tryRelaunchLocked(ctx); rerr == nil {
			return nil, ErrSessionRestarted
		}
		return nil, fmt.Errorf("send execute: %w", err)
	}

	var outputs []string
	var errors []string

	for {
		select {
		case <-ctx.Done():
			s.client.Send("WeakInterrupt", map[string]any{})
			return outputs, ctx.Err()
		default:
		}

		msg, _, err := s.client.Recv()
		if err != nil {
			if rerr := s.tryRelaunchLocked(ctx); rerr == nil {
				return nil, ErrSessionRestarted
			}
			return outputs, fmt.Errorf("recv: %w", err)
		}
		if msg == nil {
			continue
		}

		switch msg.Command {
		case "AppendSessionOutput":
			t, _ := msg.Args["type"].(float64)
			result, _ := msg.Args["result"].(string)
			result = strings.TrimRight(result, "\n")
			switch int(t) {
			case 14, 11:
				// Input echo / multiline body echo — skip
			case 5:
				// APL error output
				errors = append(errors, result)
			default:
				if result != "" {
					outputs = append(outputs, result)
				}
			}
		case "SetPromptType":
			if t, ok := msg.Args["type"].(float64); ok && t > 0 {
				if len(errors) > 0 {
					return nil, makeAPLError(errors)
				}
				return outputs, nil
			}
		}
	}
}

// execPrintLocked executes an expression, discarding output. Caller must hold mu.
func (s *Session) execPrintLocked(expr string) error {
	if err := s.client.Send("Execute", map[string]any{
		"text":  expr + "\n",
		"trace": 0,
	}); err != nil {
		return fmt.Errorf("send execute: %w", err)
	}

	for {
		msg, _, err := s.client.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		if msg == nil {
			continue
		}
		if msg.Command == "SetPromptType" {
			if t, ok := msg.Args["type"].(float64); ok && t > 0 {
				return nil
			}
		}
	}
}

func (s *Session) tryRelaunchLocked(ctx context.Context) error {
	if s.launchOpts == nil {
		return ErrNotLaunched
	}
	return s.relaunchLocked(ctx)
}

func makeAPLError(lines []string) *APLError {
	msg := strings.Join(lines, "\n")
	summary := msg
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		summary = msg[:idx]
	}
	summary = strings.TrimSpace(summary)
	summary = strings.TrimPrefix(summary, "⍎")
	return &APLError{
		Message: summary,
		Lines:   lines,
	}
}

