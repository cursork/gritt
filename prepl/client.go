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
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cursork/gritt/codec"
)

// UUIDv7 generates a v7 UUID (time-ordered, random).
func UUIDv7() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint64(b[:8], ms<<16) // 48-bit timestamp in top bits
	rand.Read(b[6:])                           // random fill rest
	b[6] = (b[6] & 0x0F) | 0x70               // version 7
	b[8] = (b[8] & 0x3F) | 0x80               // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Client connects to an APL prepl server.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

// Response is a parsed response from the prepl server.
type Response struct {
	ID  string // Request ID (if client sent one)
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
// If id is non-empty, it is sent as a UUID prefix for correlation.
func (c *Client) Eval(expr string, id ...string) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	line := expr
	if len(id) > 0 && id[0] != "" {
		line = expr + " ⍝ID:" + id[0]
	}
	if _, err := fmt.Fprintf(c.conn, "%s\n", line); err != nil {
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

	// Extract optional id
	var respID string
	if v, ok := ns.Values["id"].(string); ok {
		respID = v
	}

	switch tag {
	case "ret":
		resp := &Response{ID: respID, Tag: "ret"}
		if val, ok := ns.Values["val"]; ok {
			resp.Val = val
			resp.Raw = codec.Serialize(val, codec.SerializeOptions{UseDiamond: true})
		}
		return resp, nil

	case "err":
		resp := &Response{ID: respID, Tag: "err", Err: &Error{}}
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
