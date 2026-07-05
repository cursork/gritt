// Package adapter implements the Debug Adapter that bridges DAP and RIDE protocols.
package adapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cursork/gritt/dap"
	"github.com/cursork/gritt/ride"
)

// Adapter bridges the DAP protocol to the RIDE protocol.
type Adapter struct {
	reader *bufio.Reader
	writer io.Writer

	ride   *ride.Client
	sendMu sync.Mutex // Only protects Send operations

	seq   int
	seqMu sync.Mutex

	// Configuration from launch/attach
	config LaunchConfig

	// State tracking
	initialized bool
	configured  bool
	connected   bool

	// Track windows for stepping commands
	activeWindow   int
	activeWindowMu sync.RWMutex

	// Event channel for async events
	events chan *dap.Event

	// Breakpoints by function name, and source paths
	breakpoints   map[string][]int
	sourcePaths   map[string]string // funcName -> file path
	breakpointsMu sync.RWMutex

	// Source mapping: function name -> file location
	sourceMap   map[string]funcLocation
	sourceMapMu sync.RWMutex

	// Pending breakpoints to replay after program loads
	pendingStops []pendingStop

	// Variable references for scopes/variables
	varRefCounter int
	varRefs       map[int]varRef
	varRefsMu     sync.Mutex

	// Interpreter process (if launched)
	process *exec.Cmd

	// Windows from interpreter
	windows   map[int]*Window
	windowsMu sync.RWMutex

	// Thread and stack info
	threads   []ThreadInfo
	threadsMu sync.RWMutex
	stack     []StackEntry
	stackMu   sync.RWMutex

	// Evaluate request/response coordination
	evalMu      sync.Mutex
	evalOutputs []string
	evalDone    chan struct{}
	evalActive  bool
}

type varRef struct {
	frameID int
	scope   string // "local", "global"
}

// Window represents an editor/tracer window.
type Window struct {
	ID         int
	Name       string
	Text       []string
	Token      int
	IsTracer   bool
	CurrentRow int
	Stop       []int
	TID        int
}

// ThreadInfo represents APL thread information.
type ThreadInfo struct {
	TID         int
	Description string
	State       string
}

// StackEntry represents a stack frame.
type StackEntry struct {
	Description string
}

// LaunchConfig contains the configuration for a debug session.
type LaunchConfig struct {
	Host      string
	Port      int
	Workspace string
	Runtime   string
	Program   string
}

// New creates a new debug adapter.
func New(reader io.Reader, writer io.Writer) *Adapter {
	return &Adapter{
		reader:      bufio.NewReader(reader),
		writer:      writer,
		events:      make(chan *dap.Event, 100),
		breakpoints: make(map[string][]int),
		varRefs:     make(map[int]varRef),
		windows:     make(map[int]*Window),
	}
}

// Run starts the adapter main loop.
func (a *Adapter) Run() error {
	// Start event sender goroutine
	go a.sendEvents()

	for {
		req, err := a.readRequest()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		resp, err := a.handleRequest(req)
		if err != nil {
			a.sendErrorResponse(req, err.Error())
			continue
		}

		if resp != nil {
			if err := a.sendResponse(resp); err != nil {
				return err
			}
		}
	}
}

func (a *Adapter) readRequest() (*dap.Request, error) {
	var contentLength int
	for {
		line, err := a.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(a.reader, body); err != nil {
		return nil, err
	}

	var req dap.Request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	return &req, nil
}

func (a *Adapter) sendResponse(resp *dap.Response) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := a.writer.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := a.writer.Write(body); err != nil {
		return err
	}

	return nil
}

func (a *Adapter) sendEvent(event *dap.Event) error {
	a.seqMu.Lock()
	a.seq++
	event.Seq = a.seq
	a.seqMu.Unlock()

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := a.writer.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := a.writer.Write(body); err != nil {
		return err
	}

	return nil
}

func (a *Adapter) sendEvents() {
	for event := range a.events {
		a.sendEvent(event)
	}
}

func (a *Adapter) queueEvent(event *dap.Event) {
	event.Type = "event"
	select {
	case a.events <- event:
	default:
	}
}

func (a *Adapter) sendErrorResponse(req *dap.Request, message string) {
	a.seqMu.Lock()
	a.seq++
	seq := a.seq
	a.seqMu.Unlock()

	resp := &dap.Response{
		Message: dap.Message{
			Seq:  seq,
			Type: "response",
		},
		RequestSeq:   req.Seq,
		Success:      false,
		Command:      req.Command,
		ErrorMessage: message,
	}
	a.sendResponse(resp)
}

