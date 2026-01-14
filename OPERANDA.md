# OPERANDA - Current State

## Active Work

Connection resilience, command palette, and pane control complete. 39 passing tests.

### Recent Changes

- **Connection resilience**: Disconnection detection, red border + [disconnected] indicator, C-] r to reconnect, session preserved
- **Command palette**: C-] : opens searchable command list (debug, stack, keys, reconnect, save, quit)
- **Pane move mode**: C-] m enters mode where arrows move pane, shift+arrows resize
- **Save session**: Via command palette, prompts for filename with default
- **Config robustness**: Embedded default config, renamed to gritt.json, handles missing keys gracefully

### Key Files Changed

| File | Changes |
|------|---------|
| `tui.go` | Connection state, reconnect, pane move mode, save prompt, command dispatch |
| `config.go` | go:embed, gritt.json paths, graceful missing keys |
| `command_palette.go` | Simplified - returns selected action name |
| `gritt.default.json` | Added reconnect, command_palette, pane_move_mode keys |

### Project Structure

```
gritt/
├── main.go              # Entry point, CLI flags, color detection
├── tui.go               # bubbletea TUI - Model, Update, View
├── apl                  # Shell script for ephemeral Dyalog
├── pane.go              # Floating pane system, cellbuf compositor
├── editor.go            # EditorWindow struct
├── editor_pane.go       # Editor/tracer pane content
├── stack_pane.go        # Stack frame list pane
├── debug_pane.go        # Debug log pane
├── keys_pane.go         # Key mappings pane
├── command_palette.go   # Searchable command list pane
├── keys.go              # KeyMap struct definition
├── config.go            # Config loading (with embedded default)
├── gritt.default.json   # Default key bindings (embedded at build)
├── tui_test.go          # TUI tests (39 tests)
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

# Non-interactive
./gritt -e "⍳5"
echo "1+1" | ./gritt -stdin
./apl "2+2"  # ephemeral Dyalog
```

### Key Bindings (current)

Leader key: `Ctrl+]` (configurable)

| Key | Action |
|-----|--------|
| Enter | Execute line |
| C-] : | Command palette |
| C-] d | Toggle debug pane |
| C-] s | Toggle stack pane |
| C-] m | Pane move mode |
| C-] r | Reconnect |
| C-] ? | Show key mappings |
| C-] q | Quit |
| Tab | Cycle focus between panes |
| Esc | Close pane / exit mode / pop tracer frame |
| Ctrl+C | Shows "Type C-] q to quit" |

---

## Next Session

Remaining pane interactivity:
- Mouse drag edges to resize (partially broken)
- Multiple interactive panes

Or continue with Phase 4 tracer operations:
- Step into/over/out commands
- Breakpoints
