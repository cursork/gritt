// Package daptest provides a test client for DAP adapters.
// It follows patterns from @vscode/debugadapter-testsupport.
package daptest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// Client is a DAP test client that communicates with a debug adapter.
type Client struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader

	seq int
	mu  sync.Mutex

	events    chan Message
	responses chan Message
	done      chan struct{}
}

// Message represents a DAP message (request, response, or event).
type Message map[string]any

// NewClient starts the debug adapter and returns a test client.
func NewClient(t *testing.T, adapterPath string) *Client {
	t.Helper()

	cmd := exec.Command(adapterPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start adapter: %v", err)
	}

	c := &Client{
		t:         t,
		cmd:       cmd,
		stdin:     stdin,
		reader:    bufio.NewReader(stdout),
		events:    make(chan Message, 100),
		responses: make(chan Message, 100),
		done:      make(chan struct{}),
	}

	go c.readMessages()

	return c
}

// Stop terminates the debug adapter process.
func (c *Client) Stop() {
	close(c.done)
	c.stdin.Close()
	c.cmd.Process.Kill()
	c.cmd.Wait()
}

// readMessages reads DAP messages from the adapter and routes them.
func (c *Client) readMessages() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		var contentLength int
		for {
			line, err := c.reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}

		if contentLength == 0 {
			continue
		}

		body := make([]byte, contentLength)
		_, err := io.ReadFull(c.reader, body)
		if err != nil {
			return
		}

		var msg Message
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "response":
			c.responses <- msg
		case "event":
			c.events <- msg
		}
	}
}

// sendRequest sends a DAP request and returns the sequence number.
func (c *Client) sendRequest(command string, args any) int {
	c.mu.Lock()
	c.seq++
	seq := c.seq
	c.mu.Unlock()

	req := map[string]any{
		"seq":       seq,
		"type":      "request",
		"command":   command,
		"arguments": args,
	}

	body, _ := json.Marshal(req)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	c.stdin.Write([]byte(header))
	c.stdin.Write(body)

	return seq
}

// waitResponse waits for a response to a specific command.
func (c *Client) waitResponse(command string, timeout time.Duration) Message {
	c.t.Helper()

	select {
	case msg := <-c.responses:
		return msg
	case <-time.After(timeout):
		c.t.Fatalf("Timeout waiting for %s response", command)
		return nil
	}
}

// WaitForEvent waits for a specific event type.
func (c *Client) WaitForEvent(eventName string, timeout time.Duration) Message {
	c.t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case msg := <-c.events:
			if event, _ := msg["event"].(string); event == eventName {
				return msg
			}
			// Put back or discard other events - for now, discard
		case <-deadline:
			c.t.Fatalf("Timeout waiting for %s event", eventName)
			return nil
		}
	}
}

// DrainEvents drains events for a duration, returning all collected events.
func (c *Client) DrainEvents(duration time.Duration) []Message {
	var events []Message
	deadline := time.After(duration)
	for {
		select {
		case msg := <-c.events:
			events = append(events, msg)
		case <-deadline:
			return events
		}
	}
}

// Initialize sends the initialize request.
func (c *Client) Initialize() Message {
	c.t.Helper()

	c.sendRequest("initialize", map[string]any{
		"clientID":        "daptest",
		"adapterID":       "apl",
		"linesStartAt1":   true,
		"columnsStartAt1": true,
	})

	resp := c.waitResponse("initialize", 5*time.Second)
	c.assertSuccess(resp, "initialize")

	// Wait for initialized event
	c.WaitForEvent("initialized", 5*time.Second)

	return resp
}

// Attach sends the attach request.
func (c *Client) Attach(host string, port int) Message {
	c.t.Helper()

	c.sendRequest("attach", map[string]any{
		"host": host,
		"port": port,
	})

	resp := c.waitResponse("attach", 5*time.Second)
	c.assertSuccess(resp, "attach")
	return resp
}

// ConfigurationDone sends the configurationDone request.
func (c *Client) ConfigurationDone() Message {
	c.t.Helper()

	c.sendRequest("configurationDone", map[string]any{})
	resp := c.waitResponse("configurationDone", 5*time.Second)
	c.assertSuccess(resp, "configurationDone")
	return resp
}

