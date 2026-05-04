//go:build windows

package main

import (
	"os"
	"os/exec"
)

// setProcessGroup is a no-op on Windows.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup kills the process on Windows.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// terminateProcessGroup has no graceful equivalent on Windows; behaves as kill.
func terminateProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// processAlive reports whether the process is still running.
func processAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	return cmd.ProcessState == nil
}

// termSignals returns the signals to watch for graceful shutdown.
func termSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
