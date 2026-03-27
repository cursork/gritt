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

// termSignals returns the signals to watch for graceful shutdown.
func termSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
