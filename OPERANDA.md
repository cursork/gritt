# OPERANDA - Current State

## Active Work

Phase 2b complete. Solid foundation for a terminal APL IDE.

### What Works

- **RIDE protocol**: Connection, handshake, Execute, AppendSessionOutput, SetPromptType
- **Session UI**: Full editable session buffer with APL 6-space indent convention
- **Navigation**: Arrow keys, Home/End, PgUp/PgDn, mouse wheel scrolling
- **Edit any line**: Navigate to previous input, edit, execute - original restored, edited version appended
- **UTF-8/APL**: Proper rune-based cursor and editing (⎕, ⍳, etc. work correctly)
- **Empty lines**: Press Enter on empty input to add spacing
- **Debug pane**: F12 toggles protocol message log on right side
- **Clean layout**: Manual box rendering with titled borders

### Project Structure

```
gritt/
├── main.go           # Entry point, tea.NewProgram with mouse support
├── tui.go            # bubbletea TUI - Model, Update, View, rendering
├── ride/
│   ├── protocol.go   # Wire format (length + "RIDE" + JSON)
│   └── client.go     # Connection, handshake, Send/Recv
```

Note: `session.go` was removed - line tracking now integrated into tui.go's `Line` struct.

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
| Arrow keys | Navigate cursor |
| Home/End | Start/end of line |
| PgUp/PgDn | Scroll by page |
| Mouse wheel | Scroll by 3 lines |
| F12 | Toggle debug pane |
| Ctrl+C | Quit |

---

## Previous Issues (Resolved)

- **Race condition**: Multiple waitForRide goroutines racing on socket reads. Fixed with single reader goroutine + channel.
- **Message framing corruption**: Added buffered reader + mutex.
- **Layout bugs**: Raw ANSI cursor codes broke box width calculations. Fixed by using lipgloss styles.
- **UTF-8 corruption**: Byte-based string slicing corrupted multi-byte APL chars. Fixed with rune-based editing.
- **Blank line duplication**: Edit-execute logic was appending instead of replacing input line. Fixed.

---

## Next Up

Phase 3: Editor windows (OpenWindow/UpdateWindow/SaveChanges)
