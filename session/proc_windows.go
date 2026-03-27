//go:build windows

package session

import (
	"os/exec"
	"time"
)

// setProcAttr is a no-op on Windows (no process groups via Setpgid).
func setProcAttr(cmd *exec.Cmd) {}

// kill terminates the process, waiting up to 3s for graceful exit.
func kill(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	cmd.Process.Kill()

	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		<-done
	}
}
