# OPERANDA - Current State

## Active Work

Phase 2 (Simple) in progress: bubbletea TUI with session output and input line.

### Current Status

- [x] Phase 1: Minimal RIDE client
- [x] bubbletea TUI with scrolling output
- [x] Dedicated input line at bottom
- [x] Input history (up/down arrows)
- [ ] Proper APL character handling (not yet implemented)

### Project Structure

```
gritt/
├── main.go           # Entry point, connects and starts TUI
├── tui.go            # bubbletea model (output area, input line)
├── ride/
│   ├── protocol.go   # Wire format (send/recv messages)
│   └── client.go     # Connection, handshake, Execute
├── go.mod
├── CLAUDE.md
├── FACIENDA.md
└── OPERANDA.md
```

### Testing

Start Dyalog:
```bash
RIDE_INIT=SERVE:*:4502 dyalog +s -q
```

Run gritt:
```bash
go run . -addr localhost:4502
```

### TUI Features

- Alt-screen mode (clean terminal on exit)
- 6-space prompt matching Dyalog's default
- Prompt shows `...` when waiting for interpreter
- Block cursor with reverse video
- Up/Down for input history
- Basic line editing (backspace, delete, left/right, home/end)
- Ctrl+C to quit

### Key Findings

- Wire format: 4-byte BE length (includes itself) + "RIDE" + payload
- SERVE mode handshake: Dyalog sends first, we respond
- AppendSessionOutput type 14 = input echo (skip it)
- SetPromptType with type > 0 means ready for input

---

## Previous Issues

(none yet)
