// Package dap implements the Debug Adapter Protocol types and message handling.
package dap

import (
	"encoding/json"
)

// Message is the base type for all DAP messages.
type Message struct {
	Seq  int    `json:"seq"`
	Type string `json:"type"` // "request", "response", "event"
}

// Request is a DAP request message.
type Request struct {
	Message
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// Response is a DAP response message.
type Response struct {
	Message
	RequestSeq   int    `json:"request_seq"`
	Success      bool   `json:"success"`
	Command      string `json:"command"`
	ErrorMessage string `json:"message,omitempty"`
	Body         any    `json:"body,omitempty"`
}

// Event is a DAP event message.
type Event struct {
	Message
	Event string `json:"event"`
	Body  any    `json:"body,omitempty"`
}

// Capabilities describes the debug adapter's capabilities.
type Capabilities struct {
	SupportsConfigurationDoneRequest      bool `json:"supportsConfigurationDoneRequest,omitempty"`
	SupportsFunctionBreakpoints           bool `json:"supportsFunctionBreakpoints,omitempty"`
	SupportsConditionalBreakpoints        bool `json:"supportsConditionalBreakpoints,omitempty"`
	SupportsEvaluateForHovers             bool `json:"supportsEvaluateForHovers,omitempty"`
	SupportsSetVariable                   bool `json:"supportsSetVariable,omitempty"`
	SupportsStepBack                      bool `json:"supportsStepBack,omitempty"`
	SupportsRestartFrame                  bool `json:"supportsRestartFrame,omitempty"`
	SupportsTerminateRequest              bool `json:"supportsTerminateRequest,omitempty"`
	SupportsSingleThreadExecutionRequests bool `json:"supportsSingleThreadExecutionRequests,omitempty"`
}

// InitializeRequestArguments contains the arguments for the initialize request.
type InitializeRequestArguments struct {
	ClientID                     string `json:"clientID,omitempty"`
	ClientName                   string `json:"clientName,omitempty"`
	AdapterID                    string `json:"adapterID"`
	Locale                       string `json:"locale,omitempty"`
	LinesStartAt1                bool   `json:"linesStartAt1,omitempty"`
	ColumnsStartAt1              bool   `json:"columnsStartAt1,omitempty"`
	PathFormat                   string `json:"pathFormat,omitempty"`
	SupportsVariableType         bool   `json:"supportsVariableType,omitempty"`
	SupportsVariablePaging       bool   `json:"supportsVariablePaging,omitempty"`
	SupportsRunInTerminalRequest bool   `json:"supportsRunInTerminalRequest,omitempty"`
}

// LaunchRequestArguments contains the arguments for launch request.
type LaunchRequestArguments struct {
	NoDebug     bool   `json:"noDebug,omitempty"`
	Program     string `json:"program,omitempty"`
	StopOnEntry bool   `json:"stopOnEntry,omitempty"`
	// APL-specific
	Workspace string `json:"workspace,omitempty"` // Path to .dws file
	Host      string `json:"host,omitempty"`      // Interpreter host (default: 127.0.0.1)
	Port      int    `json:"port,omitempty"`      // RIDE port (default: 4502)
	Runtime   string `json:"runtime,omitempty"`   // Path to dyalog executable
}

// AttachRequestArguments contains the arguments for attach request.
type AttachRequestArguments struct {
	Host    string `json:"host,omitempty"`    // Interpreter host
	Port    int    `json:"port,omitempty"`    // RIDE port
	Program string `json:"program,omitempty"` // APL source file to load on connect
}

// SetBreakpointsArguments contains the arguments for setBreakpoints request.
type SetBreakpointsArguments struct {
	Source      Source             `json:"source"`
	Breakpoints []SourceBreakpoint `json:"breakpoints,omitempty"`
}

// Source describes a source file.
type Source struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

// SourceBreakpoint describes a breakpoint in source code.
type SourceBreakpoint struct {
	Line      int    `json:"line"`
	Column    int    `json:"column,omitempty"`
	Condition string `json:"condition,omitempty"`
}

// Breakpoint describes a verified breakpoint.
type Breakpoint struct {
	ID       int     `json:"id,omitempty"`
	Verified bool    `json:"verified"`
	Line     int     `json:"line,omitempty"`
	Source   *Source `json:"source,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// SetBreakpointsResponseBody is the response body for setBreakpoints.
type SetBreakpointsResponseBody struct {
	Breakpoints []Breakpoint `json:"breakpoints"`
}

// FunctionBreakpoint describes a function breakpoint.
type FunctionBreakpoint struct {
	Name      string `json:"name"`
	Condition string `json:"condition,omitempty"`
}

// SetFunctionBreakpointsArguments contains the arguments for setFunctionBreakpoints request.
type SetFunctionBreakpointsArguments struct {
	Breakpoints []FunctionBreakpoint `json:"breakpoints"`
}

// Thread describes a thread.
type Thread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ThreadsResponseBody is the response body for threads request.
type ThreadsResponseBody struct {
	Threads []Thread `json:"threads"`
}

// StackTraceArguments contains the arguments for stackTrace request.
type StackTraceArguments struct {
	ThreadID   int `json:"threadId"`
	StartFrame int `json:"startFrame,omitempty"`
	Levels     int `json:"levels,omitempty"`
}

// StackFrame describes a stack frame.
type StackFrame struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Source    *Source `json:"source,omitempty"`
	Line      int     `json:"line"`
	Column    int     `json:"column"`
	EndLine   int     `json:"endLine,omitempty"`
	EndColumn int     `json:"endColumn,omitempty"`
}

// StackTraceResponseBody is the response body for stackTrace.
type StackTraceResponseBody struct {
	StackFrames []StackFrame `json:"stackFrames"`
	TotalFrames int          `json:"totalFrames,omitempty"`
}

// ScopesArguments contains the arguments for scopes request.
type ScopesArguments struct {
	FrameID int `json:"frameId"`
}

// Scope describes a scope.
type Scope struct {
	Name               string `json:"name"`
	VariablesReference int    `json:"variablesReference"`
	Expensive          bool   `json:"expensive,omitempty"`
}

// ScopesResponseBody is the response body for scopes.
type ScopesResponseBody struct {
	Scopes []Scope `json:"scopes"`
}

// VariablesArguments contains the arguments for variables request.
type VariablesArguments struct {
	VariablesReference int `json:"variablesReference"`
}

// Variable describes a variable.
type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
}

// VariablesResponseBody is the response body for variables.
type VariablesResponseBody struct {
	Variables []Variable `json:"variables"`
}

// ContinueArguments contains the arguments for continue request.
type ContinueArguments struct {
	ThreadID     int  `json:"threadId"`
	SingleThread bool `json:"singleThread,omitempty"`
}

// ContinueResponseBody is the response body for continue.
type ContinueResponseBody struct {
	AllThreadsContinued bool `json:"allThreadsContinued,omitempty"`
}

// NextArguments contains the arguments for next (step over) request.
type NextArguments struct {
	ThreadID     int    `json:"threadId"`
	Granularity  string `json:"granularity,omitempty"`
	SingleThread bool   `json:"singleThread,omitempty"`
}

// StepInArguments contains the arguments for stepIn request.
type StepInArguments struct {
	ThreadID     int    `json:"threadId"`
	TargetID     int    `json:"targetId,omitempty"`
	Granularity  string `json:"granularity,omitempty"`
	SingleThread bool   `json:"singleThread,omitempty"`
}

// StepOutArguments contains the arguments for stepOut request.
type StepOutArguments struct {
	ThreadID     int    `json:"threadId"`
	Granularity  string `json:"granularity,omitempty"`
	SingleThread bool   `json:"singleThread,omitempty"`
}

// EvaluateArguments contains the arguments for evaluate request.
type EvaluateArguments struct {
	Expression string `json:"expression"`
	FrameID    int    `json:"frameId,omitempty"`
	Context    string `json:"context,omitempty"` // "watch", "repl", "hover"
}

// EvaluateResponseBody is the response body for evaluate.
type EvaluateResponseBody struct {
	Result             string `json:"result"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference,omitempty"`
}

// StoppedEventBody is the body for stopped events.
type StoppedEventBody struct {
	Reason            string `json:"reason"` // "step", "breakpoint", "exception", "pause", "entry"
	ThreadID          int    `json:"threadId,omitempty"`
	AllThreadsStopped bool   `json:"allThreadsStopped,omitempty"`
	Text              string `json:"text,omitempty"`
}

// OutputEventBody is the body for output events.
type OutputEventBody struct {
	Category string  `json:"category,omitempty"` // "console", "stdout", "stderr", "telemetry"
	Output   string  `json:"output"`
	Source   *Source `json:"source,omitempty"`
	Line     int     `json:"line,omitempty"`
}

// TerminatedEventBody is the body for terminated events.
type TerminatedEventBody struct {
	Restart bool `json:"restart,omitempty"`
}
