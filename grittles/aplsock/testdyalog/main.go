// Starts Dyalog with RIDE on port 4502 and waits. Used by test.sh.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"

	"github.com/cursork/gritt/session"
)

func main() {
	exe, err := session.FindDyalog("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cmd := exec.Command(exe, "+s", "-q")
	cmd.Env = append(os.Environ(), "RIDE_INIT=SERVE:*:14502")
	cmd.Env = append(cmd.Env, "RIDE_SPAWNED=1", "DYALOG_LINEEDITOR_MODE=1")
	cmd.Env = append(cmd.Env, session.DyalogEnv(exe)...)
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(cmd.Process.Pid)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, termSignals()...)
	<-sigCh
	killProcessGroup(cmd)
}