func (a *Adapter) makeResponse(req *dap.Request, body any) *dap.Response {
	a.seqMu.Lock()
	a.seq++
	seq := a.seq
	a.seqMu.Unlock()

	return &dap.Response{
		Message: dap.Message{
			Seq:  seq,
			Type: "response",
		},
		RequestSeq: req.Seq,
		Success:    true,
		Command:    req.Command,
		Body:       body,
	}
}

func (a *Adapter) handleRequest(req *dap.Request) (*dap.Response, error) {
	log.Printf("DAP request: %s", req.Command)
	switch req.Command {
	case "initialize":
		return a.handleInitialize(req)
	case "launch":
		return a.handleLaunch(req)
	case "attach":
		return a.handleAttach(req)
	case "configurationDone":
		return a.handleConfigurationDone(req)
	case "setBreakpoints":
		return a.handleSetBreakpoints(req)
	case "setFunctionBreakpoints":
		return a.handleSetFunctionBreakpoints(req)
	case "threads":
		return a.handleThreads(req)
	case "stackTrace":
		return a.handleStackTrace(req)
	case "scopes":
		return a.handleScopes(req)
	case "variables":
		return a.handleVariables(req)
	case "continue":
		return a.handleContinue(req)
	case "next":
		return a.handleNext(req)
	case "stepIn":
		return a.handleStepIn(req)
	case "stepOut":
		return a.handleStepOut(req)
	case "pause":
		return a.handlePause(req)
	case "evaluate":
		return a.handleEvaluate(req)
	case "disconnect":
		return a.handleDisconnect(req)
	case "terminate":
		return a.handleTerminate(req)
	default:
		return a.makeResponse(req, nil), nil
	}
}

func (a *Adapter) handleInitialize(req *dap.Request) (*dap.Response, error) {
	caps := dap.Capabilities{
		SupportsConfigurationDoneRequest: true,
		SupportsFunctionBreakpoints:      true,
		SupportsEvaluateForHovers:        true,
		SupportsTerminateRequest:         true,
	}

	a.initialized = true

	go func() {
		a.queueEvent(&dap.Event{Event: "initialized"})
	}()

	return a.makeResponse(req, caps), nil
}

func (a *Adapter) handleLaunch(req *dap.Request) (*dap.Response, error) {
	var args dap.LaunchRequestArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	a.config = LaunchConfig{
		Host:      args.Host,
		Port:      args.Port,
		Workspace: args.Workspace,
		Runtime:   args.Runtime,
		Program:   args.Program,
	}

	if a.config.Host == "" {
		a.config.Host = "127.0.0.1"
	}
	if a.config.Port == 0 {
		a.config.Port = 4502
	}

	if a.config.Runtime != "" {
		if err := a.launchInterpreter(); err != nil {
			return nil, fmt.Errorf("failed to launch interpreter: %w", err)
		}
	}

	if err := a.connectToInterpreter(); err != nil {
		return nil, err
	}

	return a.makeResponse(req, nil), nil
}

func (a *Adapter) handleAttach(req *dap.Request) (*dap.Response, error) {
	var args dap.AttachRequestArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	a.config.Host = args.Host
	a.config.Port = args.Port
	a.config.Program = args.Program

	if a.config.Host == "" {
		a.config.Host = "127.0.0.1"
	}
	if a.config.Port == 0 {
		a.config.Port = 4502
	}

	if err := a.connectToInterpreter(); err != nil {
		return nil, err
	}

	return a.makeResponse(req, nil), nil
}

func (a *Adapter) launchInterpreter() error {
	args := []string{"+s", "-q"}

	if a.config.Workspace != "" {
		args = append(args, a.config.Workspace)
	}

	a.process = exec.Command(a.config.Runtime, args...)
	a.process.Env = append(os.Environ(),
		fmt.Sprintf("RIDE_INIT=SERVE:*:%d", a.config.Port),
	)

	if err := a.process.Start(); err != nil {
		return err
	}

	return nil
}

