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
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/colorprofile"
	"github.com/cursork/gritt/ride"
)

const splash = `
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⡄⣀⡰⡆⠖⠢⢌⠲⠙⠤⠀⠐⠀⡄⠀⠠⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣇⡒⠂⣈⠃⠀⠂⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠂⠀⠀⠀⠈⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠄⠉⠵⠉⠀⠀⠄⠀⠀⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠄⣀⣀⣀⣀⣀⣠⡤⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠂⢄⣔⠄⠐⠀⠈⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠠⢀⣈⣽⣿⣿⣿⣿⣿⣿⣿⣷⣄⠀⠀⠀⠀
⠀⠀⠀⠀⢀⠬⠜⠦⠈⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠐⠒⠶⣶⣶⣿⣻⣿⣿⣿⣿⣿⣿⣿⣿⣦⡀⠀⠀
⠀⠀⠀⠀⠈⢀⠈⠀⠀⠀⠄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⢛⣻⣿⣿⣿⣿⣿⣿⣿⡏⠀⠉⠉⠙⠿⠋⠀⠀
⠀⠀⠈⢀⢁⠀⠀⠀⠀⠀⠀⣀⣠⡤⣠⣶⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⢄⣀⣀⣀⣀⣐⣲⣿⣿⣿⠛⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⢿⣿⣦⣤⣀⡀⠀⠂⢀⡀⠀
⠀⠀⡀⠉⢙⣛⣻⣿⠿⢛⠝⠁⢀⣿⣿⣿⣿⡿⢻⠿⣿⣿⣿⣿⣿⣿⣽⢿⣿⣿⣶⣬⡍⣿⡇⠀⠉⢢⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣶⣾⣿⡿⣿⣟⣯⣾⠿⠋⠀⠀⠀⠉⠉⠀⠀⣠⣄⣤⡾⢛⣡⣿⠀⢠⢤⢼⠀⠀
⠀⠀⠀⠀⠐⠀⠂⠀⢠⣿⠋⠁⢠⣿⠟⠋⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⠛⠋⠉⠀⠞⠋⠁⢨⡤⣿⡇⠀⠀
⠀⠀⠀⠀⠀⢀⣴⣶⠿⠁⣀⣠⡿⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⠀⢠⠂⠊⠖⣻⣟⡟⠀⠀⠀
⠀⠀⠀⠀⡐⠈⠉⠀⠀⠔⠛⠋⠁⠀⠀⠠⠀⠀⠀⠐⠀⠀⠀⠀⠀⢀⠠⡈⣤⣤⣧⣿⣿⣿⣿⠍⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⢠⣠⡠⣆⡐⢀⢀⠄⢄⢀⢤⡀⣄⠀⢀⣀⡬⡠⣶⣶⣿⣿⣾⣾⣿⣿⡿⠋⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠈⠂⠚⠯⣻⡿⣿⣻⣾⣿⣿⣾⣿⣽⣷⣷⣟⣿⣿⣿⣿⣿⣿⣿⢿⡿⠋⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠻⢿⣟⣿⢿⣿⣿⣿⣿⣮⣿⣿⣿⣿⣿⣿⠿⠻⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠈⠈⠛⠞⠟⡻⠛⠛⠛⠛⠛⠉⠁⠠⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
`

