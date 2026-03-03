# OPERANDA - Current State

Core TUI is functional with session, editors, tracer, variables, autocomplete, docs, command palette, focus mode, history paging, clear screen, save/load session.

## APL keycode support (BK/FD/SR style)

APLers expect standard keycodes like BK (Back), FD (Forward), SR (Redraw Screen). The Dyalog keycode table maps actions to keystrokes (see `$DYALOG/aplkeys/xterm`). We should support these as configurable aliases.

Terminal limitation: ctrl+shift+backspace and ctrl+shift+enter (BK/FD defaults) are not possible — backspace/enter are raw ASCII control chars that can't carry modifier info via escape sequences. We use ctrl+shift+up/down instead.

## IN PROGRESS: Command palette focus bug

Autolocalise feature is implemented and working (autolocalise.go, tests pass). One bug remains:

**Bug:** When dispatching commands from the command palette that target an editor (`toggle-local`, `localise`, `breakpoint`, `format`), focus is lost. The command palette removes itself from the pane manager, and then the dispatched command can't find the correct editor. Current hack at tui.go ~line 678 guesses (tracer → any editor) — WRONG when multiple editors are open.

**Fix needed:** Save the focused pane ID BEFORE the command palette opens. Restore it AFTER the palette dismisses. Remove all per-action focus hacks.

1. Add `prePaletteFocus string` field to Model (tui.go ~line 81)
2. In `openCommandPalette()` (tui.go ~line 2290): save `m.panes.FocusedPane()` ID into that field before adding the commands pane
3. At tui.go ~line 676 after `m.panes.Remove("commands")`: restore focus from saved ID — `m.panes.Focus(m.prePaletteFocus)` — unconditionally, for ALL commands
4. Delete the entire per-action if-block at ~line 678 (`if action == "breakpoint" || action == "toggle-local" || action == "localise"`)
5. Run `pkill -9 dyalog; sleep 1; go test -v -run TestTUI` to verify

**Also run unit tests:** `go test -run 'TestAutolocalise|TestLocalise|TestToggleLocal|TestParseHeader|TestFindAssignedVars|TestStripComment|TestParseGlobals|TestHeaderVars'`

## Recent

- **Autolocalise**: Two new features for tradfn variable localisation:
  - **Autolocalise mode**: Toggle via command palette (`autolocalise`). When enabled, updates header on Enter and save. Supports `⍝ GLOBALS: foo bar` comment to exclude intentional globals. Handles simple assignment (`x←`), modified assignment (`x+←`), chained (`x←y←`), destructuring (`(a b)←`), and `:For` loop variables. Skips comments, strings, system variables (`⎕IO←`), namespace members (`ns.x←`). Config option `"autolocalise": true` in `gritt.json` to default on (per-session, toggle doesn't persist).
  - **Toggle localisation**: Command palette `toggle-local` (like RIDE's Ctrl+Up `TL`). Cursor on a variable name → toggles it in/out of the header. When removing: adds to `⍝ GLOBALS:` (creates comment if autolocalise on; adds to existing comment if autolocalise off). When adding: removes from `⍝ GLOBALS:`. Empty GLOBALS comment is kept as a signal.
  - Title bar shows `[AL]` when autolocalise mode is active.

- **FormatCode**: CLI `-fmt` flag for batch formatting APL files in place — works on both `.aplf` (functions) and `.apln` (namespaces/classes). TUI "format" command in command palette formats the focused editor/tracer. Uses RIDE `FormatCode`/`ReplyFormatCode` protocol messages. CLI opens a dummy editor window (function or namespace via `⎕FIX`) for the required window token. Multiline input (#5) is a prerequisite for creating namespaces interactively in the TUI.
- **Busy spinner**: Animated braille spinner in title bar (`gritt ⠋`) when interpreter is executing. Driven by `m.ready` / SetPromptType. Spinner tick via `tea.Tick` at 80ms. Also fixed Unicode width bug in `renderBox()` (`len(title)` → `len([]rune(title))`).
- **WaitForIdle**: New test helper in `uitest/runner.go` — checks for absence of all spinner braille frames. Replaced all `Sleep` calls after `SendLine` in `tui_test.go` with `WaitForIdle` (deterministic, ~3s faster).
- **Code review fixes**: Rune-safe truncation in `locals_pane.go`, HTTP status check in `aplcart.go`, mutex scope fix in `main.go`, tighter test assertions with negative checks.

See FACIENDA.md for what's next.
