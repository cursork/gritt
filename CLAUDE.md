# gritt - Go RIDE Terminal

A terminal IDE for Dyalog APL, written in Go using bubbletea.

Pronounced like "grit" (G from Go + German "Ritt" = ride).

## Problem Statement

Dyalog APL ships with RIDE, a graphical IDE. RIDE uses a custom protocol over TCP to communicate with the APL interpreter. The protocol handles:

- Executing APL code and receiving output
- Opening editors for functions/operators
- Debugging (tracing, breakpoints, stepping)
- Dialogs (yes/no prompts, text input)
- Session management

RIDE's UI is Electron-based and dated. We want a modern terminal UI that speaks the same protocol.

## Goal

Build a terminal IDE for Dyalog APL that:

1. Implements the RIDE protocol in Go (client side)
2. Uses [bubbletea](https://github.com/charmbracelet/bubbletea) for the TUI
3. Connects as a "primary client" to a RIDE multiplexer (or directly to Dyalog)
4. Handles session output, editors, tracer, and dialogs

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│     gritt       │◀──RIDE──│   Multiplexer   │◀────▶ Dyalog APL
│  (Go/bubbletea) │  :4502  │   (Clojure)     │
└─────────────────┘         └─────────────────┘
```

gritt connects to either:
- A RIDE multiplexer's primary port (allows other clients to share the session)
- Directly to Dyalog in SERVE mode (`RIDE_INIT=SERVE:*:port`)

## Reference Materials

### ~/dev/ride/docs/protocol.md (FIRST POINT OF CALL)

**Read this first** when implementing any RIDE protocol feature. Complete documentation of all messages, their arguments, and expected behavior. Covers:
- Connection setup, handshake, session control
- Window management (OpenWindow, SaveChanges, CloseWindow)
- Debugging (SetHighlightLine, stepping commands, breakpoints)
- Threads, interrupts, autocompletion, value tips
- Dialogs, workspace explorer (TreeList), status bar

### ~/dev/ride (JavaScript)

Dyalog's official RIDE implementation. The source of truth for protocol behavior.

- Look here when something doesn't work as expected
- Contains handling for all UI messages (OpenWindow, dialogs, etc.)
- `src/wse.js` - Workspace explorer (TreeList usage)
- `src/dbg.js` - Debug pane (SI stack, threads)
- `src/ed.js` - Editor implementation

### ~/dev/dyctl (Clojure)

Working RIDE protocol implementation. Key files:

- `src/dyctl/ride.clj` - Protocol client (handshake, execute, message parsing)
- `src/dyctl/multiplexer.clj` - Multi-client session sharing
- `MULTIPLEXER.md` - Protocol flow documentation
- `CLAUDE.md` - Protocol details (message format, handshake sequence)

### Protocol Summary

**Wire format:**
```
┌─────────┬──────────┬─────────────────┐
│ Length  │  "RIDE"  │     Payload     │
│ 4 bytes │  4 bytes │   UTF-8 JSON    │
│ (BE u32)│  (ASCII) │                 │
└─────────┴──────────┴─────────────────┘
```

**Handshake (connecting to Dyalog in SERVE mode):**
1. Receive `SupportedProtocols=2`
2. Send `SupportedProtocols=2`
3. Send `UsingProtocol=2`
4. Receive `UsingProtocol=2`
5. Send `["Identify", {"apiVersion": 1, "identity": 1}]`
6. Send `["Connect", {"remoteId": 2}]`
7. Wait for `["SetPromptType", {"type": 1}]` (ready)

**Execute:**
- Send: `["Execute", {"text": "code\n", "trace": 0}]`
- Receive: `["AppendSessionOutput", {"result": "..."}]`
- Receive: `["SetPromptType", {"type": 1}]` when ready

**Key message types:**
| Message | Direction | Purpose |
|---------|-----------|---------|
| Execute | → Dyalog | Run APL code |
| AppendSessionOutput | ← Dyalog | Code output (type:14 = input echo, skip it) |
| SetPromptType | ← Dyalog | type:1 = ready for input |
| OpenWindow | ← Dyalog | Open editor/tracer |
| UpdateWindow | ← Dyalog | Editor content |
| CloseWindow | ← Dyalog | Close editor |
| SaveChanges | → Dyalog | Save editor |
| OptionsDialog | ← Dyalog | Yes/No/Cancel prompt |
| ReplyOptionsDialog | → Dyalog | Dialog response |

## Development Approach

1. **Start minimal**: Connect, handshake, send Execute, display output
2. **Add session UI**: Scrolling output, input line, basic bubbletea layout
3. **Add editors**: Handle OpenWindow/UpdateWindow, text editing
4. **Add tracer**: Debug UI, stepping, breakpoints
5. **Add dialogs**: Handle OptionsDialog, StringDialog, etc.

## Testing

**Main test:** `go test -v -run TestTUI` (in tui_test.go)

**Always kill Dyalog before running tests** - stale state causes flaky failures:
```bash
pkill -9 dyalog; sleep 1; go test -v -run TestTUI
```

For manual testing, start Dyalog in SERVE mode:
```bash
RIDE_INIT=SERVE:*:4502 /path/to/dyalog +s -q
```

Or use the multiplexer from dyctl:
```clojure
(require '[dyctl.multiplexer :as mux])
(mux/start! {:dyalog-port 4501 :primary-port 4502 :secondary-port 4503})
```

Then connect gritt to port 4502.

### Test Reports

Running `go test` generates reports in `test-reports/`:
- `test-YYYYMMDD-HHMMSS.html` - Visual report with snapshots
- `test-YYYYMMDD-HHMMSS.txt` - Plain text version (easier to grep/parse)

Use the **text reports** for debugging test failures - they contain the same snapshots without HTML formatting.

## CLI Usage

```bash
# Interactive TUI (requires Dyalog running on localhost:4502)
./gritt

# Launch Dyalog automatically (-l or -launch)
./gritt -l              # Interactive TUI with auto-launched Dyalog
./gritt -l -e "⍳5"      # Execute and exit

# Execute single expression
./gritt -e "⍳5"

# Pipe expressions from stdin
echo "1+1" | ./gritt -stdin
printf "x←5\nx×2\n" | ./gritt -l -stdin   # With auto-launch

# Link directory before executing
./gritt -link /path/to/src -e "MyFn 42"
./gritt -link "#:." -e "⎕nl -3"    # Link root ns to current dir

# Protocol logging (for debugging)
./gritt -log debug.log
```

## Debugging

Run with protocol logging:
```bash
./gritt -log debug.log
```

This logs all RIDE messages and TUI state changes with timestamps. Essential for diagnosing protocol timing issues.

See `adnotata/0003-debugging-protocol.md` for details.

## Dependencies

**Minimal dependencies outside the critical path.** Always ask before adding a new dependency.

Critical path (approved):
- `github.com/charmbracelet/bubbletea` - TUI framework

## Code Style

**Flat project structure.** Minimize number of packages. Splitting into files is fine, but avoid deep package hierarchies. Prefer a handful of well-organized packages over many small ones.

## Non-Goals

- Spawning Dyalog processes (use dyctl or start manually)
- Automation/scripting (use dyctl)
- Being a general-purpose terminal library

## Project Organization

Latin-named files/directories for session continuity:

- **README.md** - User's voice. Add factual updates, user will edit. Don't remove personal commentary.
- **FACIENDA.md** - *Things to be done*: THE TODO list. Any future task goes here, even if mentioned casually by user.
- **OPERANDA.md** - *Things being worked on*: Current state/context for session continuity. NOT a TODO list - don't duplicate tasks here.
- **deliberanda/** - *Things to be deliberated*: One file per pending decision (no prefix, sorted by modification time)
- **adnotata/** - *Things noted*: Numbered exploration entries (0001-topic.md)

### Naming Conventions

**adnotata/** uses numbered prefixes (0001-, 0002-, ...) because these are permanent records. We may go back and add notes to existing entries, so stable ordering matters.

**deliberanda/** has no prefix - just descriptive filenames. Multiple discussions happen concurrently, and most recent modification is the strongest signal for what's active.

### Claude Usage

**Starting a Session:** Read OPERANDA.md first, then FACIENDA.md

**During Work:**
- Update OPERANDA.md on significant progress
- Create adnotata/ entries for exploratory work (numbered sequentially)
- Keep failed attempts in adnotata/ for reference
- Create deliberanda/ files for decisions needing discussion
- **Never do git operations** - user handles all commits
