# FACIENDA - Things to be done

## Priority: Pane Interactivity
- [ ] Mouse drag edges to resize (partially broken)
- [ ] Multiple interactive panes: all N panes should be interactive, not just focused one
- [ ] Singleton panes (stack, debug, etc.) should persist position/size after dismiss/recreate
- [ ] Tab should cycle focus back to session (not just between panes)


## Future Enhancements
- [ ] Protocol audit: evaluate all unsupported RIDE messages, prioritize by importance
- [ ] Clipboard support (Ctrl+C copy, Ctrl+V paste)

## Phase 3: Editors - DONE
- [x] Handle OpenWindow/UpdateWindow messages
- [x] Editor pane (floating, using pane system)
- [x] Text editing for functions/operators
- [x] SaveChanges message
- [x] CloseWindow handling
- [x] Tracer window support (debugger:1, SetHighlightLine, WindowTypeChanged)

## Phase 4: Tracer (in progress)
- [x] Stack trace pane (C-] s toggle, click to switch frames)
- [x] Single tracer pane (not multiple overlapping windows like JS RIDE)
- [x] Escape pops stack frame
- [ ] Step into/over/out commands
- [ ] Breakpoints
- [ ] Variable inspection

## Phase 5: Dialogs
- [ ] OptionsDialog (yes/no/cancel prompts)
- [ ] StringDialog (text input)
- [ ] ReplyOptionsDialog/ReplyStringDialog

## Phase 6: APL Input - DONE
- [x] Backtick prefix for APL symbols (`` `a `` → `⍺`, `` `r `` → `⍴`, etc.)
- [x] Symbol search modal - search by name ("iota", "rho", "shape", "each")
- [x] APLcart modal - search idioms from aplcart.info

## Phase 7: Polish
- [ ] Syntax highlighting for APL
- [ ] Input history (beyond session - persist across runs?)
- [ ] Status bar (connection info, workspace name from UpdateSessionCaption)
- [ ] Better error display (HadError message handling)
- [ ] Highlight ⍝« commands in session output

## Future Ideas
- [ ] Multiline input improvements (RIDE does this poorly)
- [ ] Multiple workspace connections?

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
- OpenWindow, UpdateWindow, CloseWindow, SaveChanges, ReplySaveChanges (editors)
- SetHighlightLine, WindowTypeChanged (tracer)

**Not yet implemented:**
- OptionsDialog, StringDialog, Reply* (dialogs)
- HadError (error handling)
- Step/trace control messages

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
- [ ] Mouse: click to focus, drag to move, drag edges to resize (partial - needs fix)
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
- [x] Key mappings pane (C-] ?)

### Phase 2e: Leader Key & Polish
- [x] Leader key system (Ctrl+]) - keeps all keys free for APL
- [x] Quit behind C-] q with y/n confirmation dialog
- [x] Ctrl+C shows vim-style "Type C-] q to quit" hint
- [x] Dyalog orange (#F2A74F) for all UI borders
- [x] ANSI-aware cellbuf compositor for styled panes
- [x] Input routing fix - focused panes consume all keys
- [x] Test reports with ANSI colors and clickable test→snapshot links
- [x] Config from config.default.json (no hardcoded Go defaults)
- [x] Debug pane real-time updates (LogBuffer survives Model copies)

### Phase 2f: Session Fixes
- [x] Input indentation preserved when sending to Dyalog (6-space APL indent)
- [x] External input display (only skip our own echo, show input from Dyalog terminal)

### Phase 4a: Tracer Stack & Debugging Infrastructure
- [x] Tracer stack management (single pane, not multiple overlapping windows)
- [x] Stack pane (C-] s toggle, shows all suspended frames)
- [x] Click/Enter in stack pane switches tracer view
- [x] Escape pops stack frame (sends CloseWindow)
- [x] CloseWindow timing fix (wait for ReplySaveChanges before closing)
- [x] Protocol logging (-log flag for RIDE messages and TUI actions)
- [x] Adaptive color detection (ANSI/ANSI256/TrueColor, exact #F2A74F when supported)

### CLI & Scripting
- [x] Non-interactive mode: -e for single expression, -stdin for piping
- [x] Link support: -link path or -link ns:path runs ]link.create before executing
- [x] apl script: ephemeral Dyalog instance for one-shot execution

### Connection Resilience
- [x] Detect disconnection (EOF, connection reset) - show [disconnected] state with red border
- [x] Keep gritt alive when disconnected: session buffer, debug logs preserved
- [x] Allow reconnect (C-] r) without losing local state
- [x] `)off` intentional shutdown exits cleanly
- [x] External `)off` just disconnects (doesn't exit)
- [x] `⍝ Disconnected` marker in session output

### Config Robustness
- [x] Embedded default config (go:embed gritt.default.json)
- [x] Renamed config files to gritt.json (avoids conflicts)
- [x] Missing key bindings handled gracefully (disabled, not crash)

### Command Palette & Pane Control
- [x] Command palette (C-] :) - searchable command list
- [x] Pane move mode (C-] m) - arrows move, shift+arrows resize
- [x] Save session command (via command palette, prompts for filename)

### APL Input
- [x] Backtick prefix for APL symbols (`` `i `` → `⍳`, `` `r `` → `⍴`, etc.)
- [x] Symbol search (C-] : → symbols) - search by name
- [x] APLcart integration (C-] : → aplcart) - search 3000+ idioms
- [x] 44 passing tests
