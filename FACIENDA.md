# FACIENDA - Things to be done

## Prerequisites (before Phase 3)
- [x] Key mappings config - config.json works, leader key system (C-] prefix)
- [ ] Ctrl+C confirmation / clipboard support
- [ ] Connection resilience: on unexpected Dyalog death, keep gritt alive with session buffer intact, show disconnected state, allow reconnect (`)off` intentional shutdown should still exit cleanly)
- [ ] ⍝« command syntax (log save, etc.)

## Phase 3: Editors
- [ ] Handle OpenWindow/UpdateWindow messages
- [ ] Editor pane (floating, using pane system)
- [ ] Text editing for functions/operators
- [ ] SaveChanges message
- [ ] CloseWindow handling

## Phase 4: Tracer
- [ ] Debug UI with stack display
- [ ] Step into/over/out
- [ ] Breakpoints
- [ ] Variable inspection

## Phase 5: Dialogs
- [ ] OptionsDialog (yes/no/cancel prompts)
- [ ] StringDialog (text input)
- [ ] ReplyOptionsDialog/ReplyStringDialog

## Phase 6: Polish
- [ ] Syntax highlighting for APL
- [ ] APL keyboard input layer (backtick prefix?)
- [ ] Input history (beyond session - persist across runs?)
- [ ] Status bar (connection info, workspace name from UpdateSessionCaption)
- [ ] Better error display (HadError message handling)

## Future Ideas
- [ ] Multiline input improvements (RIDE does this poorly)
- [ ] Multiple workspace connections?
- [ ] Session export/save

---

## Notes

### Session Behavior
The Dyalog session is append-only from the interpreter's perspective. Client shows editable history, but executing always appends:
1. User sees previous input `      1+1` with result `2`
2. User navigates up, edits to `      1+2`, executes
3. Original line resets to `      1+1`
4. New line `      1+2` appended, then result `3`

### Multiline Editing
RIDE handles multiline poorly. Research needed on:
- How interpreter expects multiline input
- What protocol messages are involved
- Opportunity to do better than RIDE

### RIDE Protocol Messages (Reference)

**Implemented:**
- Execute (→), AppendSessionOutput (←), SetPromptType (←)

**Not yet implemented:**
- OpenWindow, UpdateWindow, CloseWindow, SaveChanges (editors)
- OptionsDialog, StringDialog, Reply* (dialogs)
- HadError (error handling)
- Trace-related messages (tracer)

---

## Completed

### Phase 1: Minimal RIDE Client
- [x] Connect to Dyalog/multiplexer
- [x] Implement handshake
- [x] Execute APL code and display output

### Phase 2: Session UI (Simple)
- [x] bubbletea TUI with scrolling output
- [x] Input with APL 6-space indent
- [x] Proper APL/UTF-8 character handling

### Phase 2b: Session UI (Full)
- [x] Single editable session buffer
- [x] Navigate anywhere, edit previous inputs, re-execute
- [x] Original line restored, edited version appended at bottom
- [x] Navigation: arrows, Home/End, PgUp/PgDn, mouse scroll
- [x] Debug pane with protocol messages (F12)
- [x] Empty line insertion for spacing

### Phase 2c: Floating Panes
- [x] Floating pane system (pane.go)
- [x] Cell-based compositor for rendering panes over session
- [x] Focus management with visual indicator (double border)
- [x] Mouse: click to focus, drag to move, drag edges to resize
- [x] Keyboard: Tab to cycle focus, Esc to close pane
- [x] Debug pane migrated to floating pane (scrollable)

### Phase 2d: Bubbles Integration & Testing
- [x] Upgraded to lipgloss v2, added bubbles
- [x] viewport.Model for debug pane scrolling
- [x] help.Model for keybindings display at bottom
- [x] key.Binding for all keybindings
- [x] cellbuf for pane compositing (replaces custom grid)
- [x] Go test framework (uitest/) - wraps tmux, HTML reports
- [x] Config loading from config.json
- [x] Key mappings pane (F1) - but duplicates ? help, needs fix
