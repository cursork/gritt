# OPERANDA - Current State

## Active Work

Key mappings config implemented. Leader key system (`Ctrl+]`) for gritt commands.

### What's New This Session

- **Leader key system**: `Ctrl+]` prefix for gritt commands (configurable)
- **Go test framework**: `uitest/` package wraps tmux, generates HTML reports
- **Config loading**: `config.go` loads from `config.json` or `~/.config/gritt/config.json`
- **Key mappings pane**: `C-] ?` shows floating pane with all bindings
- **Bubbles integration**: viewport in debug pane, help.Model at bottom, key.Binding for all keys
- **cellbuf compositor**: proper pane compositing with styles

### Issues Fixed

- Implemented leader key (`Ctrl+]`) for gritt commands - keeps all keys free for APL
- Removed `?` direct binding (it's roll/deal in APL)
- Removed F1/F2 direct bindings (F1 is for APL docs)
- Commands now behind leader: `C-] ?` for keys, `C-] d` for debug

### Project Structure

```
gritt/
├── main.go           # Entry point
├── tui.go            # bubbletea TUI - Model, Update, View
├── pane.go           # Floating pane system, cellbuf compositor
├── debug_pane.go     # Debug log pane (uses viewport)
├── keys_pane.go      # Key mappings pane (uses viewport)
├── keys.go           # KeyMap definitions
├── config.go         # Config loading from JSON
├── config.json.example
├── tui_test.go       # Go TUI tests (12 tests)
├── uitest/           # Test framework
│   ├── tmux.go       # tmux wrapper
│   ├── report.go     # HTML report generator
│   └── runner.go     # Test runner helpers
├── ride/
│   ├── protocol.go   # Wire format
│   └── client.go     # Connection, handshake
└── test-reports/     # Generated HTML test reports
```

### Testing

```bash
# Run TUI tests (starts Dyalog automatically if needed)
go test -v -run TestTUI

# Manual testing
RIDE_INIT=SERVE:*:4502 dyalog +s -q
go run .
```

### Key Bindings (current)

Leader key: `Ctrl+]` (configurable)

| Key | Action |
|-----|--------|
| Enter | Execute line |
| C-] ? | Show key mappings pane |
| C-] d | Toggle debug pane |
| Tab | Cycle focus between panes |
| Esc | Close focused pane |
| Ctrl+C | Quit |

---

## Next Session

1. Continue with prerequisites:
   - Ctrl+C confirmation / clipboard
   - Connection resilience
   - ⍝« command syntax
2. Then: Phase 3 Editors
