# OPERANDA - Current State

Core TUI is functional with session, editors, tracer, variables, autocomplete, docs, command palette, focus mode, history paging, clear screen, save/load session.

## APL keycode support (BK/FD/SR style)

APLers expect standard keycodes like BK (Back), FD (Forward), SR (Redraw Screen). The Dyalog keycode table maps actions to keystrokes (see `$DYALOG/aplkeys/xterm`). We should support these as configurable aliases.

Terminal limitation: ctrl+shift+backspace and ctrl+shift+enter (BK/FD defaults) are not possible — backspace/enter are raw ASCII control chars that can't carry modifier info via escape sequences. We use ctrl+shift+up/down instead.

## codec package (new)

APLAN parser+serializer ported from dapple/parse (Go) and japlan (JS) into `codec/`. Supports full roundtripping: parse APLAN → Go values → serialize back to APLAN. Handles scalars, vectors, matrices (arbitrary rank), namespaces, complex numbers, zilde. Also includes display-form parser (`Auto()`, `Int()`, etc.) for raw session output. Plus `Equal()` and `Get()` utilities.

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

TUI deduplication complete: `aplcart.go` and `doc_search.go` now delegate to library packages (`aplcart/` and `docs/`) — all data loading, search, caching, and refresh logic removed from root. TUI pane structs (rendering, key handling) stay in `package main`. `tui.go` calls library functions directly (`aplcart.LoadCache()`, `docs.OpenCache()`, `docs.CacheIsStale()`). `aplcart/aplcart_test.go` has 5 unit tests covering cache round-trip, TSV parsing, and search.

## Cache Infrastructure

APLcart and docs now use `os.UserCacheDir()/gritt/` (`~/Library/Caches/gritt/` on macOS). Generic cache utilities in `cache/` package (`Dir()`, `Path()`, `IsStale()`). Feature-specific fetch/cache logic in `aplcart/` and `docs/` library packages (TUI pane code still in `package main`).

- **APLcart**: TSV fetched from GitHub, parsed, stored in `aplcart.db` (SQLite). Loaded synchronously from cache on open (instant). If cache missing, shows "Loading..." and fetches. If stale (>7 days), serves stale immediately, refreshes in background.
- **Docs**: `dyalog-docs.db` downloaded from `xpqz/bundle-docs` GitHub releases. Opened lazily on first docs use. Old location `~/.config/gritt/` no longer used — startup warns if old file exists.
- **SQLite driver**: `modernc.org/sqlite` (pure Go, no cgo). Enables `CGO_ENABLED=0` cross-compilation in release workflow.
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

## aplsock / Prepl (new — experimental)

Pure APL socket server inspired by Clojure's prepl. Long-term goal: replace RIDE as gritt's primary connection to Dyalog, running entirely in APL on a separate interpreter thread.

**Architecture:** `aplsock` bootstraps an APL prepl server inside Dyalog via RIDE, then proxies between external clients and the APL server. RIDE stays connected (drain goroutine reads messages) but all eval goes through the prepl's Conga TCP channel.

**What exists:**
- `prepl/Prepl.apln` — APL namespace: Conga TCP server, `⍎` in `#` context, APLAN serialization via `⎕SE.Dyalog.Array.Serialise` + `62583⌶` (compact formatter). Standalone-testable.
- `prepl/client.go` — Go client: `Eval` (parses response APLAN via codec), `EvalRaw` (raw passthrough), `UUIDv7()` generator.
- `prepl/embed.go` — `go:embed` of APL source for bootstrap injection.
- `grittles/aplsock/` — standalone binary with `test.sh`.
- `grittles/aplsock/testdyalog/` — helper to start Dyalog with RIDE for testing.

**Protocol (pure APLAN):**
```
→ 1+2                                              plain expression
← (tag: 'ret' ⋄ val: 3)

→ ⍳5 ⍝ID:019abc12-3456-7890-abcd-ef1234567890     with correlation ID
← (id: '019abc12-...' ⋄ tag: 'ret' ⋄ val: 1 2 3 4 5)

→ ÷0
← (tag: 'err' ⋄ en: 11 ⋄ message: 'Divide by zero' ⋄ dm: (...))
```
ID is optional (`⍝ID:uuid` trailing comment — `⍎` ignores it). For tooling correlation, not required for interactive use.

