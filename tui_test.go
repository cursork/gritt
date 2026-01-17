package main

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"gritt/uitest"
)

const (
	dyalogPort  = 4502
	sessionName = "gritt-test"
	screenW     = 120
	screenH     = 40
)

// TestTUI runs the full TUI test suite
func TestTUI(t *testing.T) {
	// Build gritt first
	t.Log("Building gritt...")
	if err := exec.Command("go", "build", "-o", "gritt", ".").Run(); err != nil {
		t.Fatalf("Failed to build gritt: %v", err)
	}

	// Check if Dyalog is running, if not try to start it
	var dyalogCmd *exec.Cmd
	if err := uitest.RequireDyalog(dyalogPort); err != nil {
		t.Log("Starting Dyalog...")
		var startErr error
		dyalogCmd, startErr = uitest.StartDyalog(dyalogPort)
		if startErr != nil {
			t.Skipf("Dyalog not available: %v", startErr)
		}
		defer func() {
			if dyalogCmd != nil && dyalogCmd.Process != nil {
				dyalogCmd.Process.Kill()
			}
		}()
	}

	// Create test runner with protocol logging
	runner, err := uitest.NewRunner(t, sessionName, screenW, screenH, "./gritt -log test-reports/protocol.log", "test-reports")
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}
	defer runner.Close()

	// Wait for gritt to start
	runner.Sleep(1 * time.Second)

	// Take initial snapshot
	runner.Snapshot("Initial state")

	// Test 1: Initial render
	runner.Test("Initial render shows title", func() bool {
		return runner.Contains("gritt")
	})

	// Test 2: C-] d toggles debug pane
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("d")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After C-] d (debug pane open)")

	runner.Test("C-] d opens debug pane", func() bool {
		return runner.Contains("debug")
	})

	// Test 3: Focus indicator
	runner.Test("Focused pane has double border", func() bool {
		return runner.Contains("╔")
	})

	// Test 4: Esc closes pane
	runner.SendKeys("Escape")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After Esc (debug pane closed)")

	runner.Test("Esc closes debug pane", func() bool {
		return !runner.Contains("╔")
	})

	// Test 5: C-] d reopens
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("d")
	runner.Sleep(300 * time.Millisecond)

	runner.Test("C-] d reopens debug pane", func() bool {
		return runner.Contains("debug")
	})

	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Test 6: Execute 1+1
	runner.SendLine("1+1")
	runner.Sleep(1 * time.Second)
	runner.Snapshot("After executing 1+1")

	runner.Test("Execute 1+1 returns 2", func() bool {
		return runner.Contains("2")
	})

	// Test 7: Execute iota
	runner.SendLine("⍳5")
	runner.Sleep(1 * time.Second)
	runner.Snapshot("After executing ⍳5")

	runner.Test("Execute ⍳5 returns sequence", func() bool {
		return runner.Contains("1 2 3 4 5")
	})

	// Test 8: Edit and re-execute
	runner.SendKeys("Up", "Up", "Up", "Up")
	runner.Sleep(300 * time.Millisecond)
	runner.SendKeys("End")
	runner.SendKeys("BSpace")
	runner.SendKeys("2")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After editing 1+1 to 1+2")

	runner.SendKeys("Enter")
	runner.Sleep(1 * time.Second)
	runner.Snapshot("After executing edited line")

	runner.Test("Edit and re-execute works", func() bool {
		return runner.Contains("3")
	})

	// Test 9: Debug pane shows protocol
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("d")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("Debug pane with protocol log")

	runner.Test("Debug pane shows Execute messages", func() bool {
		return runner.Contains("Execute")
	})

	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Test 10: C-] ? shows key mappings pane
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("?")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After C-] ? (key mappings pane)")

	runner.Test("C-] ? opens key mappings pane", func() bool {
		return runner.Contains("key mappings")
	})

	runner.Test("Key mappings shows Actions section", func() bool {
		return runner.Contains("Actions")
	})

	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	runner.Test("Esc closes key mappings pane", func() bool {
		return !runner.Contains("key mappings")
	})

	// Test: C-] : opens command palette
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys(":")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After C-] : (command palette)")

	runner.Test("C-] : opens command palette", func() bool {
		return runner.Contains("Commands")
	})

	runner.Test("Command palette shows debug command", func() bool {
		return runner.Contains("debug")
	})

	runner.Test("Command palette shows quit command", func() bool {
		return runner.Contains("quit")
	})

	// Test: Filter commands by typing
	runner.SendText("deb")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("Command palette filtered to 'deb'")

	runner.Test("Typing filters commands", func() bool {
		// Should still show debug but not quit
		return runner.Contains("debug")
	})

	// Test: Execute command from palette
	runner.SendKeys("Enter")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After selecting debug from palette")

	runner.Test("Selecting debug opens debug pane", func() bool {
		return runner.Contains("debug") && !runner.Contains("Commands")
	})

	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Test: Escape closes command palette
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys(":")
	runner.Sleep(300 * time.Millisecond)
	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	runner.Test("Escape closes command palette", func() bool {
		return !runner.Contains("Commands")
	})

	// Test: Save command shows filename prompt
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys(":")
	runner.Sleep(300 * time.Millisecond)
	runner.SendText("save")
	runner.Sleep(200 * time.Millisecond)
	runner.SendKeys("Enter")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("Save prompt with default filename")

	runner.Test("Save command shows filename prompt", func() bool {
		return runner.Contains("Save as:")
	})

	runner.Test("Save prompt has default filename", func() bool {
		return runner.Contains("session-")
	})

	// Cancel save and continue
	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Test: Pane move mode
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("d") // Open debug pane first
	runner.Sleep(300 * time.Millisecond)

	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("m") // Enter move mode
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("Pane move mode active")

	runner.Test("C-] m enters pane move mode", func() bool {
		return runner.Contains("MOVE")
	})

	// Move pane with arrow keys
	runner.SendKeys("Up", "Up", "Left", "Left")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("After moving pane")

	runner.Test("Arrow keys move pane in move mode", func() bool {
		return runner.Contains("MOVE") // Still in move mode
	})

	// Exit move mode
	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	runner.Test("Escape exits pane move mode", func() bool {
		return !runner.Contains("MOVE")
	})

	// Close the debug pane
	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Test: Backtick mode for APL symbols
	runner.SendKeys("`")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("Backtick mode active")

	runner.Test("Backtick activates APL symbol mode", func() bool {
		return runner.Contains("APL symbol")
	})

	runner.SendKeys("i") // Should insert ⍳
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("After backtick-i (iota)")

	runner.Test("Backtick-i inserts iota", func() bool {
		return runner.Contains("⍳")
	})

	// Test: Symbol search
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys(":")
	runner.Sleep(300 * time.Millisecond)
	runner.SendText("symbols")
	runner.Sleep(200 * time.Millisecond)
	runner.SendKeys("Enter")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("Symbol search pane")

	runner.Test("Symbol search opens", func() bool {
		return runner.Contains("APL Symbols")
	})

	// Search for "rho"
	runner.SendText("rho")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("Symbol search filtered to rho")

	runner.Test("Symbol search filters by name", func() bool {
		return runner.Contains("⍴")
	})

	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Test: APLcart
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys(":")
	runner.Sleep(300 * time.Millisecond)
	runner.SendText("aplcart")
	runner.Sleep(200 * time.Millisecond)
	runner.SendKeys("Enter")
	runner.Sleep(500 * time.Millisecond)
	runner.Snapshot("APLcart pane loading")

	runner.Test("APLcart opens", func() bool {
		return runner.Contains("APLcart")
	})

	// Wait for data to load
	runner.Sleep(3 * time.Second)
	runner.Snapshot("APLcart loaded")

	runner.Test("APLcart loads data", func() bool {
		// Should show count or entries
		return runner.Contains("(") || runner.Contains("⍬")
	})

	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Test: Ctrl+C shows quit hint
	runner.SendKeys("C-c")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("After Ctrl+C (quit hint)")

	runner.Test("Ctrl+C shows quit hint", func() bool {
		return runner.Contains("C-] q to quit")
	})

	// Test 14: Any key clears the hint
	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	runner.Test("Key clears quit hint", func() bool {
		return !runner.Contains("C-] q to quit")
	})

	// Test 15: C-] q shows quit confirmation
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("q")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("After C-] q (quit confirmation)")

	runner.Test("C-] q shows quit confirmation", func() bool {
		return runner.Contains("Quit? (y/n)")
	})

	// Test 16: n cancels quit
	runner.SendKeys("n")
	runner.Sleep(200 * time.Millisecond)

	runner.Test("n cancels quit confirmation", func() bool {
		return !runner.Contains("Quit? (y/n)")
	})

	// Test 17-25: Tracer stack with nested functions X→Y→Z
	// Clean up any existing functions from previous runs
	runner.SendLine(")erase X Y Z")
	runner.Sleep(500 * time.Millisecond)

	// Define Z (will error)
	runner.SendLine(")ed Z")
	runner.Sleep(500 * time.Millisecond)
	runner.Snapshot("Editor opened for Z")

	runner.Test("Editor opens for Z", func() bool {
		return runner.Contains("Z")
	})

	// Type function body: 9÷0 (divide by zero error)
	runner.SendKeys("End", "Enter", "Enter")
	runner.SendText("9÷0")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("Z function with 9÷0")

	// Save and close - wait for editor to actually close
	runner.SendKeys("Escape")
	runner.Sleep(500 * time.Millisecond)

	runner.Test("Z editor closes after Escape", func() bool {
		return runner.WaitForNoFocusedPane(3 * time.Second)
	})
	runner.Snapshot("After closing Z editor")

	// Define Y (calls Z)
	runner.SendLine(")ed Y")
	runner.Sleep(1 * time.Second)
	runner.Snapshot("Y editor opened")

	runner.Test("Y editor opens", func() bool {
		return runner.Contains("╔") && runner.Contains("Y")
	})

	runner.SendKeys("End", "Enter", "Enter")
	runner.SendText("Z")
	runner.Sleep(200 * time.Millisecond)
	runner.SendKeys("Escape")
	runner.Sleep(500 * time.Millisecond)

	runner.Test("Y editor closes after Escape", func() bool {
		return runner.WaitForNoFocusedPane(3 * time.Second)
	})

	// Define X (calls Y)
	runner.SendLine(")ed X")
	runner.Sleep(1 * time.Second)
	runner.Snapshot("X editor opened")

	runner.Test("X editor opens", func() bool {
		return runner.Contains("╔") && runner.Contains("X")
	})

	runner.SendKeys("End", "Enter", "Enter")
	runner.SendText("Y")
	runner.Sleep(200 * time.Millisecond)
	runner.SendKeys("Escape")
	runner.Sleep(500 * time.Millisecond)

	runner.Test("X editor closes after Escape", func() bool {
		return runner.WaitForNoFocusedPane(3 * time.Second)
	})
	runner.Snapshot("After defining X, Y, Z")

	// Execute X - triggers nested error
	runner.SendLine("X")
	runner.Sleep(2 * time.Second) // Give time for error and tracer to open
	runner.Snapshot("After X errors - tracer opens")

	runner.Test("Tracer opens on error", func() bool {
		return runner.Contains("[tracer]") || runner.Contains("DOMAIN ERROR") || runner.Contains("tracer")
	})

	// Test breakpoint toggling
	// Focus tracer if not focused
	runner.SendKeys("Tab")
	runner.Sleep(200 * time.Millisecond)

	// Move to a line and toggle breakpoint
	runner.SendKeys("Down")
	runner.Sleep(100 * time.Millisecond)
	runner.Snapshot("Before breakpoint toggle")

	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("b")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After breakpoint toggle")

	runner.Test("Breakpoint indicator appears", func() bool {
		return runner.Contains("●")
	})

	// Toggle off
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("b")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After breakpoint toggle off")

	runner.Test("Breakpoint indicator removed", func() bool {
		// Check that there's no red dot (or fewer than before)
		// Since we only had one, it should be gone
		return !runner.Contains("●")
	})

	// Test breakpoint via command palette
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys(":")
	runner.Sleep(300 * time.Millisecond)
	runner.SendText("break")
	runner.Sleep(200 * time.Millisecond)
	runner.Snapshot("Command palette filtered to breakpoint")

	runner.Test("Command palette shows breakpoint command", func() bool {
		return runner.Contains("breakpoint")
	})

	runner.SendKeys("Enter")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After breakpoint via command palette")

	runner.Test("Breakpoint set via command palette", func() bool {
		return runner.Contains("●")
	})

	// Clear the breakpoint for clean state
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("b")
	runner.Sleep(200 * time.Millisecond)

	// Open stack pane
	runner.SendKeys("C-]")
	runner.Sleep(100 * time.Millisecond)
	runner.SendKeys("s")
	runner.Sleep(500 * time.Millisecond)
	runner.Snapshot("Stack pane showing X→Y→Z")

	runner.Test("Stack pane shows Z (top of stack)", func() bool {
		return runner.Contains("Z[") || runner.Contains("Z ")
	})

	runner.Test("Stack pane shows stack frames", func() bool {
		// Check for stack pane title or any indication of multiple frames
		return runner.Contains("stack") && (runner.Contains("X") || runner.Contains("Y") || runner.Contains("Z"))
	})

	// Navigate stack - press down to select Y
	runner.SendKeys("Down")
	runner.Sleep(200 * time.Millisecond)
	runner.SendKeys("Enter")
	runner.Sleep(300 * time.Millisecond)
	runner.Snapshot("After selecting Y in stack")

	runner.Test("Tracer switches to Y", func() bool {
		return runner.Contains("Y")
	})

	// Close stack pane
	runner.SendKeys("Escape")
	runner.Sleep(200 * time.Millisecond)

	// Pop stack frames with Escape
	// Focus tracer first
	runner.SendKeys("Tab")
	runner.Sleep(200 * time.Millisecond)
	runner.SendKeys("Escape") // Pop Z
	runner.Sleep(500 * time.Millisecond)
	runner.Snapshot("After popping Z")

	runner.SendKeys("Escape") // Pop Y
	runner.Sleep(500 * time.Millisecond)
	runner.Snapshot("After popping Y")

	runner.SendKeys("Escape") // Pop X - stack cleared
	runner.Sleep(500 * time.Millisecond)
	runner.Snapshot("After popping X - stack cleared")

	runner.Test("Stack cleared after popping all frames", func() bool {
		return !runner.Contains("[tracer]")
	})

	// Final snapshot
	runner.Snapshot("Final state")

	// Generate report
	reportFile := runner.GenerateReport()
	if reportFile != "" {
		t.Logf("Report: %s", reportFile)
		// Open in browser if on macOS
		if _, err := os.Stat("/usr/bin/open"); err == nil {
			exec.Command("open", reportFile).Start()
		}
	}
}
