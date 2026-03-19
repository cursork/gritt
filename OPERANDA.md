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

## Grittles (new)

Standalone CLI tools built on gritt's libraries. Five tools in `grittles/`:

- **aplanconv** — APLAN ↔ JSON conversion (codec only, no Dyalog needed). Auto-detects format via quick peek at first char; ambiguous cases (e.g. `[`) guess and fallback — try one parser, if it fails try the other. Accepts `-from`/`-to` for explicit control and future format extensibility. Also accepts a filename argument instead of stdin.
- **aplcart** — search APLcart from terminal (shares cache with TUI)
- **apldocs** — search and display Dyalog docs with glamour rendering (shares cache with TUI)
- **aplfmt** — format APL source files via Dyalog interpreter
- **aplmcp** — MCP server for LLM ↔ Dyalog (replaces dapple)

New library packages extracted from `package main`:

- **`cache/`** — shared cache dir/path/staleness (extracted from `cache.go`, which is now a thin wrapper)
- **`session/`** — headless Dyalog session API: Launch, Connect, Eval, Format, Link, etc. (extracted from `main.go` + `dyalog.go`; `dyalog.go` deleted from root, `main.go` imports `session.FindDyalog`)
- **`mcp/`** — MCP server (from dapple, rewired to use `session/`)
- **`aplcart/`** — APLcart data loading, caching, search
- **`docs/`** — Dyalog docs search, caching, content retrieval
- **`codec/json.go`** — `ToJSON()` / `FromJSON()` for APLAN ↔ JSON-safe values

Design doc: `GRITTLES-PLAN.md`. README: `grittles/README.md`. dapple is now deprecated — its functionality lives in gritt's libraries.

**Not yet done**: tests for new library packages (`session/`, `mcp/`, `aplcart/`, `docs/`, `codec/json.go`). TUI integration with extracted libraries (aplcart.go and doc_search.go still have their own copies of data functions — not yet switched to the library packages).

## Cache Infrastructure

APLcart and docs now use `os.UserCacheDir()/gritt/` (`~/Library/Caches/gritt/` on macOS). Generic cache utilities in `cache/` package (`Dir()`, `Path()`, `IsStale()`). Feature-specific fetch/cache logic in `aplcart/` and `docs/` library packages (TUI pane code still in `package main`).

- **APLcart**: TSV fetched from GitHub, parsed, stored in `aplcart.db` (SQLite). Loaded synchronously from cache on open (instant). If cache missing, shows "Loading..." and fetches. If stale (>7 days), serves stale immediately, refreshes in background.
- **Docs**: `dyalog-docs.db` downloaded from `xpqz/bundle-docs` GitHub releases. Opened lazily on first docs use. Old location `~/.config/gritt/` no longer used — startup warns if old file exists.
- **`:cache-refresh`**: Command palette command to force re-download both caches.
- **`NO_CACHE=1`**: Env var for tests to force fresh fetch.

Also fixed: `cmd/explore-locals/main.go` had wrong import path (`"gritt/ride"` → `"github.com/cursork/gritt/ride"`).

## Bindable Commands (new)

Unified key dispatch via `CommandRegistry` in `commands.go`. Three separate dispatch systems (leader switch, direct switch, command palette switch) replaced by one registry.

**Architecture**: `buildCommands()` registers all ~30 commands with name, help text, leader/direct/tracer classification, and action closure. `applyBindings()` applies config. `buildIndexes()` partitions into leader/direct/tracer slices. Matching functions: `MatchLeader`, `MatchDirect`, `MatchTracer`, `ByName`.

**Config format**: `bindings` map (command name → `{keys, leader, context}`) + `navigation` map (input primitives). Old `keys` + `tracer_keys` format auto-migrates in `config.go:migrateFromLegacy()`.

**Files changed**: `commands.go` (new), `config.go` (rewritten), `gritt.default.json` (new format), `tui.go` (wired registry), `editor_pane.go` (tracer bindings use `key.Matches`), `keys_pane.go` (auto-generated from registry), `keys.go` (deleted).

**Special cases**: `close-pane` stays inline in `tui.go` due to context-dependent logic (tracer edit mode fallthrough, data browser stack). `edit-mode` tracer binding has nil callback — EditorPane handles it locally by setting `editMode = true`.

## Rebind Pane (new)

`rebind_pane.go` — interactive keybinding editor accessible via command palette `rebind`. Up/down navigate, Enter captures next key press, Tab toggles leader, Delete unbinds. Changes apply immediately via `applyRebind()` in tui.go. 14 unit tests in `rebind_pane_test.go`, 11 TUI integration tests in `tui_test.go`.

Escape-in-capture-mode fix: close-pane handler in tui.go checks `rp.capturing` and falls through to pane's HandleKey (same pattern as tracer edit mode).

**Config save**: `save-config` command writes full config (bindings, navigation, accent, autolocalise) as JSON. Auto-detects existing `./gritt.json` or `~/.config/gritt/gritt.json`; prompts [l]ocal/[g]lobal if neither exists.

**Fixed TUI test failures** (were pre-existing):
- "No docs database message logged" — command was "doc-search" but test typed "docs", no palette match. Renamed to "docs".
- "Load prompt shows default filename" — "cache-refresh" help text ("Re-download") matched "load" in palette and appeared first. Fixed palette filter to prioritize name matches over help-only matches.
- "Loaded session contains saved content" — cascading failure from above.

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