// InitializeAndAttach performs the standard initialization sequence.
func (c *Client) InitializeAndAttach(host string, port int) {
	c.t.Helper()
	c.Initialize()
	c.Attach(host, port)
	c.ConfigurationDone()
}

// Evaluate sends an evaluate request and returns the result.
func (c *Client) Evaluate(expression string) EvaluateResult {
	c.t.Helper()

	c.sendRequest("evaluate", map[string]any{
		"expression": expression,
		"context":    "repl",
	})

	resp := c.waitResponse("evaluate", 10*time.Second)
	c.assertSuccess(resp, "evaluate")

	result := EvaluateResult{}
	if body, ok := resp["body"].(map[string]any); ok {
		result.Result, _ = body["result"].(string)
		result.Type, _ = body["type"].(string)
	}
	return result
}

// EvaluateResult holds the result of an evaluate request.
type EvaluateResult struct {
	Result string
	Type   string
}

// SetBreakpoints sets breakpoints in a source file.
func (c *Client) SetBreakpoints(path string, lines ...int) []Breakpoint {
	c.t.Helper()

	bps := make([]map[string]any, len(lines))
	for i, line := range lines {
		bps[i] = map[string]any{"line": line}
	}

	c.sendRequest("setBreakpoints", map[string]any{
		"source":      map[string]any{"path": path},
		"breakpoints": bps,
	})

	resp := c.waitResponse("setBreakpoints", 5*time.Second)
	c.assertSuccess(resp, "setBreakpoints")

	var result []Breakpoint
	if body, ok := resp["body"].(map[string]any); ok {
		if bpList, ok := body["breakpoints"].([]any); ok {
			for _, bp := range bpList {
				if bpMap, ok := bp.(map[string]any); ok {
					result = append(result, Breakpoint{
						ID:       int(getFloat(bpMap, "id")),
						Verified: getBool(bpMap, "verified"),
						Line:     int(getFloat(bpMap, "line")),
					})
				}
			}
		}
	}
	return result
}

// Breakpoint describes a verified breakpoint.
type Breakpoint struct {
	ID       int
	Verified bool
	Line     int
	Message  string
}

// Continue resumes execution.
func (c *Client) Continue(threadID int) Message {
	c.t.Helper()

	c.sendRequest("continue", map[string]any{
		"threadId": threadID,
	})

	resp := c.waitResponse("continue", 5*time.Second)
	c.assertSuccess(resp, "continue")
	return resp
}

// Next performs a step over.
func (c *Client) Next(threadID int) Message {
	c.t.Helper()

	c.sendRequest("next", map[string]any{
		"threadId": threadID,
	})

	resp := c.waitResponse("next", 5*time.Second)
	c.assertSuccess(resp, "next")
	return resp
}

// StepIn performs a step into.
func (c *Client) StepIn(threadID int) Message {
	c.t.Helper()

	c.sendRequest("stepIn", map[string]any{
		"threadId": threadID,
	})

	resp := c.waitResponse("stepIn", 5*time.Second)
	c.assertSuccess(resp, "stepIn")
	return resp
}

// StepOut performs a step out.
func (c *Client) StepOut(threadID int) Message {
	c.t.Helper()

	c.sendRequest("stepOut", map[string]any{
		"threadId": threadID,
	})

	resp := c.waitResponse("stepOut", 5*time.Second)
	c.assertSuccess(resp, "stepOut")
	return resp
}

// StackTrace returns the current stack trace.
func (c *Client) StackTrace(threadID int) []StackFrame {
	c.t.Helper()

	c.sendRequest("stackTrace", map[string]any{
		"threadId": threadID,
	})

	resp := c.waitResponse("stackTrace", 5*time.Second)
	c.assertSuccess(resp, "stackTrace")

	var frames []StackFrame
	if body, ok := resp["body"].(map[string]any); ok {
		if sf, ok := body["stackFrames"].([]any); ok {
			for _, f := range sf {
				if fm, ok := f.(map[string]any); ok {
					frame := StackFrame{
						ID:   int(getFloat(fm, "id")),
						Name: getString(fm, "name"),
						Line: int(getFloat(fm, "line")),
					}
					if src, ok := fm["source"].(map[string]any); ok {
						frame.SourcePath = getString(src, "path")
						frame.SourceName = getString(src, "name")
					}
					frames = append(frames, frame)
				}
			}
		}
	}
	return frames
}

