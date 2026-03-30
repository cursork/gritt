//go:build windows

package main

import (
	"os"
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}

func termSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
