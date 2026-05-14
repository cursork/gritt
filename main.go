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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/colorprofile"
	"github.com/cursork/gritt/ride"
	"github.com/cursork/gritt/session"
)

// launchDyalog starts Dyalog APL with RIDE on a random port.
// version constrains which installed version to use (empty = highest available).
func launchDyalog(version string) (*exec.Cmd, int) {
	exe := resolveDyalog(version)

	port := 10000 + rand.Intn(50000)
	cmd := exec.Command(exe, "+s", "-q")
	cmd.Env = append(os.Environ(), fmt.Sprintf("RIDE_INIT=SERVE:*:%d", port))
	cmd.Env = append(cmd.Env, session.DyalogEnv(exe)...)
	setProcessGroup(cmd)
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
func resolveDyalog(version string) string {
	exe, err := session.FindDyalog(version)
	if err != nil {
		log.Fatal(err)
	}
	return exe
}

// closeClient is the standard cleanup for a non-TUI RIDE connection. When
// gritt owns the Dyalog process (-l) we send `)off` first so the
// interpreter exits cleanly; then we close our end of the socket.
// Best-effort: errors are ignored — the deferred gracefulKillDyalog still
// escalates if needed.
func closeClient(c *ride.Client, ownsDyalog bool) {
	if c == nil {
		return
	}
	if ownsDyalog {
		_ = c.Send("Execute", map[string]any{"text": ")off\n", "trace": 0})
	}
	c.Close()
}

// gracefulKillDyalog sends SIGTERM, waits up to timeout for the launch-time
// wait goroutine to observe the exit (via the closed `exited` channel),
// then SIGKILLs and waits for the reap. Silent — used by non-TUI cleanup
// paths (-e, -stdin, -sock, -fmt, signal handler, panic safety net) where
// the caller-process exit code is the only feedback channel.
//
// SIGTERM is mostly a formality for gritt-launched Dyalog (no controlling
// tty, handler hangs trying to surface a destructor debug — see
// adnotata/0011-graceful-kill-and-protocol.md), but it's the right
// protocol-level "please exit" signal and may take effect in future
// Dyalog versions. The SIGKILL escalation on timeout is the actual safety
// net.
//
// Safe to call repeatedly: short-circuits if `exited` is already closed.
func gracefulKillDyalog(cmd *exec.Cmd, exited <-chan struct{}, timeout time.Duration) {
	if cmd == nil {
		return
	}
	select {
	case <-exited:
		return
	default:
	}
	terminateProcessGroup(cmd)
	select {
	case <-exited:
		return
	case <-time.After(timeout):
	}
	killProcessGroup(cmd)
	<-exited
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
	sock := flag.String("sock", "", "Listen for injection on Unix path (contains '/') or TCP port (e.g. 9876, :9876, host:port)")
	var links multiFlag
	flag.Var(&links, "link", "Link directory (path or ns:path, can be repeated)")
	launch := flag.Bool("launch", false, "Launch Dyalog automatically (alias: -l)")
	flag.BoolVar(launch, "l", false, "Launch Dyalog automatically")
	version := flag.String("version", "", "Dyalog version (e.g. 20.0) or path to binary")
	fmtMode := flag.Bool("fmt", false, "Format APL files in place")
	historyMode := flag.Bool("history", false, "Print command history to stdout")
	var cfgFlag string
	var cfgSet bool
	flag.Func("cfg", "Config file path ('' = no config, use defaults)", func(s string) error {
		cfgFlag = s
		cfgSet = true
		return nil
	})
	flag.Parse()

	// Print history and exit — no Dyalog needed
	if *historyMode {
		printHistory()
		return
	}

	var cfgArg *string
	if cfgSet {
		cfgArg = &cfgFlag
	}

	// Launch Dyalog if requested
	var dyalogCmd *exec.Cmd
	dyalogExited := make(chan struct{}) // pre-closed unless we launch
	close(dyalogExited)
	if *launch {
		var port int
		dyalogCmd, port = launchDyalog(*version)
		*addr = fmt.Sprintf("localhost:%d", port)

		// One owner of cmd.Wait() — closes the channel when the process
		// truly exits (and reaps it), so the TUI can distinguish "still
		// running" from "exited but not yet reaped (zombie)".
		dyalogExited = make(chan struct{})
		go func() {
			dyalogCmd.Wait()
			close(dyalogExited)
		}()

		// Resolve kill_timeout once for non-TUI cleanup paths. The TUI
		// reloads config inside NewModel (cheap, JSON parse).
		cfg := LoadConfig(cfgArg)
		killTimeoutSec := cfg.KillTimeout
		if killTimeoutSec <= 0 {
			killTimeoutSec = DefaultKillTimeout
		}
		killTimeout := time.Duration(killTimeoutSec) * time.Second

		// Safety-net cleanup for crashes/panics and non-TUI modes (-e, -stdin,
		// -sock, -fmt). The TUI's normal quit flow handles graceful kill via
		// the kill-wait modal. This helper is silent — exit code is the
		// only signal scripted callers see.
		defer gracefulKillDyalog(dyalogCmd, dyalogExited, killTimeout)

		// Handle external signals (kill PID etc.) — no chance to show UI.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, termSignals()...)
		go func() {
			<-sigCh
			gracefulKillDyalog(dyalogCmd, dyalogExited, killTimeout)
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
		defer closeClient(client, *launch)
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
		defer closeClient(client, *launch)
		runLinks(client, links)
		var executed []string
		for _, expr := range exprs {
			// Split multiline expressions and execute each line
			for _, line := range strings.Split(expr, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					runExpr(client, line)
					executed = append(executed, line)
				}
			}
		}
		if err := appendHistory(executed); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *stdin {
		client, err := ride.Connect(*addr)
		if err != nil {
			log.Fatal(err)
		}
		defer closeClient(client, *launch)
		runLinks(client, links)
		var executed []string
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			runExpr(client, line)
			executed = append(executed, line)
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		if err := appendHistory(executed); err != nil {
			log.Fatal(err)
		}
		return
	}
	// Interactive TUI mode
	colorProfile := colorprofile.Detect(os.Stdout, os.Environ())
	// Trust COLORTERM over colorprofile's tmux heuristics
	if ct := os.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		colorProfile = colorprofile.TrueColor
	}

	p := tea.NewProgram(NewModel(*addr, logWriter, colorProfile, cfgArg, dyalogCmd, dyalogExited), tea.WithAltScreen(), tea.WithMouseCellMotion())

	// -sock injection listener. Runs alongside the TUI; each connection's
	// lines are submitted into the bubbletea program and processed
	// sequentially when the interpreter is idle.
	if *sock != "" {
		network, address := parseSockAddr(*sock)
		listener, err := startSocketListener(network, address, p)
		if err != nil {
			log.Fatalf("Failed to open -sock listener: %v", err)
		}
		defer listener.Close()
		if network == "unix" {
			defer os.Remove(address)
		}
	}

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

// runFormat formats APL files in place using FormatCode
func runFormat(client *ride.Client, files []string) {
	// Lazily opened windows — one for functions, one for namespaces
	fnToken := -1
	nsToken := -1

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", file, err)
		}
		content := string(data)
		content = strings.TrimRight(content, "\n")
		lines := strings.Split(content, "\n")

		// Detect namespace files by first non-blank line
		isNamespace := false
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				isNamespace = strings.HasPrefix(trimmed, ":Namespace") ||
					strings.HasPrefix(trimmed, ":Class") ||
					strings.HasPrefix(trimmed, ":Interface")
				break
			}
		}

		var token int
		if isNamespace {
			if nsToken < 0 {
				nsToken = openDummyNamespace(client)
			}
			token = nsToken
		} else {
			if fnToken < 0 {
				fnToken = openDummyEditor(client)
			}
			token = fnToken
		}

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

	// Close dummy windows
	if fnToken >= 0 {
		client.Send("CloseWindow", map[string]any{"win": fnToken})
	}
	if nsToken >= 0 {
		client.Send("CloseWindow", map[string]any{"win": nsToken})
	}
}

var fmtCounter int

// openDummyEditor opens a dummy function editor and returns its token.
func openDummyEditor(client *ride.Client) int {
	fmtCounter++
	name := fmt.Sprintf("gritt∆fmt%d", fmtCounter)
	if err := client.Send("Edit", map[string]any{
		"win":  0,
		"text": name,
		"pos":  0,
	}); err != nil {
		log.Fatalf("Failed to send Edit: %v", err)
	}
	return waitForOpenWindow(client)
}

// openDummyNamespace creates a dummy namespace via ⎕FIX and opens its editor.
func openDummyNamespace(client *ride.Client) int {
	fmtCounter++
	name := fmt.Sprintf("gritt∆fmt%d", fmtCounter)
	runExpr(client, fmt.Sprintf("⎕FIX ':Namespace %s' ':EndNamespace'", name))
	if err := client.Send("Edit", map[string]any{
		"win":  0,
		"text": name,
		"pos":  0,
	}); err != nil {
		log.Fatalf("Failed to send Edit: %v", err)
	}
	return waitForOpenWindow(client)
}

// waitForOpenWindow reads messages until OpenWindow arrives and returns its token.
func waitForOpenWindow(client *ride.Client) int {
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
