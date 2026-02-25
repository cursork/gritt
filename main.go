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

// launchDyalog starts Dyalog APL with RIDE on a random port.
// version constrains which installed version to use (empty = highest available).
func launchDyalog(version string) (*exec.Cmd, int) {
	exe := resolveDyalog(version)

	port := 10000 + rand.Intn(50000)
	cmd := exec.Command(exe, "+s", "-q")
	cmd.Env = append(os.Environ(), fmt.Sprintf("RIDE_INIT=SERVE:*:%d", port))
	cmd.Env = append(cmd.Env, dyalogEnv(exe)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start Dyalog (%s): %v", exe, err)
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

// resolveDyalog finds the Dyalog binary to use.
// If version is empty, tries PATH first then discovery.
// If version contains a path separator, treats it as a direct path to the binary.
// Otherwise, uses discovery to find that specific version.
func resolveDyalog(version string) string {
	// Direct path (contains / or \)
	if strings.ContainsAny(version, `/\`) {
		if _, err := os.Stat(version); err != nil {
			log.Fatalf("Dyalog binary not found: %s", version)
		}
		return version
	}

	// If no version requested, try PATH first
	if version == "" {
		if path, err := exec.LookPath("dyalog"); err == nil {
			return path
		}
	}

	// Discovery
	if exe := findDyalog(version); exe != "" {
		return exe
	}

	// Helpful error
	if version != "" {
		log.Fatalf("Dyalog version %s not found.\nSearched:\n  %s", version, searchedPaths())
	}
	log.Fatalf("Dyalog not found in PATH or standard install locations.\nSearched:\n  %s\n  %s",
		"$PATH", searchedPaths())
	return ""
}

// multiFlag allows a flag to be specified multiple times, collecting all values
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ", ") }
func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	addr := flag.String("addr", "localhost:4502", "Dyalog RIDE address")
	logFile := flag.String("log", "", "Log protocol messages to file")
	var exprs multiFlag
	flag.Var(&exprs, "e", "Execute expression and exit (can be repeated)")
	stdin := flag.Bool("stdin", false, "Read expressions from stdin")
	sock := flag.String("sock", "", "Unix socket path for APL server")
	var links multiFlag
	flag.Var(&links, "link", "Link directory (path or ns:path, can be repeated)")
	launch := flag.Bool("launch", false, "Launch Dyalog automatically (alias: -l)")
	flag.BoolVar(launch, "l", false, "Launch Dyalog automatically")
	version := flag.String("version", "", "Dyalog version (e.g. 20.0) or path to binary")
	fmtMode := flag.Bool("fmt", false, "Format APL files in place")
	flag.Parse()

	// Launch Dyalog if requested
	var dyalogCmd *exec.Cmd
	if *launch {
		var port int
		dyalogCmd, port = launchDyalog(*version)
		*addr = fmt.Sprintf("localhost:%d", port)

		// Cleanup function to kill Dyalog process group
		killDyalog := func() {
			if dyalogCmd.Process != nil {
				syscall.Kill(-dyalogCmd.Process.Pid, syscall.SIGKILL)
				dyalogCmd.Wait() // Reap zombie
			}
		}
		defer killDyalog()

		// Handle signals to ensure cleanup on Ctrl+C, etc.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			killDyalog()
			os.Exit(0)
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

	// Format mode
	if *fmtMode {
		files := flag.Args()
		if len(files) == 0 {
			log.Fatal("-fmt requires at least one file")
		}
		client, err := ride.Connect(*addr)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
		runFormat(client, files)
		return
	}

	// Non-interactive mode
	if len(exprs) > 0 && *stdin {
		log.Fatal("-e and -stdin are mutually exclusive")
	}
	if len(exprs) > 0 {
		client, err := ride.Connect(*addr)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
		runLinks(client, links)
		for _, expr := range exprs {
			// Split multiline expressions and execute each line
			for _, line := range strings.Split(expr, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					runExpr(client, line)
				}
			}
		}
		return
	}
	if *stdin {
		client, err := ride.Connect(*addr)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
		runLinks(client, links)
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
		runLinks(client, links)
		runSocket(client, *sock)
		return
	}

	// Interactive TUI mode
	colorProfile := colorprofile.Detect(os.Stdout, os.Environ())
	// Trust COLORTERM over colorprofile's tmux heuristics
	if ct := os.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		colorProfile = colorprofile.TrueColor
	}

	p := tea.NewProgram(NewModel(*addr, logWriter, colorProfile), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

// runLinks runs ]link.create for each spec
func runLinks(client *ride.Client, specs []string) {
	for _, spec := range specs {
		runLink(client, spec)
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
				c.Write([]byte(result))
				mu.Unlock()
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

// runFormat formats APL files in place using FormatCode
func runFormat(client *ride.Client, files []string) {
	token := openDummyEditor(client)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", file, err)
		}
		content := string(data)
		// Strip trailing newline for splitting, we'll add it back
		content = strings.TrimRight(content, "\n")
		lines := strings.Split(content, "\n")

		formatted := formatCode(client, token, lines)

		// Check if anything changed
		changed := len(lines) != len(formatted)
		if !changed {
			for i := range lines {
				if lines[i] != formatted[i] {
					changed = true
					break
				}
			}
		}

		if changed {
			out := strings.Join(formatted, "\n") + "\n"
			if err := os.WriteFile(file, []byte(out), 0644); err != nil {
				log.Fatalf("Failed to write %s: %v", file, err)
			}
			fmt.Println(file)
		}
	}

	// Close the dummy editor
	client.Send("CloseWindow", map[string]any{"win": token})
}

// openDummyEditor opens a dummy editor window and returns its token.
// FormatCode requires a valid window token that the interpreter knows about.
func openDummyEditor(client *ride.Client) int {
	if err := client.Send("Edit", map[string]any{
		"win":  0,
		"text": "gritt∆fmt",
		"pos":  0,
	}); err != nil {
		log.Fatalf("Failed to send Edit: %v", err)
	}

	for {
		msg, _, err := client.Recv()
		if err != nil {
			log.Fatalf("Recv failed waiting for OpenWindow: %v", err)
		}
		if msg != nil && msg.Command == "OpenWindow" {
			if t, ok := msg.Args["token"].(float64); ok {
				return int(t)
			}
		}
	}
}

// formatCode sends FormatCode and waits for ReplyFormatCode
func formatCode(client *ride.Client, win int, lines []string) []string {
	text := make([]any, len(lines))
	for i, l := range lines {
		text[i] = l
	}

	if err := client.Send("FormatCode", map[string]any{
		"win":  win,
		"text": text,
	}); err != nil {
		log.Fatalf("FormatCode failed: %v", err)
	}

	for {
		msg, _, err := client.Recv()
		if err != nil {
			log.Fatalf("Recv failed waiting for ReplyFormatCode: %v", err)
		}
		if msg != nil && msg.Command == "ReplyFormatCode" {
			if result, ok := msg.Args["text"].([]any); ok {
				out := make([]string, len(result))
				for i, l := range result {
					out[i], _ = l.(string)
				}
				return out
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
