# OPERANDA - Current State

Core TUI is functional with session, editors, tracer, variables, autocomplete, docs, command palette, focus mode, history paging, clear screen, save/load session.

## APL keycode support (BK/FD/SR style)

APLers expect standard keycodes like BK (Back), FD (Forward), SR (Redraw Screen). The Dyalog keycode table maps actions to keystrokes (see `$DYALOG/aplkeys/xterm`). We should support these as configurable aliases.

Terminal limitation: ctrl+shift+backspace and ctrl+shift+enter (BK/FD defaults) are not possible — backspace/enter are raw ASCII control chars that can't carry modifier info via escape sequences. We use ctrl+shift+up/down instead.

## codec package (new)

APLAN parser+serializer ported from dapple/parse (Go) and japlan (JS) into `codec/`. Supports full roundtripping: parse APLAN → Go values → serialize back to APLAN. Handles scalars, vectors, matrices (arbitrary rank), namespaces, complex numbers, zilde. 97 tests. Also includes display-form parser (`Auto()`, `Int()`, etc.) for raw session output. Plus `Equal()` and `Get()` utilities.

See FACIENDA "codec package" section for planned uses (structured variable viewer/editor, .apla formatting, -json output).

## Structured Data Browser

New `DataBrowserPane` in `data_browser.go` implements `PaneContent` for structured viewing and editing of APLAN data. When an editor window has entityType 262144, the APLAN text is parsed via `codec.APLAN()` and shown in a type-specific view instead of raw text:

- **Namespace**: key-value list with type-glyph-prefixed previews
- **Matrix**: grid with row/column headers, 2D cell navigation, selected column header highlighted
- **Vector**: indexed list with 1-based APL indices
- **Scalars**: simple display (leaf nodes, no drill-down)

View stack with breadcrumb title bar for drill-down navigation. Enter drills in, Esc/Backspace pops out. Type glyphs: `#` namespace, `⊞` matrix, `≡` vector.

**v1 editing** (implemented, not yet tested): Enter on scalar starts inline edit with cursor. Type validation maintains original type (int→int, string→string, etc.). Red error on invalid input. Save serializes modified root back to APLAN via SaveChanges on close.

Integration in `tui.go`: OpenWindow and UpdateWindow both check for entityType 262144 and swap to DataBrowserPane. ClosePane (Esc) handler has special data browser path: cancel edit → pop stack → save-if-modified → close. Design doc in `deliberanda/structured-editing.md`.

### KNOWN BUG: Interpreter stuck after ShowAsArrayNotation

**Symptoms**: Spinner in title bar after opening data browser via `)ed data` + Enter (ShowAsArrayNotation). Interpreter becomes unresponsive — session commands show spinner too. Esc at data browser root calls `sendCloseWindow(token)` but pane stays because it waits for Dyalog's CloseWindow response which never comes (interpreter stuck).

**Suspicion**: ShowAsArrayNotation (or the UpdateWindow response changing entityType to 262144) may leave the interpreter in a busy state — SetPromptType type=1 may not be sent after the conversion. Need to check protocol logs (`-log debug.log`) to see what messages flow during ShowAsArrayNotation and whether we get a SetPromptType back.

**Possible fix**: For unmodified data browser close, remove the pane locally immediately instead of waiting for Dyalog's CloseWindow response. For the stuck interpreter, may need to investigate the ShowAsArrayNotation protocol flow — check if RIDE does anything extra after receiving the UpdateWindow.

**Not yet done**: testing editing, adding/removing elements, pagination.

## Recent

- **Autolocalise**: Three commands for tradfn variable localisation (`autolocalise.go`):
  - **Autolocalise mode**: Toggle via command palette (`autolocalise`). When enabled, updates header on Enter and save. Supports `⍝ GLOBALS: foo bar` comment to exclude intentional globals. Handles simple assignment (`x←`), modified assignment (`x+←`), chained (`x←y←`), destructuring (`(a b)←`), and `:For` loop variables. Skips comments, strings, system variables (`⎕IO←`), namespace members (`ns.x←`). Config option `"autolocalise": true` in `gritt.json` to default on (per-session, toggle doesn't persist). Title bar shows `[AL]` when active.
  - **Toggle localisation**: Command palette `toggle-local` (like RIDE's Ctrl+Up `TL`). Cursor on a variable name → toggles it in/out of the header. When removing: adds to `⍝ GLOBALS:` (creates comment if autolocalise on; adds to existing comment if autolocalise off). When adding: removes from `⍝ GLOBALS:`. Empty GLOBALS comment is kept as a signal.
  - **Localise**: Command palette `localise` — on-demand cleanup that adds missing locals AND removes stale ones.
- **Overlay focus restoration**: All overlay panes (command palette, symbol search, APLcart, doc search) now save/restore the previously focused pane. Commands dispatched from the palette return to the exact editor you were in. Symbol search and APLcart insert into the focused editor (not always the session).
- **FormatCode**: CLI `-fmt` flag for batch formatting APL files in place — works on both `.aplf` (functions) and `.apln` (namespaces/classes). TUI "format" command in command palette formats the focused editor/tracer. Uses RIDE `FormatCode`/`ReplyFormatCode` protocol messages. CLI opens a dummy editor window (function or namespace via `⎕FIX`) for the required window token. Multiline input (#5) is a prerequisite for creating namespaces interactively in the TUI.
- **Busy spinner**: Animated braille spinner in title bar (`gritt ⠋`) when interpreter is executing. Driven by `m.ready` / SetPromptType. Spinner tick via `tea.Tick` at 80ms. Also fixed Unicode width bug in `renderBox()` (`len(title)` → `len([]rune(title))`).
- **WaitForIdle**: New test helper in `uitest/runner.go` — checks for absence of all spinner braille frames. Replaced all `Sleep` calls after `SendLine` in `tui_test.go` with `WaitForIdle` (deterministic, ~3s faster).
- **Code review fixes**: Rune-safe truncation in `locals_pane.go`, HTTP status check in `aplcart.go`, mutex scope fix in `main.go`, tighter test assertions with negative checks.

See FACIENDA.md for what's next.