func (a *Adapter) connectToInterpreter() error {
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	client, err := ride.Connect(addr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	a.ride = client
	a.connected = true

	// Start async message listener
	go a.listenForMessages()

	return nil
}

// listenForMessages reads messages from the interpreter and dispatches them.
func (a *Adapter) listenForMessages() {
	for {
		msg, _, err := a.ride.Recv()

		if err != nil {
			a.queueEvent(&dap.Event{
				Event: "output",
				Body: dap.OutputEventBody{
					Category: "stderr",
					Output:   fmt.Sprintf("Connection error: %v\n", err),
				},
			})
			a.queueEvent(&dap.Event{Event: "terminated"})
			return
		}

		if msg == nil {
			continue
		}

		a.handleRideMessage(msg)
	}
}

func (a *Adapter) handleRideMessage(msg *ride.Message) {
	switch msg.Command {
	case "AppendSessionOutput":
		// type 14 is input echo - skip it
		if t, ok := msg.Args["type"].(float64); ok && t == 14 {
			return
		}
		if result, ok := msg.Args["result"].(string); ok {
			// If we're collecting for an evaluate request, store output
			a.evalMu.Lock()
			if a.evalActive {
				a.evalOutputs = append(a.evalOutputs, result)
			} else {
				// Otherwise send as output event
				a.queueEvent(&dap.Event{
					Event: "output",
					Body: dap.OutputEventBody{
						Category: "stdout",
						Output:   result,
					},
				})
			}
			a.evalMu.Unlock()
		}

	case "SetPromptType":
		// Prompt type > 0 means interpreter is ready for input
		if t, ok := msg.Args["type"].(float64); ok && t > 0 {
			a.evalMu.Lock()
			if a.evalActive && a.evalDone != nil {
				close(a.evalDone)
				a.evalDone = nil
			}
			a.evalMu.Unlock()
		}

	case "OpenWindow", "UpdateWindow":
		a.handleWindow(msg.Args)

		// If this is a debugger window, we hit a breakpoint - signal eval to complete
		// debugger is 0/1 integer (float64 in JSON), not boolean
		isDebugger := false
		if d, ok := msg.Args["debugger"].(float64); ok && d != 0 {
			isDebugger = true
		} else if d, ok := msg.Args["debugger"].(bool); ok && d {
			isDebugger = true
		}
		if isDebugger {
			a.evalMu.Lock()
			if a.evalActive && a.evalDone != nil {
				close(a.evalDone)
				a.evalDone = nil
			}
			a.evalMu.Unlock()
		}

	case "CloseWindow":
		if win, ok := msg.Args["win"].(float64); ok {
			a.windowsMu.Lock()
			delete(a.windows, int(win))
			a.windowsMu.Unlock()
		}

	case "SetHighlightLine":
		if win, ok := msg.Args["win"].(float64); ok {
			a.activeWindowMu.Lock()
			a.activeWindow = int(win)
			a.activeWindowMu.Unlock()

			a.queueEvent(&dap.Event{
				Event: "stopped",
				Body: dap.StoppedEventBody{
					Reason:            "step",
					ThreadID:          0,
					AllThreadsStopped: true,
				},
			})
		}

	case "ReplyGetThreads":
		a.handleThreadsResponse(msg.Args)

	case "ReplyGetSIStack":
		a.handleStackResponse(msg.Args)
	}
}

func (a *Adapter) handleWindow(args map[string]any) {
	win := &Window{}

	if token, ok := args["token"].(float64); ok {
		win.ID = int(token)
		win.Token = int(token)
	}
	if name, ok := args["name"].(string); ok {
		win.Name = name
	}
	// debugger is 0/1 integer (float64 in JSON), not boolean
	if debugger, ok := args["debugger"].(float64); ok {
		win.IsTracer = debugger != 0
	} else if debugger, ok := args["debugger"].(bool); ok {
		win.IsTracer = debugger
	}
	if row, ok := args["currentRow"].(float64); ok {
		win.CurrentRow = int(row)
	}
	if tid, ok := args["tid"].(float64); ok {
		win.TID = int(tid)
	}

	a.windowsMu.Lock()
	a.windows[win.ID] = win
	a.windowsMu.Unlock()

	if win.IsTracer {
		a.activeWindowMu.Lock()
		a.activeWindow = win.ID
		a.activeWindowMu.Unlock()

		a.queueEvent(&dap.Event{
			Event: "stopped",
			Body: dap.StoppedEventBody{
				Reason:            "breakpoint",
				ThreadID:          0,
				AllThreadsStopped: true,
			},
		})
	}
}

func (a *Adapter) handleThreadsResponse(args map[string]any) {
	threads, ok := args["threads"].([]any)
	if !ok {
		return
	}

	a.threadsMu.Lock()
	defer a.threadsMu.Unlock()

	a.threads = make([]ThreadInfo, 0, len(threads))
	for _, t := range threads {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}

		info := ThreadInfo{}
		if tid, ok := tm["tid"].(float64); ok {
			info.TID = int(tid)
		}
		if desc, ok := tm["description"].(string); ok {
			info.Description = desc
		}
		if state, ok := tm["state"].(string); ok {
			info.State = state
		}

		a.threads = append(a.threads, info)
	}
}

