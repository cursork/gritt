// Package mcp implements a Model Context Protocol server for Dyalog APL.
// JSON-RPC 2.0 over stdio. The server starts with no APL session;
// use the launch or connect tools to start one.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/cursork/gritt/codec"
	"github.com/cursork/gritt/session"
)

// Server implements the Model Context Protocol (JSON-RPC 2.0 over stdio).
type Server struct {
	sess *session.Session
	mu   sync.Mutex
}

// NewServer creates an MCP server with no active session.
func NewServer() *Server {
	return &Server{}
}

// JSON-RPC 2.0 message types.

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Serve reads JSON-RPC 2.0 messages from r and writes responses to w.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(w, rpcResponse{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}

		if req.ID == nil {
			continue // notifications get no response
		}

		resp := s.handle(ctx, req)
		writeResponse(w, resp)
	}

	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":   map[string]any{"tools": map[string]any{}},
				"serverInfo":     map[string]any{"name": "aplmcp", "version": "0.1.0"},
			},
		}
	case "ping":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": tools()}}
	case "tools/call":
		return s.handleToolCall(ctx, req)
	default:
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleToolCall(ctx context.Context, req rpcRequest) rpcResponse {
	if len(req.Params) == 0 {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "missing params"}}
	}

	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := s.callTool(ctx, params.Name, params.Arguments)
	return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) callTool(ctx context.Context, name string, args json.RawMessage) toolResult {
	switch name {
	case "launch":
		return s.toolLaunch(ctx, args)
	case "connect":
		return s.toolConnect(ctx, args)
	case "disconnect":
		return s.toolDisconnect()
	case "eval":
		return s.toolEval(ctx, args)
	case "batch":
		return s.toolBatch(ctx, args)
	case "link":
		return s.toolLink(ctx, args)
	case "names":
		return s.toolNames(ctx, args)
	case "get":
		return s.toolGet(ctx, args)
	case "fix":
		return s.toolFix(ctx, args)
	case "alive":
		return s.toolAlive()
	default:
		return errResult(fmt.Sprintf("unknown tool: %s", name))
	}
}

func noSession() toolResult {
	return errResult("no interpreter connected — use the launch or connect tool first")
}

func (s *Server) toolLaunch(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess != nil {
		return errResult("already connected — disconnect first")
	}
	var p struct {
		Version string `json:"version"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &p)
	}
	sess, err := session.Launch(ctx, session.LaunchOptions{Version: p.Version})
	if err != nil {
		return errResult("launch failed: " + err.Error())
	}
	s.sess = sess
	return textResult("launched")
}

func (s *Server) toolConnect(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess != nil {
		return errResult("already connected — disconnect first")
	}
	var p struct {
		Addr string `json:"addr"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &p)
	}
	sess, err := session.Connect(ctx, session.ConnectOptions{Addr: p.Addr})
	if err != nil {
		return errResult("connect failed: " + err.Error())
	}
	s.sess = sess
	return textResult("connected")
}

func (s *Server) toolDisconnect() toolResult {
	if s.sess == nil {
		return errResult("not connected")
	}
	s.sess.Close()
	s.sess = nil
	return textResult("disconnected")
}

func (s *Server) toolEval(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess == nil {
		return noSession()
	}
	var p struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}

	result, err := s.sess.Eval(ctx, p.Code)
	if err != nil {
		return sessionErr(err)
	}

	// Try APLAN parse for structured result
	if parsed, parseErr := codec.APLAN(result); parseErr == nil {
		return jsonResult(map[string]any{"format": "aplan", "result": codec.ToJSON(parsed)})
	}

	return jsonResult(map[string]any{"format": "display", "result": result})
}

func (s *Server) toolBatch(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess == nil {
		return noSession()
	}
	var p struct {
		Expressions []string `json:"expressions"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}

	results, err := s.sess.Batch(ctx, p.Expressions)
	if err != nil {
		if errors.Is(err, session.ErrSessionRestarted) {
			return sessionErr(err)
		}
		return errResultJSON(map[string]any{"results": results, "error": err.Error()})
	}
	return jsonResult(map[string]any{"results": results})
}

func (s *Server) toolLink(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess == nil {
		return noSession()
	}
	var p struct {
		Directory string `json:"directory"`
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}

	var err error
	if p.Namespace != "" {
		err = s.sess.Link(ctx, p.Directory, session.NS(p.Namespace))
	} else {
		err = s.sess.Link(ctx, p.Directory)
	}
	if err != nil {
		return sessionErr(err)
	}
	return textResult("linked")
}

func (s *Server) toolNames(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess == nil {
		return noSession()
	}
	var p struct {
		Namespace string `json:"namespace"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &p)
	}

	var names []string
	var err error
	if p.Namespace != "" {
		names, err = s.sess.Names(ctx, session.NS(p.Namespace))
	} else {
		names, err = s.sess.Names(ctx)
	}
	if err != nil {
		return sessionErr(err)
	}
	return jsonResult(names)
}

