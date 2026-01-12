# OPERANDA - Current State

## Active Work

Tracer stack experience implemented. Protocol logging available for debugging.

### What's Done

- **Tracer stack**: Single tracer pane (not multiple overlapping windows), stack pane for navigation
- **Stack pane**: `C-] s` toggles, shows all suspended frames, click/Enter to switch
- **Protocol logging**: `-log <file>` flag logs all RIDE messages and TUI actions
- **Adaptive colors**: Detects terminal capabilities (ANSI, ANSI256, TrueColor), uses exact #F2A74F when supported
- **CloseWindow timing fix**: Wait for `ReplySaveChanges` before sending `CloseWindow`
- **28 passing tests**: Including full X→Y→Z tracer scenario

### Key Files Changed

| File | Changes |
|------|---------|
| `tui.go` | Tracer stack state, adaptive color init, CloseWindow timing fix |
| `editor.go` | Added `PendingClose` flag |
| `stack_pane.go` | New - stack list pane |
| `ride/logger.go` | New - protocol logging |
| `main.go` | `-log` flag, color profile detection |

### Project Structure

```
gritt/
├── main.go              # Entry point, color profile detection
├── tui.go               # bubbletea TUI - Model, Update, View
├── pane.go              # Floating pane system, cellbuf compositor
├── editor.go            # EditorWindow struct
├── editor_pane.go       # Editor/tracer pane content
├── stack_pane.go        # Stack frame list pane
├── debug_pane.go        # Debug log pane
├── keys_pane.go         # Key mappings pane
├── keys.go              # KeyMap struct definition
├── config.go            # Config loading from JSON files
├── config.default.json  # Default key bindings
├── tui_test.go          # TUI tests (28 tests)
├── uitest/              # Test framework (tmux, HTML reports)
├── ride/
│   ├── protocol.go      # Wire format
│   ├── client.go        # Connection, handshake
│   └── logger.go        # Protocol logging
├── test-reports/        # Generated HTML test reports
└── adnotata/            # Design notes and exploration
```

### Testing

```bash
# Run TUI tests (starts Dyalog automatically if needed)
go test -v -run TestTUI

# With protocol logging
./gritt -log debug.log

# Manual testing
RIDE_INIT=SERVE:*:4502 dyalog +s -q
./gritt
```

### Key Bindings (current)

Leader key: `Ctrl+]` (configurable)

| Key | Action |
|-----|--------|
| Enter | Execute line |
| C-] ? | Show key mappings pane |
| C-] d | Toggle debug pane |
| C-] s | Toggle stack pane (in tracer) |
| C-] q | Quit |
| Tab | Cycle focus between panes |
| Esc | Close focused pane / pop tracer frame |
| Ctrl+C | Shows "Type C-] q to quit" (vim style) |

---

## Next Session

Phase 4 continuation: Tracer operations
- Step into/over/out commands
- Breakpoint toggling
- Variable inspection

Reference: `adnotata/0003-debugging-protocol.md` for protocol debugging approach