func (a *Adapter) handleStackResponse(args map[string]any) {
	stack, ok := args["stack"].([]any)
	if !ok {
		return
	}

	a.stackMu.Lock()
	defer a.stackMu.Unlock()

	a.stack = make([]StackEntry, 0, len(stack))
	for _, s := range stack {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}

		entry := StackEntry{}
		if desc, ok := sm["description"].(string); ok {
			entry.Description = desc
		}
		a.stack = append(a.stack, entry)
	}

	log.Printf("SI stack: %d entries", len(a.stack))
	for i, e := range a.stack {
		log.Printf("  [%d] %s", i, e.Description)
	}
}

func (a *Adapter) handleConfigurationDone(req *dap.Request) (*dap.Response, error) {
	a.configured = true

	if a.config.Program != "" {
		prog := a.config.Program
		if !filepath.IsAbs(prog) && a.config.Workspace != "" {
			prog = filepath.Join(a.config.Workspace, prog)
		}
		var cmd string
		if strings.HasSuffix(strings.ToLower(prog), ".dws") {
			cmd = fmt.Sprintf(")LOAD %s\n", prog)
		} else {
			cmd = fmt.Sprintf("2⎕FIX'file://%s'\n", prog)
		}
		log.Printf("configurationDone: loading program: %s", cmd)
		a.sendMu.Lock()
		a.ride.Send("Execute", map[string]any{
			"text":  cmd,
			"trace": 0,
		})
		a.sendMu.Unlock()

		// Parse the source file to build function -> file location map
		if !strings.HasSuffix(strings.ToLower(prog), ".dws") {
			locs := parseSourceFile(prog)
			a.sourceMapMu.Lock()
			if a.sourceMap == nil {
				a.sourceMap = make(map[string]funcLocation)
			}
			for name, loc := range locs {
				a.sourceMap[name] = loc
			}
			a.sourceMapMu.Unlock()
		}
	}

	// Replay any breakpoints that were set before the program was loaded
	if len(a.pendingStops) > 0 {
		log.Printf("Replaying %d pending breakpoints", len(a.pendingStops))
		for _, ps := range a.pendingStops {
			// Convert file lines to function lines now that source map is populated
			funcLines := make([]int, len(ps.Lines))
			for i, l := range ps.Lines {
				funcLines[i] = a.fileLineToFuncLine(ps.FuncName, l)
			}
			a.sendStopCommand(ps.FuncName, funcLines)
		}
		a.pendingStops = nil
	}

	return a.makeResponse(req, nil), nil
}

func (a *Adapter) handleSetBreakpoints(req *dap.Request) (*dap.Response, error) {
	var args dap.SetBreakpointsArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	funcName := extractFunctionName(args.Source.Path)
	log.Printf("setBreakpoints: file=%s func=%s lines=%v", args.Source.Path, funcName, args.Breakpoints)

	// Store source path for this function (for stack frame mapping)
	a.breakpointsMu.Lock()
	if a.sourcePaths == nil {
		a.sourcePaths = make(map[string]string)
	}
	a.sourcePaths[funcName] = args.Source.Path
	a.breakpointsMu.Unlock()

	// Convert file line numbers to function-relative line numbers for ⎕STOP
	lines := make([]int, len(args.Breakpoints))
	for i, bp := range args.Breakpoints {
		lines[i] = a.fileLineToFuncLine(funcName, bp.Line)
	}

	// Use ⎕STOP to set breakpoints (works without open tracer window)
	// Format: lines ⎕STOP 'funcName'
	if len(lines) > 0 && funcName != "" {
		if !a.configured {
			// Queue for replay after program loads
			a.pendingStops = append(a.pendingStops, pendingStop{FuncName: funcName, Lines: lines})
			log.Printf("Queuing line breakpoints (pre-config): %s lines=%v", funcName, lines)
		} else {
			a.sendStopCommand(funcName, lines)
		}
	} else if funcName != "" && a.configured {
		// Clear breakpoints
		clearExpr := fmt.Sprintf("⍬ ⎕STOP '%s'", funcName)
		a.sendMu.Lock()
		a.ride.Send("Execute", map[string]any{
			"text":  clearExpr + "\n",
			"trace": 0,
		})
		a.sendMu.Unlock()
	}

	breakpoints := make([]dap.Breakpoint, len(args.Breakpoints))
	for i, bp := range args.Breakpoints {
		breakpoints[i] = dap.Breakpoint{
			ID:       i + 1,
			Verified: true,
			Line:     bp.Line,
			Source:   &args.Source,
		}
	}

	return a.makeResponse(req, dap.SetBreakpointsResponseBody{
		Breakpoints: breakpoints,
	}), nil
}