func (s *Server) toolGet(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess == nil {
		return noSession()
	}
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}

	result, err := s.sess.Get(ctx, p.Name)
	if err != nil {
		return sessionErr(err)
	}
	return textResult(result)
}

func (s *Server) toolFix(ctx context.Context, args json.RawMessage) toolResult {
	if s.sess == nil {
		return noSession()
	}
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errResult("invalid arguments: " + err.Error())
	}

	if err := s.sess.Fix(ctx, p.Path); err != nil {
		return sessionErr(err)
	}
	return textResult("fixed")
}

func (s *Server) toolAlive() toolResult {
	if s.sess == nil {
		return jsonResult(false)
	}
	return jsonResult(s.sess.Alive())
}

// --- Result helpers ---

func jsonResult(v any) toolResult {
	data, _ := json.Marshal(v)
	return toolResult{Content: []toolContent{{Type: "text", Text: string(data)}}}
}

func textResult(text string) toolResult {
	return toolResult{Content: []toolContent{{Type: "text", Text: text}}}
}

func errResult(msg string) toolResult {
	return toolResult{Content: []toolContent{{Type: "text", Text: msg}}, IsError: true}
}

func sessionErr(err error) toolResult {
	if errors.Is(err, session.ErrSessionRestarted) {
		return errResult("interpreter crashed and was restarted — all workspace state has been lost")
	}
	return errResult(err.Error())
}

func errResultJSON(v any) toolResult {
	data, _ := json.Marshal(v)
	return toolResult{Content: []toolContent{{Type: "text", Text: string(data)}}, IsError: true}
}

func writeResponse(w io.Writer, resp rpcResponse) {
	data, _ := json.Marshal(resp)
	w.Write(data)
	w.Write([]byte("\n"))
}

// --- Tool definitions ---

func tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "launch",
			"description": "Launch a new Dyalog APL interpreter. Spawns a Dyalog process and connects via RIDE. Must disconnect first if already connected.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"version": map[string]any{"type": "string", "description": "Dyalog version (e.g. \"20.0\") or path to binary. Default: auto-discover."},
				},
			},
		},
		{
			"name":        "connect",
			"description": "Connect to an already-running Dyalog APL interpreter in SERVE mode (RIDE_INIT=SERVE:*:port). Must disconnect first if already connected.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"addr": map[string]any{"type": "string", "description": "host:port (default: localhost:4502)"},
				},
			},
		},
		{
			"name":        "disconnect",
			"description": "Disconnect from the current APL interpreter. All workspace state is lost.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "eval",
			"description": "Execute a Dyalog APL expression and return the result. The interpreter persists between calls. Use ← for assignment.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{"code": map[string]any{"type": "string", "description": "APL expression to evaluate"}},
				"required": []string{"code"},
			},
			"annotations": map[string]any{"openWorldHint": true},
		},
		{
			"name":        "batch",
			"description": "Execute multiple APL expressions sequentially. Stops on first error and returns partial results.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{"expressions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "APL expressions to execute in order"}},
				"required": []string{"expressions"},
			},
			"annotations": map[string]any{"openWorldHint": true},
		},
		{
			"name":        "link",
			"description": "Link a filesystem directory into the APL workspace. APL source files become available as functions/operators/namespaces.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{
					"directory": map[string]any{"type": "string", "description": "Absolute path to directory"},
					"namespace": map[string]any{"type": "string", "description": "Target namespace (default: root \"#\")"},
				},
				"required": []string{"directory"},
			},
			"annotations": map[string]any{"readOnlyHint": false},
		},
		{
			"name":        "names",
			"description": "List all defined names in a namespace.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"namespace": map[string]any{"type": "string", "description": "Namespace to list (default: root \"#\")"}},
			},
			"annotations": map[string]any{"readOnlyHint": true},
		},
		{
			"name":        "get",
			"description": "Get the display form of a variable's value.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{"name": map[string]any{"type": "string", "description": "Variable name"}},
				"required": []string{"name"},
			},
			"annotations": map[string]any{"readOnlyHint": true},
		},
		{
			"name":        "fix",
			"description": "Load an APL source file into the workspace via ⎕FIX.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{"path": map[string]any{"type": "string", "description": "Absolute path to APL source file"}},
				"required": []string{"path"},
			},
			"annotations": map[string]any{"readOnlyHint": false},
		},
		{
			"name":        "alive",
			"description": "Check if the APL interpreter is running and responsive.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
			"annotations": map[string]any{"readOnlyHint": true},
		},
	}
}
