package ride

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Client is a RIDE protocol client connected to Dyalog APL.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex // Protects reads
}

// Connect connects to a Dyalog interpreter in SERVE mode and performs handshake.
func Connect(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	c := &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: conn,
	}
	if err := c.handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}

	return c, nil
}

// handshake performs the RIDE protocol handshake for SERVE mode.
// In SERVE mode, Dyalog sends first.
func (c *Client) handshake() error {
	// Receive SupportedProtocols from Dyalog
	if _, raw, err := Recv(c.reader); err != nil {
		return fmt.Errorf("recv SupportedProtocols: %w", err)
	} else if raw != "SupportedProtocols=2" {
		return fmt.Errorf("unexpected: %q", raw)
	}

	// Send our handshake
	if err := sendRaw(c.writer, "SupportedProtocols=2"); err != nil {
		return err
	}
	if err := sendRaw(c.writer, "UsingProtocol=2"); err != nil {
		return err
	}

	// Receive UsingProtocol
	if _, raw, err := Recv(c.reader); err != nil {
		return fmt.Errorf("recv UsingProtocol: %w", err)
	} else if raw != "UsingProtocol=2" {
		return fmt.Errorf("unexpected: %q", raw)
	}

	// Send Identify and Connect
	if err := Send(c.writer, "Identify", map[string]any{"apiVersion": 1, "identity": 1}); err != nil {
		return err
	}
	if err := Send(c.writer, "Connect", map[string]any{"remoteId": 2}); err != nil {
		return err
	}

	// Wait for SetPromptType with type > 0 (interpreter ready)
	for {
		msg, _, err := Recv(c.reader)
		if err != nil {
			return fmt.Errorf("waiting for ready: %w", err)
		}
		if msg != nil && msg.Command == "SetPromptType" {
			if t, ok := msg.Args["type"].(float64); ok && t > 0 {
				return nil
			}
		}
	}
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Send sends a command to the interpreter.
func (c *Client) Send(cmd string, args map[string]any) error {
	return Send(c.writer, cmd, args)
}

// Recv receives a single message from the interpreter.
// Returns (message, raw, error). For JSON messages, message is non-nil.
// For handshake messages, raw is the string.
func (c *Client) Recv() (*Message, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Recv(c.reader)
}

// Execute runs APL code and returns the output.
// Skips input echo (type 14) and waits for SetPromptType.
func (c *Client) Execute(code string) ([]string, error) {
	if err := Send(c.writer, "Execute", map[string]any{"text": code + "\n", "trace": 0}); err != nil {
		return nil, err
	}

	var outputs []string
	for {
		msg, _, err := c.Recv()
		if err != nil {
			return outputs, err
		}
		if msg == nil {
			continue
		}

		switch msg.Command {
		case "AppendSessionOutput":
			// type 14 is input echo - skip it
			if t, ok := msg.Args["type"].(float64); ok && t == 14 {
				continue
			}
			if result, ok := msg.Args["result"].(string); ok {
				outputs = append(outputs, result)
			}
		case "SetPromptType":
			if t, ok := msg.Args["type"].(float64); ok && t > 0 {
				return outputs, nil
			}
		}
	}
}
