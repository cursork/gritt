# OPERANDA - Current State

## Latest Session: Auto-Launch Dyalog

Added `-launch` / `-l` flag to automatically start Dyalog APL with RIDE.

**Usage:**
```bash
./gritt -l              # Interactive TUI with auto-launched Dyalog
./gritt -l -e "⍳5"      # Execute and exit
./gritt -l -stdin       # Pipe mode with auto-launch
```

**Implementation:**
- Picks random port (10000-60000)
- Starts Dyalog with `RIDE_INIT=SERVE:*:$port`
- Polls for port availability (up to 5 seconds)
- Uses process group (`Setpgid: true`) for clean cleanup
- All Dyalog helper processes killed on exit via `syscall.Kill(-pid, SIGKILL)`

---

## Previous Session: Variables Pane Complete

Full variables pane implementation with local/all modes and visual distinction between local and outer-scope variables.

### Variables Pane (C-] l)

**Features:**
- Shows variables with values in tracer or main session
- Two modes with explicit titles: `[local]` and `[all]`
- `~` key toggles between modes
- **Bullet markers (•)** distinguish locals from outer-scope vars in `[all]` mode
- `Down/Up` to select, `Enter` to open editor for variable
- Shows "Loading..." while fetching asynchronously

**Display in [all] mode:**
```
╔ variables [all] ════════════════╗
║• a    = 42                      ║  ← local (declared in function header)
║• b    = hello                   ║  ← local
║  yvar = 123                     ║  ← from outer scope (no bullet)
╚═════════════════════════════════╝
```

**Implementation:**
- `locals_pane.go` - VariablesPane component with `LocalVar.IsLocal` field
- `[local]` mode: parses function source for `name←` patterns
- `[all]` mode: single APL query `{⎕←⍵,'=',⍕⍎⍵}¨↓⎕NL 2`
- Parses function header (`FnName;local1;local2`) to identify locals for bullet markers
- Uses `executeInternal` for silent queries (no session pollution)

**Critical bug fix - chained callbacks:**
- Callbacks from `executeInternal` captured stale Model references (bubbletea value semantics)
- Fix: use single APL query instead of chaining `⎕NL 2` → values query
- Also fixed: callback completion now preserves new query if callback starts one

**Toggle behavior:**
- C-] l focuses existing pane if already open (instead of closing)
- Only closes if already focused

**Editor title fix:**
- Regular editors now show `[edit]` suffix (like tracers show `[tracer]`/`[edit]`)

### Test Coverage (71 tests)

New variables tests:
- `Variables pane shows 'a'` / `'b'` - values displayed correctly
- `Editor opens for variable b` - Enter key opens editor
- `Variables pane shows [all] in title` - ~ toggles mode
- `All mode shows bullet for locals (a, b)` - • prefix on locals
- `All mode shows yvar without bullet` - outer-scope var has no bullet
- `Variables pane back to [local] mode` - ~ toggles back
- `Session variables pane shows sessionVar` - works outside tracer too

---

## Previous Session: Test Suite Improvements

### Changes to tui_test.go

**New: Breakpoint workflow test (function B)**
- Defines multi-line function with `⎕←'before'`, `1+2`, `⎕←'after'`
- Sets breakpoint on line 2 in editor
- Verifies tracer opens at breakpoint when function runs
- Tests breakpoint toggling (add second breakpoint, remove it)
- Tests breakpoint persistence after editing (enter edit mode, make change, Escape back)
- Steps through with 'n' key, verifies output at each step
- Verifies function completes and tracer closes

**Restructured: Stack test moved before manipulation**
- Stack (3 frames) test now runs immediately after X→Y→Z error
- Previously tested after edit mode, which had already popped frames
- Now correctly shows all 3 frames: X, Y, Z

**Fixed: Input line corruption**
- Added `BSpace` before `)erase X Y Z` to clear leftover `⍳` from backtick test
- Session now shows clean `)erase` command

**Consolidated: Breakpoint tests**
- Removed duplicate breakpoint toggling from X→Y→Z section
- All breakpoint tests now in B workflow section

### Test Coverage (63 tests)

Key tests added/fixed:
- `Breakpoint set in editor` - ● appears when set
- `Tracer opens at breakpoint` - function stops at breakpoint
- `Breakpoint still visible in tracer` - persists from editor
- `Command palette shows breakpoint command` - C-] : → break
- `Edit mode active` - 'e' enters edit mode
- `Breakpoint persists after editing` - survives edit + Escape
- `Step executes line` - 'n' advances execution
- `Stack shows 3 frames` - full X→Y→Z stack visible
- `Escape in edit mode returns to tracer` - doesn't close window