func (a *Adapter) handleSetFunctionBreakpoints(req *dap.Request) (*dap.Response, error) {
	var args dap.SetFunctionBreakpointsArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	log.Printf("setFunctionBreakpoints: %v", args.Breakpoints)

	breakpoints := make([]dap.Breakpoint, len(args.Breakpoints))
	for i, fb := range args.Breakpoints {
		if !a.configured {
			// Queue for replay after program loads
			a.pendingStops = append(a.pendingStops, pendingStop{FuncName: fb.Name})
			log.Printf("Queuing function breakpoint (pre-config): %s", fb.Name)
		} else {
			stopExpr := fmt.Sprintf("1 ⎕STOP '%s'", fb.Name)
			log.Printf("Setting function breakpoint: %s", stopExpr)
			a.sendMu.Lock()
			a.ride.Send("Execute", map[string]any{
				"text":  stopExpr + "\n",
				"trace": 0,
			})
			a.sendMu.Unlock()
		}

		breakpoints[i] = dap.Breakpoint{
			ID:       i + 1,
			Verified: true,
		}
	}

	return a.makeResponse(req, dap.SetBreakpointsResponseBody{
		Breakpoints: breakpoints,
	}), nil
}

func (a *Adapter) handleThreads(req *dap.Request) (*dap.Response, error) {
	a.sendMu.Lock()
	a.ride.Send("GetThreads", map[string]any{})
	a.sendMu.Unlock()

	a.threadsMu.RLock()
	threads := a.threads
	a.threadsMu.RUnlock()

	dapThreads := make([]dap.Thread, len(threads))
	for i, t := range threads {
		name := t.Description
		if name == "" {
			name = fmt.Sprintf("Thread %d", t.TID)
		}
		dapThreads[i] = dap.Thread{
			ID:   t.TID,
			Name: name,
		}
	}

	if len(dapThreads) == 0 {
		dapThreads = []dap.Thread{{ID: 0, Name: "Main Thread"}}
	}

	return a.makeResponse(req, dap.ThreadsResponseBody{
		Threads: dapThreads,
	}), nil
}

func (a *Adapter) handleStackTrace(req *dap.Request) (*dap.Response, error) {
	var args dap.StackTraceArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	a.sendMu.Lock()
	a.ride.Send("GetSIStack", map[string]any{})
	a.sendMu.Unlock()
	time.Sleep(50 * time.Millisecond)

	a.stackMu.RLock()
	stack := a.stack
	a.stackMu.RUnlock()

	log.Printf("stackTrace: SI stack has %d entries, building frames", len(stack))
	frames := make([]dap.StackFrame, len(stack))
	for i, entry := range stack {
		name, line := parseStackDescription(entry.Description)
		sourcePath := a.getSourcePath(name)
		fileLine := a.funcLineToFileLine(name, line)
		frames[i] = dap.StackFrame{
			ID:     i,
			Name:   name,
			Line:   fileLine,
			Column: 0,
			Source: &dap.Source{
				Name: name,
				Path: sourcePath,
			},
		}
	}

	if len(frames) == 0 {
		a.windowsMu.RLock()
		for _, w := range a.windows {
			if w.IsTracer {
				sourcePath := a.getSourcePath(w.Name)
				fileLine := a.funcLineToFileLine(w.Name, w.CurrentRow+1)
				frames = append(frames, dap.StackFrame{
					ID:     0,
					Name:   w.Name,
					Line:   fileLine,
					Column: 0,
					Source: &dap.Source{
						Name: w.Name,
						Path: sourcePath,
					},
				})
			}
		}
		a.windowsMu.RUnlock()
	}

	return a.makeResponse(req, dap.StackTraceResponseBody{
		StackFrames: frames,
		TotalFrames: len(frames),
	}), nil
}

func (a *Adapter) handleScopes(req *dap.Request) (*dap.Response, error) {
	var args dap.ScopesArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	a.varRefsMu.Lock()
	a.varRefCounter++
	localRef := a.varRefCounter
	a.varRefs[localRef] = varRef{frameID: args.FrameID, scope: "local"}
	a.varRefsMu.Unlock()

	scopes := []dap.Scope{
		{
			Name:               "Local",
			VariablesReference: localRef,
			Expensive:          false,
		},
	}

	return a.makeResponse(req, dap.ScopesResponseBody{
		Scopes: scopes,
	}), nil
}

