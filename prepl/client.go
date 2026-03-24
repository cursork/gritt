// Package prepl implements a client for the APL prepl (programmable REPL) server.
//
// The prepl server runs inside Dyalog APL and accepts expressions over TCP,
// returning tagged APLAN namespace responses:
//
//	(tag: 'ret' ⋄ val: 1 2 3)
//	(tag: 'err' ⋄ en: 11 ⋄ message: 'DOMAIN ERROR' ⋄ dm: (...))
package prepl

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cursork/gritt/codec"
)

// Client connects to an APL prepl server.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

// Response is a parsed response from the prepl server.
type Response struct {
	Tag string // "ret" or "err"
	Val any    // Parsed APLAN value (for "ret"), nil for void
	Raw string // APLAN string of the value (for "ret")
	Err *Error // Error details (for "err")
}

// Error holds structured error information from ⎕DMX.
type Error struct {
	Message string
	EN      int
	DM      []string
}

func (e *Error) Error() string { return e.Message }

// Connect establishes a TCP connection to a prepl server.
func Connect(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

// Eval sends an expression to the prepl server and returns the response.
func (c *Client) Eval(expr string) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := fmt.Fprintf(c.conn, "%s\n", expr); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("recv: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")

	parsed, err := codec.APLAN(line)
	if err != nil {
		return nil, fmt.Errorf("parse APLAN response: %w: %q", err, line)
	}

	ns, ok := parsed.(*codec.Namespace)
	if !ok {
		return nil, fmt.Errorf("expected namespace response, got %T", parsed)
	}

	tagVal, ok := ns.Values["tag"]
	if !ok {
		return nil, fmt.Errorf("response missing tag field")
	}
	tag, ok := tagVal.(string)
	if !ok {
		return nil, fmt.Errorf("tag field is not a string: %T", tagVal)
	}

	switch tag {
	case "ret":
		resp := &Response{Tag: "ret"}
		if val, ok := ns.Values["val"]; ok {
			resp.Val = val
			resp.Raw = codec.Serialize(val, codec.SerializeOptions{UseDiamond: true})
		}
		return resp, nil

	case "err":
		resp := &Response{Tag: "err", Err: &Error{}}
		if v, ok := ns.Values["message"].(string); ok {
			resp.Err.Message = v
		}
		if v, ok := ns.Values["en"].(int); ok {
			resp.Err.EN = v
		}
		if v, ok := ns.Values["dm"]; ok {
			resp.Err.DM = toStringSlice(v)
		}
		return resp, nil

	default:
		return nil, fmt.Errorf("unknown tag: %q", tag)
	}
}

// EvalRaw sends an expression and returns the raw APLAN response line.
func (c *Client) EvalRaw(expr string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := fmt.Fprintf(c.conn, "%s\n", expr); err != nil {
		return "", fmt.Errorf("send: %w", err)
	}

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("recv: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// Close closes the connection to the prepl server.
func (c *Client) Close() error {
	return c.conn.Close()
}

// toStringSlice converts an APLAN vector value to []string.
func toStringSlice(v any) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, elem := range val {
			if s, ok := elem.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
