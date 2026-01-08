package ride

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Wire format:
// ┌─────────┬──────────┬─────────────────┐
// │ Length  │  "RIDE"  │     Payload     │
// │ 4 bytes │  4 bytes │   UTF-8 JSON    │
// │ (BE u32)│  (ASCII) │                 │
// └─────────┴──────────┴─────────────────┘
//
// Length includes "RIDE" + payload (not itself).

const rideHeader = "RIDE"

// sendRaw writes a raw RIDE message (payload includes "RIDE" prefix for handshake messages).
func sendRaw(w io.Writer, payload string) error {
	data := []byte(rideHeader + payload)
	length := uint32(len(data) + 4)

	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write length: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

// recvRaw reads a raw RIDE message, returning the payload (without "RIDE" prefix).
func recvRaw(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", fmt.Errorf("read length: %w", err)
	}

	// Sanity check - messages shouldn't be huge
	if length < 8 || length > 10*1024*1024 {
		return "", fmt.Errorf("invalid length: %d", length)
	}

	buf := make([]byte, length-4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("read payload: %w", err)
	}

	s := string(buf)
	if strings.HasPrefix(s, rideHeader) {
		return s[4:], nil
	}
	return s, nil
}

// Message is a RIDE protocol message: ["Command", {args}]
type Message struct {
	Command string
	Args    map[string]any
}

// Send writes a JSON command message.
func Send(w io.Writer, cmd string, args map[string]any) error {
	data, err := json.Marshal([]any{cmd, args})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return sendRaw(w, string(data))
}

// Recv reads and parses a message. Returns the raw string for handshake messages.
func Recv(r io.Reader) (*Message, string, error) {
	payload, err := recvRaw(r)
	if err != nil {
		return nil, "", err
	}

	// Handshake messages aren't JSON
	if !strings.HasPrefix(payload, "[") {
		return nil, payload, nil
	}

	// Parse JSON: ["Command", {args}]
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &arr); err != nil {
		return nil, payload, nil // Return as raw if parse fails
	}
	if len(arr) < 2 {
		return nil, payload, nil
	}

	var cmd string
	if err := json.Unmarshal(arr[0], &cmd); err != nil {
		return nil, payload, nil
	}

	var args map[string]any
	if err := json.Unmarshal(arr[1], &args); err != nil {
		return nil, payload, nil
	}

	return &Message{Command: cmd, Args: args}, "", nil
}