// StackFrame describes a stack frame.
type StackFrame struct {
	ID         int
	Name       string
	Line       int
	SourcePath string
	SourceName string
}

// Threads returns the list of threads.
func (c *Client) Threads() []Thread {
	c.t.Helper()

	c.sendRequest("threads", map[string]any{})

	resp := c.waitResponse("threads", 5*time.Second)
	c.assertSuccess(resp, "threads")

	var threads []Thread
	if body, ok := resp["body"].(map[string]any); ok {
		if tl, ok := body["threads"].([]any); ok {
			for _, t := range tl {
				if tm, ok := t.(map[string]any); ok {
					threads = append(threads, Thread{
						ID:   int(getFloat(tm, "id")),
						Name: getString(tm, "name"),
					})
				}
			}
		}
	}
	return threads
}

// Thread describes a thread.
type Thread struct {
	ID   int
	Name string
}

// Disconnect sends the disconnect request.
func (c *Client) Disconnect() {
	c.t.Helper()

	c.sendRequest("disconnect", map[string]any{})
	c.waitResponse("disconnect", 5*time.Second)
}

// ExpectStopped waits for a stopped event and returns its details.
func (c *Client) ExpectStopped(timeout time.Duration) StoppedEvent {
	c.t.Helper()

	msg := c.WaitForEvent("stopped", timeout)
	event := StoppedEvent{}
	if body, ok := msg["body"].(map[string]any); ok {
		event.Reason = getString(body, "reason")
		event.ThreadID = int(getFloat(body, "threadId"))
		event.AllThreadsStopped = getBool(body, "allThreadsStopped")
	}
	return event
}

// StoppedEvent holds details of a stopped event.
type StoppedEvent struct {
	Reason            string
	ThreadID          int
	AllThreadsStopped bool
}

// HitBreakpoint is a helper that sets a breakpoint, runs code, and waits for stop.
// It returns the stopped event and stack trace.
func (c *Client) HitBreakpoint(setupCode, triggerCode, sourcePath string, line int) (StoppedEvent, []StackFrame) {
	c.t.Helper()

	// Setup: define function, set breakpoint
	if setupCode != "" {
		c.Evaluate(setupCode)
		time.Sleep(100 * time.Millisecond)
	}

	c.SetBreakpoints(sourcePath, line)

	// Trigger breakpoint (async - won't return until after we continue)
	go func() {
		// Give it a moment to send
		time.Sleep(50 * time.Millisecond)
		c.sendRequest("evaluate", map[string]any{
			"expression": triggerCode,
			"context":    "repl",
		})
	}()

	// Wait for stopped event
	stopped := c.ExpectStopped(5 * time.Second)
	frames := c.StackTrace(stopped.ThreadID)

	return stopped, frames
}

// AssertStoppedLocation verifies the stop reason and location.
func (c *Client) AssertStoppedLocation(stopped StoppedEvent, frames []StackFrame, expectedReason string, expectedLine int) {
	c.t.Helper()

	if stopped.Reason != expectedReason {
		c.t.Errorf("Expected stop reason %q, got %q", expectedReason, stopped.Reason)
	}

	if len(frames) == 0 {
		c.t.Error("Expected at least one stack frame")
		return
	}

	if expectedLine > 0 && frames[0].Line != expectedLine {
		c.t.Errorf("Expected line %d, got %d", expectedLine, frames[0].Line)
	}
}

// assertSuccess checks that a response was successful.
func (c *Client) assertSuccess(resp Message, command string) {
	c.t.Helper()

	success, _ := resp["success"].(bool)
	if !success {
		errMsg, _ := resp["message"].(string)
		c.t.Fatalf("%s failed: %s", command, errMsg)
	}
}

// Helper functions for extracting values from maps

func getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
