package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// TODO: this is hoisted out of a more complete implementation (the one aplsock
//       uses) - it may be the case we need to unify the two, or at least
//       ensure no divergence in behaviour.

// socketRequest is a single line injected via -sock. The reader goroutine
// owns the connection and blocks on done until the TUI finishes the Execute;
// the result then gets written back to the connection.
//
// outputs accumulates AppendSessionOutput chunks for this request. It is
// only appended to while m.activeSocket == this request, so the request
// itself owns its captured output — no parallel buffer in the Model.
type socketRequest struct {
	code    string
	outputs []string
	done    chan string
}

// socketLineMsg delivers a queued injection to the bubbletea program.
type socketLineMsg struct {
	req *socketRequest
}

// parseSockAddr decides whether the -sock value is a Unix path or a TCP
// address. Values containing '/' are paths; a bare integer becomes ":N";
// anything else is passed through (`:9876`, `host:port`).
func parseSockAddr(value string) (network, address string) {
	if strings.Contains(value, "/") {
		return "unix", value
	}
	if _, err := strconv.Atoi(value); err == nil {
		return "tcp", ":" + value
	}
	return "tcp", value
}

// startSocketListener opens a listener and spawns the accept loop. Each
// accepted connection gets its own goroutine that reads newline-delimited
// expressions, submits each to the TUI via socketLineMsg, blocks on the
// response, and writes it back to the connection.
func startSocketListener(network, address string, p *tea.Program) (net.Listener, error) {
	if network == "unix" {
		_ = os.Remove(address) // stale socket would block Listen
	}
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, fmt.Errorf("listen %s %s: %w", network, address, err)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handleSocketConn(conn, p)
		}
	}()
	return l, nil
}

func handleSocketConn(conn net.Conn, p *tea.Program) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		req := &socketRequest{code: line, done: make(chan string, 1)}
		p.Send(socketLineMsg{req: req})
		reply := <-req.done
		if _, err := conn.Write([]byte(reply)); err != nil {
			return // client gone
		}
	}
}
