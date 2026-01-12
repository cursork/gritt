package ride

import (
	"fmt"
	"io"
	"time"
)

// Logger is an optional logger for protocol messages.
// Set this before creating a Client to enable logging.
var Logger io.Writer

// logSend logs an outgoing message if Logger is set.
func logSend(cmd string, payload string) {
	if Logger == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(Logger, "[%s] → %s %s\n", ts, cmd, payload)
}

// logRecv logs an incoming message if Logger is set.
func logRecv(payload string) {
	if Logger == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(Logger, "[%s] ← %s\n", ts, payload)
}

// logRaw logs a raw handshake message if Logger is set.
func logRaw(direction, msg string) {
	if Logger == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(Logger, "[%s] %s %s\n", ts, direction, msg)
}
