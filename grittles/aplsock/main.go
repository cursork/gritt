// aplsock bootstraps a pure APL socket server inside Dyalog and serves
// the prepl protocol to external clients over TCP or Unix sockets.
//
// Usage:
//
//	aplsock -l -sock :4200           # Launch Dyalog, serve on TCP 4200
//	aplsock -sock /tmp/apl.sock      # Connect to existing Dyalog on :4502
//	aplsock -addr host:4502 -sock :4200
//
// Clients connect with netcat, telnet, or gritt (phase 2):
//
//	nc localhost 4200
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/cursork/gritt/prepl"
	"github.com/cursork/gritt/ride"
	"github.com/cursork/gritt/session"
)

func main() {
	addr := flag.String("addr", "localhost:4502", "Dyalog RIDE address")
	launch := flag.Bool("launch", false, "Launch Dyalog automatically")
	flag.BoolVar(launch, "l", false, "Launch Dyalog automatically")
	version := flag.String("version", "", "Dyalog version or path to binary")
	sock := flag.String("sock", ":4200", "Socket to serve on (:port or /path)")
	mode := flag.String("mode", "aplan", "Output mode: plain, aplan, aplor")
	// Legacy alias
	repl := flag.Bool("repl", false, "Legacy alias for -mode plain")
	flag.Parse()

	if *repl {
		*mode = "plain"
	}

	// 1. Launch Dyalog if requested
	var dyalogCmd *exec.Cmd
	if *launch {
		var port int
		dyalogCmd, port = launchDyalog(*version)
		*addr = fmt.Sprintf("localhost:%d", port)
	}
	cleanup := func() {
		if dyalogCmd != nil && dyalogCmd.Process != nil {
			killProcessGroup(dyalogCmd)
			dyalogCmd.Wait()
		}
	}
	defer cleanup()

	// 2. Connect via RIDE
	rc, err := ride.Connect(*addr)
	if err != nil {
		log.Fatalf("RIDE connect to %s: %v", *addr, err)
	}
	log.Printf("RIDE connected to %s", *addr)

	// 3. Bootstrap: inject APL prepl code, set mode, start server on a thread
	internalPort := 10000 + rand.Intn(50000)
	bootstrap(rc, internalPort, *mode)

	// 4. Drain RIDE messages in background so the connection doesn't back up.
	go func() {
		for {
			msg, _, err := rc.Recv()
			if err != nil {
				return
			}
			if msg != nil && msg.Command == "AppendSessionOutput" {
				if result, ok := msg.Args["result"].(string); ok {
					if t, ok := msg.Args["type"].(float64); !ok || t != 14 {
						log.Printf("APL: %s", strings.TrimRight(result, "\n"))
					}
				}
			}
		}
	}()

	// 5. Connect to the APL prepl server
	preplAddr := fmt.Sprintf("localhost:%d", internalPort)
	pc := waitForPrepl(preplAddr)
	log.Printf("prepl connected on internal port %d", internalPort)

	// 6. Serve external clients (pc shared across all client connections)
	serve(pc, *sock, *mode, cleanup)
}