---

## Previous Session: Critical Bug Fixes

Phase 4 tracer work complete. Connection resilience improved with automatic window restoration.

### Completed Earlier (this session)

**Breakpoints:**
- Toggle breakpoint: `C-] b` (on current line in editor/tracer)
- Visual indicator: red `●` in gutter
- `SetLineAttributes` sent immediately (breakpoints work without save)
- Breakpoints saved with function via `SaveChanges`

**Tracer Controls (single-key in tracer mode):**
| Key | Action | RIDE Message |
|-----|--------|--------------|
| Enter / n | Step over (next line) | RunCurrentLine |
| i | Step into | StepInto |
| o | Step out | ContinueTrace |
| c | Continue | Continue |
| r | Resume all threads | RestartThreads |
| p | Previous (trace backward) | TraceBackward |
| f | Forward (skip line) | TraceForward |
| e | Enter edit mode | (local) |
| Esc | Close/pop frame | CloseWindow |

**Edit Mode in Tracer:**
- Tracer windows (`Debugger=true`) are read-only by default
- Press `e` to enter edit mode → title changes from `[tracer]` to `[edit]`
- Can edit function code while debugging
- Press `Esc` in edit mode → saves changes, returns to `[tracer]` mode
- Press `Esc` in tracer mode → closes/pops frame

**Automatic Window Restoration:**
- On connect/reconnect, gritt sends `GetWindowLayout`
- Dyalog responds with `OpenWindow` for any orphaned editors/tracers
- Windows automatically restored (no manual intervention needed)
- `close-all-windows` command still available via command palette for manual clearing

**Command Palette Fixes:**
- Added scrolling support
- Fixed rendering issues

**Bug Fixes:**
- Tracer now properly blocks text insertion (checks `Debugger` flag)
- Breakpoints persist across editor close (set `Modified=true`)
- Command palette mouse clicks account for scroll offset

### Key Files Changed

| File | Changes |
|------|---------|
| `editor.go` | `ToggleStop` always sets `Modified` (breakpoints persist) |
| `editor_pane.go` | Tracer mode key handling, edit mode toggle, `InTracerMode()` |
| `tui.go` | `GetWindowLayout` on connect, `closeAllWindows`, tracer controls |
| `command_palette.go` | Scrolling support, rendering fixes |
| `tui_test.go` | Tracer mode tests, APLcart filter test |
| `cmd/explore/main.go` | Protocol exploration tool |

### Pending

1. **Tracer status bar**: Show tracer keys at bottom when tracer focused
2. **Configurable tracer keys**: Move hardcoded keys (n, i, o, etc.) to config

### Testing

```bash
# Run full test suite (71 tests)
go test -v -run TestTUI

# Manual testing with protocol log
./gritt -log debug.log

# Protocol exploration
go run ./cmd/explore/
```

### Project Structure

```
gritt/
├── main.go              # Entry point, CLI flags, color detection
├── tui.go               # bubbletea TUI - Model, Update, View
├── apl                  # Shell script for ephemeral Dyalog
├── pane.go              # Floating pane system, cellbuf compositor
├── editor.go            # EditorWindow struct (Stop, Modified, etc.)
├── editor_pane.go       # Editor/tracer pane (tracer mode, edit mode)
├── stack_pane.go        # Stack frame list pane
├── locals_pane.go       # Variables pane (locals/all modes)
├── debug_pane.go        # Debug log pane
├── keys_pane.go         # Key mappings pane
├── command_palette.go   # Searchable command list pane (with scrolling)
├── apl_symbols.go       # Backtick map and symbol definitions
├── symbol_search.go     # Symbol search pane
├── aplcart.go           # APLcart integration
├── keys.go              # KeyMap struct definition
├── config.go            # Config loading (with embedded default)
├── gritt.default.json   # Default key bindings (embedded at build)
├── tui_test.go          # TUI tests (71 tests)
├── uitest/              # Test framework (tmux, HTML reports)
├── cmd/explore/         # Protocol exploration tool
├── ride/
│   ├── protocol.go      # Wire format
│   ├── client.go        # Connection, handshake
│   └── logger.go        # Protocol logging
├── test-reports/        # Generated HTML test reports
└── adnotata/            # Design notes and exploration
```

---

## Next Session

See FACIENDA.md for TODO list. Key areas to continue:
- Variables pane testing (stack frame updates, large values)
- Tracer polish (bold current line, status bar)
- Pane color consistency
