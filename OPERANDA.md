# OPERANDA - Current State

## Active Work

Floating pane system implemented. Ready for testing.

### What's New

- **Floating panes**: All panes are now floating, resizable, moveable
- **Cell-based compositor**: Panes render over session content
- **Focus management**: Tab cycles focus, visual indicator (double border when focused)
- **Mouse support**: Click to focus, drag title to move, drag edges to resize
- **Debug pane migrated**: F12 creates a floating debug pane (scrollable with keys/mouse)

### What Works

- **RIDE protocol**: Connection, handshake, Execute, AppendSessionOutput, SetPromptType
- **Session UI**: Full editable session buffer with APL 6-space indent convention
- **Navigation**: Arrow keys, Home/End, PgUp/PgDn, mouse wheel scrolling
- **Edit any line**: Navigate to previous input, edit, execute - original restored, edited version appended
- **UTF-8/APL**: Proper rune-based cursor and editing
- **Floating panes**: Moveable, resizable, focus-aware
- **Debug pane**: F12 toggles, scrollable, moveable

### Project Structure

```
gritt/
├── main.go           # Entry point
├── tui.go            # bubbletea TUI - Model, Update, View
├── pane.go           # Floating pane system (Pane, PaneManager, compositor)
├── debug_pane.go     # Debug log pane implementation
├── ride/
│   ├── protocol.go   # Wire format
│   └── client.go     # Connection, handshake
```

### Testing

```bash
# Terminal 1
RIDE_INIT=SERVE:*:4502 dyalog +s -q

# Terminal 2
go run .
```

### Key Bindings

| Key | Action |
|-----|--------|
| Enter | Execute line (or add blank line if empty) |
| Arrow keys | Navigate cursor / scroll debug pane when focused |
| Home/End | Start/end of line |
| PgUp/PgDn | Scroll by page |
| Mouse wheel | Scroll by 3 lines |
| F12 | Toggle debug pane |
| Tab | Cycle focus between panes |
| Esc | Close focused pane |
| Ctrl+C | Quit |

### Mouse Actions

| Action | Effect |
|--------|--------|
| Click on pane | Focus pane |
| Click on session | Unfocus panes |
| Drag title bar | Move pane |
| Drag edges/corners | Resize pane |
| Scroll wheel | Scroll focused pane or session |

---

## Next Up

Prerequisites from `deliberanda/editor-prerequisites.md`:
1. Key mappings config (config.json)
2. Ctrl+C confirmation / clipboard support
3. Connection resilience
4. ⍝« command syntax

Then: Editor windows (Phase 3)