// launchDyalog starts Dyalog APL with RIDE on a random port
func launchDyalog() (*exec.Cmd, int) {
	port := 10000 + rand.Intn(50000)
	cmd := exec.Command("dyalog", "+s", "-q")
	cmd.Env = append(os.Environ(), fmt.Sprintf("RIDE_INIT=SERVE:*:%d", port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start Dyalog: %v", err)
	}
	// Poll for RIDE to be ready
	addr := fmt.Sprintf("localhost:%d", port)
	for i := 0; i < 50; i++ { // 5 second timeout
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return cmd, port
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Fatalf("Dyalog did not start on port %d", port)
	return nil, 0
}

func main() {
	addr := flag.String("addr", "localhost:4502", "Dyalog RIDE address")
	logFile := flag.String("log", "", "Log protocol messages to file")
	expr := flag.String("e", "", "Execute expression and exit")
	stdin := flag.Bool("stdin", false, "Read expressions from stdin")
	sock := flag.String("sock", "", "Unix socket path for APL server")
	link := flag.String("link", "", "Link directory (path or ns:path)")
	launch := flag.Bool("launch", false, "Launch Dyalog automatically (alias: -l)")
	flag.BoolVar(launch, "l", false, "Launch Dyalog automatically")
	flag.Parse()

	// Launch Dyalog if requested
	var dyalogCmd *exec.Cmd
	if *launch {
		var port int
		dyalogCmd, port = launchDyalog()
		*addr = fmt.Sprintf("localhost:%d", port)
		defer func() {
			if dyalogCmd.Process != nil {
				// Kill process group to clean up helper processes
				syscall.Kill(-dyalogCmd.Process.Pid, syscall.SIGKILL)
			}
		}()
	}

	// Set up logging if requested
	var logWriter *os.File
	if *logFile != "" {
		var err error
		logWriter, err = os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer logWriter.Close()
		ride.Logger = logWriter
	}

	// Non-interactive mode
	if *expr != "" && *stdin {
		log.Fatal("-e and -stdin are mutually exclusive")
	}
	if *expr != "" {
		client, err := ride.Connect(*addr)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
		if *link != "" {
			runLink(client, *link)
		}
		runExpr(client, *expr)
		return
	}
	if *stdin {
		client, err := ride.Connect(*addr)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
		if *link != "" {
			runLink(client, *link)
		}
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			runExpr(client, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *sock != "" {
		client, err := ride.Connect(*addr)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
		if *link != "" {
			runLink(client, *link)
		}
		runSocket(client, *sock)
		return
	}

	// Interactive TUI mode
	colorProfile := colorprofile.Detect(os.Stdout, os.Environ())
	// Trust COLORTERM over colorprofile's tmux heuristics
	if ct := os.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		colorProfile = colorprofile.TrueColor
	}

	fmt.Print(splash)
	fmt.Printf("\n  gritt - Go RIDE Terminal\n  Connecting to %s...\n", *addr)
	client, err := ride.Connect(*addr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	p := tea.NewProgram(NewModel(client, *addr, logWriter, colorProfile), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

// runLink runs ]link.create with the given spec
func runLink(client *ride.Client, spec string) {
	var cmd string
	if idx := strings.Index(spec, ":"); idx >= 0 {
		// ns:path -> ]link.create ns path
		ns := spec[:idx]
		path := spec[idx+1:]
		cmd = fmt.Sprintf("]link.create %s %s", ns, path)
	} else {
		// path -> ]link.create path
		cmd = fmt.Sprintf("]link.create %s", spec)
	}
	runExpr(client, cmd)
}

// runSocket starts a Unix domain socket server for APL expressions
func runSocket(client *ride.Client, sockPath string) {
	// Remove stale socket
	os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Fatalf("Failed to create socket: %v", err)
	}
	defer listener.Close()
	defer os.Remove(sockPath)

	// Handle signals for cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		listener.Close()
		os.Remove(sockPath)
		os.Exit(0)
	}()

	fmt.Printf("Listening on %s\n", sockPath)

	var mu sync.Mutex
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Listener closed (signal handler)
			return
		}

		go func(c net.Conn) {
			defer c.Close()

			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				expr := strings.TrimSpace(scanner.Text())
				if expr == "" {
					continue
				}

				// Serialize execution (RIDE is single-threaded)
				mu.Lock()
				result := execCapture(client, expr)
				mu.Unlock()

				c.Write([]byte(result))
			}
		}(conn)
	}
}

// execCapture executes an expression and returns the result as a string
func execCapture(client *ride.Client, expr string) string {
	var buf strings.Builder

	if err := client.Send("Execute", map[string]any{
		"trace": 0,
		"text":  expr + "\n",
	}); err != nil {
		return fmt.Sprintf("Execute failed: %v\n", err)
	}

	for {
		msg, _, err := client.Recv()
		if err != nil {
			return buf.String() + fmt.Sprintf("Recv failed: %v\n", err)
		}

		switch msg.Command {
		case "AppendSessionOutput":
			if t, ok := msg.Args["type"].(float64); ok && t == 14 {
				continue
			}
			if result, ok := msg.Args["result"].(string); ok {
				buf.WriteString(result)
			}
		case "SetPromptType":
			// Return on type > 0:
			// - type 1: ready for input (expression complete)
			// - type 2: quad input (⎕:)
			// - type 3: quote-quad input (⍞)
			// - type 0: no prompt (processing) - keep waiting
			if t, ok := msg.Args["type"].(float64); ok && t > 0 {
				return buf.String()
			}
		}
	}
}

// runExpr executes an expression and prints the result
func runExpr(client *ride.Client, expr string) {
	// Send execute
	if err := client.Send("Execute", map[string]any{
		"text":  expr + "\n",
		"trace": 0,
	}); err != nil {
		log.Fatalf("Execute failed: %v", err)
	}

	// Read until we get SetPromptType with type:1 (ready)
	for {
		msg, _, err := client.Recv()
		if err != nil {
			log.Fatalf("Recv failed: %v", err)
		}

		switch msg.Command {
		case "AppendSessionOutput":
			// type:14 is input echo, skip it
			if t, ok := msg.Args["type"].(float64); ok && t == 14 {
				continue
			}
			if result, ok := msg.Args["result"].(string); ok {
				fmt.Print(result)
			}
		case "SetPromptType":
			if t, ok := msg.Args["type"].(float64); ok && t == 1 {
				return // Ready for next input
			}
		}
	}
}