func (a *Adapter) handleVariables(req *dap.Request) (*dap.Response, error) {
	var args dap.VariablesArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	a.varRefsMu.Lock()
	ref, ok := a.varRefs[args.VariablesReference]
	a.varRefsMu.Unlock()

	variables := []dap.Variable{}
	if !ok || ref.scope != "local" {
		return a.makeResponse(req, dap.VariablesResponseBody{Variables: variables}), nil
	}

	// Get the current function name from the stack
	funcName := ""
	a.stackMu.RLock()
	if len(a.stack) > ref.frameID {
		name, _ := parseStackDescription(a.stack[ref.frameID].Description)
		funcName = name
	}
	a.stackMu.RUnlock()

	// Also check tracer windows
	if funcName == "" {
		a.windowsMu.RLock()
		for _, w := range a.windows {
			if w.IsTracer {
				funcName = w.Name
				break
			}
		}
		a.windowsMu.RUnlock()
	}

	if funcName == "" {
		return a.makeResponse(req, dap.VariablesResponseBody{Variables: variables}), nil
	}

	// Get local variable names from the function header
	varNames := a.getLocalVarNames(funcName)
	log.Printf("variables: func=%s vars=%v", funcName, varNames)

	// Evaluate each variable
	for _, name := range varNames {
		value := a.evalExpression(name)
		variables = append(variables, dap.Variable{
			Name:  name,
			Value: value,
		})
	}

	return a.makeResponse(req, dap.VariablesResponseBody{Variables: variables}), nil
}

func (a *Adapter) handleContinue(req *dap.Request) (*dap.Response, error) {
	a.activeWindowMu.RLock()
	win := a.activeWindow
	a.activeWindowMu.RUnlock()

	a.sendMu.Lock()
	if win > 0 {
		a.ride.Send("Continue", map[string]any{"win": win})
	} else {
		a.ride.Send("RestartThreads", map[string]any{})
	}
	a.sendMu.Unlock()

	return a.makeResponse(req, dap.ContinueResponseBody{
		AllThreadsContinued: true,
	}), nil
}

func (a *Adapter) handleNext(req *dap.Request) (*dap.Response, error) {
	a.activeWindowMu.RLock()
	win := a.activeWindow
	a.activeWindowMu.RUnlock()

	if win > 0 {
		a.sendMu.Lock()
		a.ride.Send("RunCurrentLine", map[string]any{"win": win})
		a.sendMu.Unlock()
	}

	return a.makeResponse(req, nil), nil
}

func (a *Adapter) handleStepIn(req *dap.Request) (*dap.Response, error) {
	a.activeWindowMu.RLock()
	win := a.activeWindow
	a.activeWindowMu.RUnlock()

	if win > 0 {
		a.sendMu.Lock()
		a.ride.Send("StepInto", map[string]any{"win": win})
		a.sendMu.Unlock()
	}

	return a.makeResponse(req, nil), nil
}

func (a *Adapter) handleStepOut(req *dap.Request) (*dap.Response, error) {
	a.activeWindowMu.RLock()
	win := a.activeWindow
	a.activeWindowMu.RUnlock()

	if win > 0 {
		a.sendMu.Lock()
		a.ride.Send("ContinueTrace", map[string]any{"win": win})
		a.sendMu.Unlock()
	}

	return a.makeResponse(req, nil), nil
}

func (a *Adapter) handlePause(req *dap.Request) (*dap.Response, error) {
	a.sendMu.Lock()
	a.ride.Send("WeakInterrupt", map[string]any{})
	a.sendMu.Unlock()
	return a.makeResponse(req, nil), nil
}

