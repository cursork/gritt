package adapter_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cursork/gritt/dap/daptest"
)

var adapterPath string

const (
	testHost = "127.0.0.1"
	testPort = 9502 // multapl primary port
)

// TestMain builds the apldap binary for the tests to drive.
func TestMain(m *testing.M) {
	adapterPath = filepath.Join(os.TempDir(), "apldap-test")
	cmd := exec.Command("go", "build", "-o", adapterPath, "github.com/cursork/gritt/grittles/apldap")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("Failed to build adapter: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

// skipIfNoEnvironment skips the test if Dyalog/multapl isn't running.
func skipIfNoEnvironment(t *testing.T) {
	t.Helper()

	// Try a quick connection check
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.Initialize()

	// Try to attach - if it fails, environment isn't running
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Skip("Test environment not running (start with ./scripts/start-test-env.sh)")
			}
		}()
		client.Attach(testHost, testPort)
	}()

	client.Disconnect()
}

func TestInitializeAndAttach(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.Initialize()
	client.Attach(testHost, testPort)
	client.ConfigurationDone()
	client.Disconnect()
}

func TestEvaluateSimple(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.InitializeAndAttach(testHost, testPort)
	defer client.Disconnect()

	// Test simple arithmetic
	result := client.Evaluate("2+2")
	if result.Result != "4" {
		t.Errorf("Expected '4', got %q", result.Result)
	}
}

func TestEvaluateAPLExpressions(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.InitializeAndAttach(testHost, testPort)
	defer client.Disconnect()

	tests := []struct {
		name       string
		expression string
		want       string
	}{
		{"addition", "2+2", "4"},
		{"iota", "⍳5", "1 2 3 4 5"},
		{"reduce", "+/⍳10", "55"},
		{"shape", "⍴3 4⍴⍳12", "3 4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.Evaluate(tt.expression)
			if result.Result != tt.want {
				t.Errorf("Evaluate(%q) = %q, want %q", tt.expression, result.Result, tt.want)
			}
		})
	}
}

func TestDefineAndCallFunction(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.InitializeAndAttach(testHost, testPort)
	defer client.Disconnect()

	// Define a dfn
	client.Evaluate("double←{⍵×2}")
	time.Sleep(100 * time.Millisecond)

	// Call it
	result := client.Evaluate("double 21")
	if result.Result != "42" {
		t.Errorf("Expected '42', got %q", result.Result)
	}
}

func TestThreads(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.InitializeAndAttach(testHost, testPort)
	defer client.Disconnect()

	threads := client.Threads()
	if len(threads) == 0 {
		t.Error("Expected at least one thread")
	}
}

func TestBreakpointViaSTOP(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.InitializeAndAttach(testHost, testPort)
	defer client.Disconnect()

	// Define a function
	client.Evaluate("⎕FX 'r←addone n' 'r←n+1'")
	time.Sleep(100 * time.Millisecond)

	// Set a stop
	client.Evaluate("1 ⎕STOP 'addone'")
	time.Sleep(100 * time.Millisecond)

	// Trigger the function (async - will stop at breakpoint)
	go func() {
		time.Sleep(50 * time.Millisecond)
		client.Evaluate("addone 5")
	}()

	// Wait for stopped event
	stopped := client.ExpectStopped(5 * time.Second)
	if stopped.Reason != "breakpoint" {
		t.Errorf("Expected stop reason 'breakpoint', got %q", stopped.Reason)
	}

	// Get stack trace
	frames := client.StackTrace(stopped.ThreadID)
	if len(frames) == 0 {
		t.Error("Expected at least one stack frame")
	} else if frames[0].Name != "addone" {
		t.Errorf("Expected top frame 'addone', got %q", frames[0].Name)
	}

	// Continue
	client.Continue(stopped.ThreadID)

	// Clear the stop
	time.Sleep(100 * time.Millisecond)
	client.Evaluate("⍬ ⎕STOP 'addone'")
}

func TestStepOver(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.InitializeAndAttach(testHost, testPort)
	defer client.Disconnect()

	// Define a multi-line function
	client.Evaluate("⎕FX 'r←multiline n' 'a←n+1' 'b←a×2' 'r←b-1'")
	time.Sleep(100 * time.Millisecond)

	// Set stop on first line
	client.Evaluate("1 ⎕STOP 'multiline'")
	time.Sleep(100 * time.Millisecond)

	// Trigger
	go func() {
		time.Sleep(50 * time.Millisecond)
		client.Evaluate("multiline 10")
	}()

	// Wait for stop
	stopped := client.ExpectStopped(5 * time.Second)
	frames := client.StackTrace(stopped.ThreadID)
	if len(frames) == 0 {
		t.Fatal("No stack frames")
	}
	initialLine := frames[0].Line

	// Step over
	client.Next(stopped.ThreadID)

	// Should get another stopped event
	stopped = client.ExpectStopped(5 * time.Second)
	frames = client.StackTrace(stopped.ThreadID)

	// Line should have advanced
	if len(frames) > 0 && frames[0].Line <= initialLine {
		t.Errorf("Expected line to advance from %d, got %d", initialLine, frames[0].Line)
	}

	// Continue to finish
	client.Continue(stopped.ThreadID)
	time.Sleep(100 * time.Millisecond)

	// Clear stop
	client.Evaluate("⍬ ⎕STOP 'multiline'")
}

func TestHitBreakpointHelper(t *testing.T) {
	client := daptest.NewClient(t, adapterPath)
	defer client.Stop()

	client.InitializeAndAttach(testHost, testPort)
	defer client.Disconnect()

	// Use the HitBreakpoint helper
	setupCode := "⎕FX 'r←helper n' 'r←n+1'"
	triggerCode := "helper 5"

	// Set breakpoint via ⎕STOP (since VSCode breakpoints don't work yet)
	client.Evaluate(setupCode)
	time.Sleep(100 * time.Millisecond)
	client.Evaluate("1 ⎕STOP 'helper'")
	time.Sleep(100 * time.Millisecond)

	// Trigger async
	go func() {
		time.Sleep(50 * time.Millisecond)
		client.Evaluate(triggerCode)
	}()

	// Wait for stop and verify
	stopped := client.ExpectStopped(5 * time.Second)
	frames := client.StackTrace(stopped.ThreadID)

	client.AssertStoppedLocation(stopped, frames, "breakpoint", 1)

	// Cleanup
	client.Continue(stopped.ThreadID)
	time.Sleep(100 * time.Millisecond)
	client.Evaluate("⍬ ⎕STOP 'helper'")
}
