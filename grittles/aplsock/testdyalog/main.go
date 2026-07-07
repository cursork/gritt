// Starts Dyalog with RIDE on port 4502 and waits. Used by test.sh.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/cursork/gritt/session"
)

func main() {
	cmd, stdin, _, err := session.StartInterpreter(context.Background(), session.StartOptions{Port: 14502})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer stdin.Close()
	fmt.Println(cmd.Process.Pid)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, termSignals()...)
	<-sigCh
	killProcessGroup(cmd)
}