func (a *Adapter) handleEvaluate(req *dap.Request) (*dap.Response, error) {
	var args dap.EvaluateArguments
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, err
	}

	// Setup to collect output
	a.evalMu.Lock()
	a.evalOutputs = nil
	a.evalDone = make(chan struct{})
	a.evalActive = true
	a.evalMu.Unlock()

	// Send Execute command (must add \n for interpreter to process)
	a.sendMu.Lock()
	err := a.ride.Send("Execute", map[string]any{
		"text":  args.Expression + "\n",
		"trace": 0,
	})
	a.sendMu.Unlock()

	if err != nil {
		a.evalMu.Lock()
		a.evalActive = false
		a.evalMu.Unlock()
		return nil, err
	}

	// Wait for SetPromptType signal (interpreter ready) with timeout
	select {
	case <-a.evalDone:
		// Success
	case <-time.After(30 * time.Second):
		a.evalMu.Lock()
		a.evalActive = false
		a.evalMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for interpreter response")
	}

	// Collect results
	a.evalMu.Lock()
	outputs := a.evalOutputs
	a.evalActive = false
	a.evalMu.Unlock()

	// Chunks arrive with their own newlines; concatenate and trim the final one.
	result := strings.TrimSuffix(strings.Join(outputs, ""), "\n")
	return a.makeResponse(req, dap.EvaluateResponseBody{
		Result: result,
	}), nil
}

func (a *Adapter) handleDisconnect(req *dap.Request) (*dap.Response, error) {
	if a.ride != nil {
		// Cut back and close any open tracer windows to clear suspensions
		a.windowsMu.RLock()
		for id, w := range a.windows {
			if w.IsTracer {
				a.ride.Send("Cutback", map[string]any{"win": id})
				a.ride.Send("CloseWindow", map[string]any{"win": id})
			}
		}
		a.windowsMu.RUnlock()
		time.Sleep(50 * time.Millisecond)
		// Do NOT send a RIDE Disconnect message: it ends the interpreter's
		// session (it stops evaluating). A DAP disconnect in attach mode must
		// leave Dyalog running so VSCode can detach/restart and reconnect
		// later. Just close our end of the connection.
		a.ride.Close()
	}

	if a.process != nil {
		a.process.Process.Kill()
	}

	return a.makeResponse(req, nil), nil
}

func (a *Adapter) handleTerminate(req *dap.Request) (*dap.Response, error) {
	if a.ride != nil {
		a.ride.Send("Exit", map[string]any{"code": 0})
		a.ride.Close()
	}

	if a.process != nil {
		a.process.Process.Kill()
	}

	a.queueEvent(&dap.Event{Event: "terminated"})

	return a.makeResponse(req, nil), nil
}

// sendStopCommand sends a ⎕STOP command to the interpreter.
func (a *Adapter) sendStopCommand(funcName string, lines []int) {
	var stopExpr string
	if len(lines) == 0 {
		stopExpr = fmt.Sprintf("1 ⎕STOP '%s'", funcName)
	} else {
		lineStrs := make([]string, len(lines))
		for i, l := range lines {
			lineStrs[i] = fmt.Sprintf("%d", l)
		}
		stopExpr = fmt.Sprintf("%s ⎕STOP '%s'", strings.Join(lineStrs, " "), funcName)
	}
	log.Printf("Setting breakpoints: %s", stopExpr)
	a.sendMu.Lock()
	a.ride.Send("Execute", map[string]any{
		"text":  stopExpr + "\n",
		"trace": 0,
	})
	a.sendMu.Unlock()
}

// pendingStop records a ⎕STOP to replay after program loads.
type pendingStop struct {
	FuncName string
	Lines    []int // empty means line 1 (function breakpoint)
}

// funcLocation maps a function to its position in a source file.
type funcLocation struct {
	FilePath   string
	HeaderLine int // 1-indexed line number of the function header in the file
}

// parseSourceFile reads an APL source file and builds a map of function name to file location.
// It detects tradfn headers of the form "r←name args" or "name args" at the start of a block.
func parseSourceFile(filePath string) map[string]funcLocation {
	result := make(map[string]funcLocation)

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("parseSourceFile: failed to read %s: %v", filePath, err)
		return result
	}

	lines := strings.Split(string(data), "\n")
	tradfnHeader := regexp.MustCompile(`^(?:\w+\s*←\s*)?(\w+)\s`)

	inFunc := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			inFunc = false
			continue
		}
		if !inFunc {
			if m := tradfnHeader.FindStringSubmatch(trimmed); m != nil {
				funcName := m[1]
				result[funcName] = funcLocation{
					FilePath:   filePath,
					HeaderLine: i + 1, // 1-indexed
				}
				inFunc = true
			}
		}
	}

	log.Printf("parseSourceFile: %s -> %v", filePath, result)
	return result
}

// Helper functions

func extractFunctionName(path string) string {
	if strings.HasPrefix(path, "apl://") {
		return strings.TrimPrefix(path, "apl://")
	}
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	if idx := strings.LastIndex(name, "."); idx > 0 {
		name = name[:idx]
	}
	return name
}