// launchDyalog starts Dyalog APL with RIDE on a random port.
func launchDyalog(version string) (*exec.Cmd, int) {
	exe, err := session.FindDyalog(version)
	if err != nil {
		log.Fatal(err)
	}

	port := 10000 + rand.Intn(50000)
	cmd := exec.Command(exe, "+s", "-q")
	cmd.Env = append(os.Environ(), fmt.Sprintf("RIDE_INIT=SERVE:*:%d", port))
	cmd.Env = append(cmd.Env, "RIDE_SPAWNED=1", "DYALOG_LINEEDITOR_MODE=1")
	cmd.Env = append(cmd.Env, session.DyalogEnv(exe)...)
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		log.Fatalf("start Dyalog (%s): %v", exe, err)
	}

	rideAddr := fmt.Sprintf("localhost:%d", port)
	for i := 0; i < 50; i++ {
		conn, err := net.DialTimeout("tcp", rideAddr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			log.Printf("Dyalog launched on RIDE port %d (pid %d)", port, cmd.Process.Pid)
			return cmd, port
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Fatalf("Dyalog did not start RIDE on port %d", port)
	return nil, 0
}

// bootstrap injects the APL prepl namespace, sets mode, and starts the server.
func bootstrap(rc *ride.Client, port int, mode string) {
	f, err := os.CreateTemp("", "prepl-*.apln")
	if err != nil {
		log.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(prepl.Source); err != nil {
		log.Fatalf("write temp file: %v", err)
	}
	f.Close()
	defer os.Remove(f.Name())

	out, err := rc.Execute(fmt.Sprintf("2 ⎕FIX 'file://%s'", f.Name()))
	if err != nil {
		log.Fatalf("⎕FIX failed: %v", err)
	}
	log.Printf("⎕FIX: %s", strings.Join(out, ""))

	// Set output mode before starting
	if mode != "aplan" {
		out, err = rc.Execute(fmt.Sprintf("Prepl.SetMode '%s'", mode))
		if err != nil {
			log.Fatalf("SetMode failed: %v", err)
		}
		log.Printf("SetMode: %s", mode)
	}

	out, err = rc.Execute("Prepl.LoadConga")
	if err != nil {
		log.Fatalf("LoadConga failed: %v", err)
	}
	log.Printf("LoadConga: %s", strings.Join(out, ""))

	out, err = rc.Execute(fmt.Sprintf("Prepl.Start&%d", port))
	if err != nil {
		log.Fatalf("Prepl.Start failed: %v", err)
	}
	log.Printf("Prepl.Start: thread %s", strings.Join(out, ""))
}

// waitForPrepl polls until the APL prepl server is accepting connections.
func waitForPrepl(addr string) *prepl.Client {
	for i := 0; i < 50; i++ {
		pc, err := prepl.Connect(addr)
		if err == nil {
			return pc
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Fatalf("prepl server did not start on %s", addr)
	return nil
}

// serve listens on the given address and proxies client connections to the APL prepl.
func serve(pc *prepl.Client, sockAddr string, mode string, cleanup func()) {
	var listener net.Listener
	var err error
	var isUnix bool

	if strings.Contains(sockAddr, ":") {
		listener, err = net.Listen("tcp", sockAddr)
	} else {
		isUnix = true
		os.Remove(sockAddr)
		listener, err = net.Listen("unix", sockAddr)
	}
	if err != nil {
		log.Fatalf("listen %s: %v", sockAddr, err)
	}
	defer listener.Close()

	log.Printf("serving on %s", sockAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, termSignals()...)
	go func() {
		<-sigCh
		listener.Close()
		pc.Close()
		if isUnix {
			os.Remove(sockAddr)
		}
		cleanup()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		switch mode {
		case "plain":
			go handleConnPlain(pc, conn)
		default:
			// Both 'aplan' and 'aplor' use raw APLAN passthrough.
			// The difference is what the APL side puts in val:
			// aplan → APLAN text, aplor → 220⌶ signed int vector.
			// The consumer knows which mode and parses accordingly.
			go handleConn(pc, conn)
		}
	}
}

// handleConn pipes between client and APL prepl — raw APLAN passthrough.
// No parsing, no decoding. Tooling reads the tagged APLAN protocol directly.
func handleConn(pc *prepl.Client, conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		expr := scanner.Text()
		if expr == "" {
			continue
		}
		raw, err := pc.EvalRaw(expr)
		if err != nil {
			log.Printf("eval error: %v", err)
			return
		}
		fmt.Fprintf(conn, "%s\n", raw)
	}
}

// handleConnPlain decodes APLAN and returns plain text — for interactive use.
func handleConnPlain(pc *prepl.Client, conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		expr := scanner.Text()
		if expr == "" {
			continue
		}
		resp, err := pc.Eval(expr)
		if err != nil {
			log.Printf("eval error: %v", err)
			return
		}
		switch resp.Tag {
		case "ret":
			if resp.Raw != "" {
				fmt.Fprintf(conn, "%s\n", resp.Raw)
			}
		case "err":
			fmt.Fprintf(conn, "%s\n", resp.Err.Message)
			for _, line := range resp.Err.DM {
				fmt.Fprintf(conn, "%s\n", line)
			}
		}
	}
}