**Usage:**
```
aplsock -l -sock :4200                       # Launch Dyalog, serve on TCP 4200
aplsock -addr host:4502 -sock /tmp/apl.sock  # Connect to existing, Unix socket
nc localhost 4200                             # Interactive netcat session
```

**Test suite:** `grittles/aplsock/test.sh` — tests both `-l` and existing-Dyalog modes. Covers scalars, vectors, matrices, strings, namespaces, nested/mixed arrays, errors, dfn assignment, error recovery, complex numbers, booleans, raw protocol with `⍝ID:`.

**Key learnings:**
- `RIDE_SPAWNED=1` env var is critical when launching Dyalog — without it, threads spawned with `&` don't get scheduled.
- Running Conga event loop on thread 0 deadlocks with RIDE (both use Conga). Prepl MUST run on its own thread.
- Eval stores result in `#.⍙r` (global) before serializing, then cleans up.
- `62583⌶` (Kamila's APLAN formatter) with left arg 1 compacts `Serialise` output to single-line with `⋄` separators.
- Namespace references are serialized via `NsToAPLAN` which iterates members and serializes each value. `⎕NC` check routes namespaces to this path.

**Modes:** Default serves raw APLAN (for tooling). `-repl` flag decodes to plain text (for interactive nc).

**Tests:** Go integration tests in `prepl/integration_test.go` cover all scalar types, vectors (int/float/string/unicode/bool/complex/empty), matrices (int/char/rank-3), nested structures (simple/mixed/deep), namespaces, errors, ID correlation, raw mode, and large arrays. Also `grittles/aplsock/test.sh` for shell-level protocol tests.

**Known limitations:**
- `⎕←` in expressions is a no-op (output goes to RIDE drain, not returned to client). Parked for APL-side solution.
- System commands (`)ts`, `)vars`) may not serialize cleanly.
- Single shared `_buf` on APL side — one connection at a time per prepl server.

**Design decisions:** See `deliberanda/prepl.md`.

## amicable package (new)

Go library for Dyalog's `220⌶` binary array serialization format. Named after 220, the first amicable number. Package: `amicable/`.

**Files:** `amicable.go` (marshal/unmarshal), `decompile.go` (⎕OR bytecode → APL source), `amicable_test.go` (unit tests), `e2e_test.go` (Dyalog round-trip tests), `decompile_test.go` (decompiler e2e tests).

### Array Serialization

**API:** `amicable.Unmarshal([]byte) (any, error)` and `amicable.Marshal(any) ([]byte, error)`. Uses same Go types as `codec` package (`*codec.Array`, `string`, `[]any`, `int`, `float64`, `complex128`).

**Format (reverse-engineered):** 2-byte magic (`DF A4` 64-bit, `DF 94` 32-bit), then ptrSize-aligned fields: size, type/rank, shape, data. Type codes are Dyalog-internal (0x21=bool through 0x2E=decimal128, 0x06=nested, 0x00=opaque). Reads both 32-bit and 64-bit formats, writes 64-bit. Full spec in `adnotata/0010-220-ibeam-binary-format.md`.

**Special types:**
- `amicable.Decimal128` — 16-byte opaque IEEE 754 decimal (no Go equivalent)
- `amicable.Raw` — opaque blob for types we can't parse structurally (⎕OR, namespaces). Preserves bytes exactly for round-tripping.

**Tests:** Unit tests with exact Dyalog v20 bytes, Go round-trips, byte-exact comparison with Dyalog output, e2e tests (serialize in APL → unmarshal/marshal in Go → deserialize in APL, verify `≡` identity for 25 array types). Includes ⎕OR dfn round-trip challenge.

### ⎕OR Bytecode Decompiler

`Raw.Decompile()` reconstructs APL dfn source from opaque ⎕OR binary blobs — no Dyalog interpreter needed. The bytecode format was reverse-engineered by probing Dyalog v20.

**All decompiler test cases pass** (dfns including all primitives, tradfns, namespaces). Each test serializes via `⎕OR` in a live Dyalog session, unmarshals to `Raw`, decompiles, and compares with the original. Tested:

- Arithmetic: `{⍵+1}`, `{⍺+⍵}`, `{⍵-1}`, `{⍵×2}`
- Operators: `{+/⍵}`, `{+\⍵}`, `{+⍨⍵}`
- Control flow: `{0=⍵:0 ⋄ ⍵}`, `{r←⍵+1 ⋄ r}`
- Expressions: `{(⍵+1)×2}`, `{⍵[1]}`, `{⎕IO}`
- Strings: `{⎕←'hello world'}`
- Recursion: `{⍵≤1:⍵ ⋄ (∇⍵-1)+∇⍵-2}` (fibonacci), `{0=⍵:⍺ ⋄ ⍵∇⍵|⍺}` (GCD)
- Real functions: `{0=2|⍵:⍵÷2 ⋄ 1+3×⍵}` (Collatz), `{(+/⍵)÷≢⍵}` (average), `{×/⍵⍴⍺}` (power)
- Tradfns: `r←add x / r←x+1`, `halve x / ⎕←x÷2`, `r←a gcd b` with `:If/:Else/:EndIf`
- Namespaces: variable-only (`ns.x←42 ⋄ ns.name←'Neil'`), function-only (`ns.double←{⍵×2}`), no-literal (`ns.avg←{(+/⍵)÷≢⍵}`)

**How it works:**
1. Finds the bytecode char8 vector inside the ⎕OR blob (FF FF header marker)
2. Extracts expression regions between `XX 1B 6F` (start) and `XX 1E 6F` (end) markers
3. Decodes tokens: single-byte primitives (02=+, 03=−, ...), 2-byte refs (XX 4C=name, XX 57=literal, XX 3E=sysvar), operator suffixes (40=/, 42=\\, 47=¨, 4A=⍨)
4. Resolves literal pool references — sub-arrays after bytecode, stored in **reverse order** (last sub-array = pool[0])
5. Variable names are inline ASCII bytes, arg refs are 00=⍺ 01=⍵ 02=∇

**Tradfn decompiler** uses the same token codes but different framing: names indexed from 0x70 in reverse order (extracted from char16 entries in blob), lines separated by 0x67, control keywords (`:If`=0x00, `:Else`=0x04, `:EndIf`=0x05) in `XX YY 6F` markers. Literal pool offset = number of names.

**Grittle: `aplor`** — standalone CLI tool (`grittles/aplor/`) that reads 220⌶ bytes from stdin or file and prints decompiled APL source. No Dyalog installation needed on the machine running aplor. Usage: `gritt -l -e "1(220⌶)⎕OR'fn'" | aplor`.

**Vision:** With amicable as transport and aplor for decompilation, Dyalog can live on a remote server while Go tooling on the client side can: parse arrays into native types, decompile function source, and eventually synthesize/modify bytecode — all without a local Dyalog installation.

**Known limitations:** Multi-line dfns not yet tested. System functions beyond ⎕← and ⎕IO not mapped. Tradfn string literals not yet supported. Namespace function literals may fail in mixed (var+fn) namespaces. Nested namespaces not tested.

## ibeam package + TUI pane (new)

I-beam (⌶) lookup library at `ibeam/` with TUI pane integration. Two-tier search:

1. **Public Dyalog docs** — queries the cached docs DB (same as doc search). Full-text search over titles and content.
2. **Private/undocumented I-beams** — reads `~/.config/gritt/ibeams.csv`. CSV format: `number,name,signature,description`.

**TUI:** Command palette `ibeam` opens a searchable pane. Type to filter (number prefix match or text search). Enter on public entries opens the full docs page. Enter on private entries shows the description inline with word-wrap and scrolling. Escape from detail returns to list. Page up/down for navigation.

**CSV generator:** `ibeam/cmd/gen-ibeams-csv/` builds the CSV from a Dyalog internal wiki webarchive + a text file of known I-beam numbers. Numbers not in the wiki get "UNKNOWN" entries. 9000–9996 range marked "Allocated to RH".

## `-cfg` flag (new)

`gritt -cfg path` loads a specific config file. `gritt -cfg ''` uses embedded defaults only (no file). Without `-cfg`, the existing hierarchy applies: `./gritt.json` → `~/.config/gritt/gritt.json` → embedded defaults.

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