// getSourcePath returns the file path for a function name, or a virtual path if unknown
func (a *Adapter) getSourcePath(funcName string) string {
	a.sourceMapMu.RLock()
	if loc, ok := a.sourceMap[funcName]; ok {
		a.sourceMapMu.RUnlock()
		return loc.FilePath
	}
	a.sourceMapMu.RUnlock()

	a.breakpointsMu.RLock()
	defer a.breakpointsMu.RUnlock()
	if path, ok := a.sourcePaths[funcName]; ok {
		return path
	}
	return fmt.Sprintf("apl://%s", funcName)
}

// funcLineToFileLine converts a function-relative line number to a file line number.
// APL function line 0 = header, line 1 = first body line.
// Returns the original line if no mapping exists.
func (a *Adapter) funcLineToFileLine(funcName string, funcLine int) int {
	a.sourceMapMu.RLock()
	defer a.sourceMapMu.RUnlock()
	if loc, ok := a.sourceMap[funcName]; ok {
		return loc.HeaderLine + funcLine
	}
	return funcLine
}

// getLocalVarNames parses the tradfn header to extract local variable names.
// Header format: "r←name args ; locals" or "name args ; locals"
func (a *Adapter) getLocalVarNames(funcName string) []string {
	a.sourceMapMu.RLock()
	loc, ok := a.sourceMap[funcName]
	a.sourceMapMu.RUnlock()
	if !ok {
		return nil
	}

	data, err := os.ReadFile(loc.FilePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	if loc.HeaderLine < 1 || loc.HeaderLine > len(lines) {
		return nil
	}
	header := strings.TrimSpace(lines[loc.HeaderLine-1])

	var names []string

	// Extract result variable: "r←..."
	if idx := strings.Index(header, "←"); idx > 0 {
		names = append(names, strings.TrimSpace(header[:idx]))
		header = header[idx+len("←"):]
	}

	// Split off locals after ";"
	locals := ""
	if idx := strings.Index(header, ";"); idx >= 0 {
		locals = header[idx+1:]
		header = header[:idx]
	}

	// Skip function name, get args
	fields := strings.Fields(header)
	if len(fields) > 1 {
		names = append(names, fields[1:]...)
	}

	// Add local variables
	if locals != "" {
		for _, l := range strings.Split(locals, ";") {
			l = strings.TrimSpace(l)
			if l != "" {
				names = append(names, l)
			}
		}
	}

	return names
}

// evalExpression evaluates an APL expression and returns the result string.
func (a *Adapter) evalExpression(expr string) string {
	// Use ⎕NC to check if the name exists first, then evaluate with error trapping
	safeExpr := fmt.Sprintf(":If 0<⎕NC'%s' ⋄ %s ⋄ :Else ⋄ '(undefined)' ⋄ :EndIf", expr, expr)

	a.evalMu.Lock()
	a.evalOutputs = nil
	a.evalDone = make(chan struct{})
	a.evalActive = true
	a.evalMu.Unlock()

	log.Printf("evalExpression: %s", safeExpr)

	a.sendMu.Lock()
	a.ride.Send("Execute", map[string]any{
		"text":  safeExpr + "\n",
		"trace": 0,
	})
	a.sendMu.Unlock()

	select {
	case <-a.evalDone:
	case <-time.After(3 * time.Second):
		a.evalMu.Lock()
		a.evalActive = false
		a.evalMu.Unlock()
		log.Printf("evalExpression: timeout for %s", expr)
		return "..."
	}

	a.evalMu.Lock()
	outputs := a.evalOutputs
	a.evalActive = false
	a.evalMu.Unlock()

	log.Printf("evalExpression: %s -> %v", expr, outputs)
	return strings.TrimSuffix(strings.Join(outputs, ""), "\n")
}

// fileLineToFuncLine converts a file line number to a function-relative line number.
func (a *Adapter) fileLineToFuncLine(funcName string, fileLine int) int {
	a.sourceMapMu.RLock()
	defer a.sourceMapMu.RUnlock()
	if loc, ok := a.sourceMap[funcName]; ok {
		return fileLine - loc.HeaderLine
	}
	return fileLine
}

func parseStackDescription(desc string) (string, int) {
	re := regexp.MustCompile(`([^[]+)\[(\d+)\]`)
	matches := re.FindStringSubmatch(desc)
	if len(matches) >= 3 {
		name := matches[1]
		// Strip namespace prefix (e.g. "#.foo" -> "foo")
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}
		line, _ := strconv.Atoi(matches[2])
		return name, line
	}
	return desc, 0
}
