# OPERANDA - Current State

Core TUI is functional with session, editors, tracer, variables, autocomplete, docs, command palette, focus mode, history paging, clear screen, save/load session.

## APL keycode support (BK/FD/SR style)

APLers expect standard keycodes like BK (Back), FD (Forward), SR (Redraw Screen). The Dyalog keycode table maps actions to keystrokes (see `$DYALOG/aplkeys/xterm`). We should support these as configurable aliases.

Terminal limitation: ctrl+shift+backspace and ctrl+shift+enter (BK/FD defaults) are not possible — backspace/enter are raw ASCII control chars that can't carry modifier info via escape sequences. We use ctrl+shift+up/down instead.

## codec package (new)

APLAN parser+serializer ported from dapple/parse (Go) and japlan (JS) into `codec/`. Supports full roundtripping: parse APLAN → Go values → serialize back to APLAN. Handles scalars, vectors, matrices (arbitrary rank), namespaces, complex numbers, zilde. Also includes display-form parser (`Auto()`, `Int()`, etc.) for raw session output. Plus `Equal()` and `Get()` utilities.

See FACIENDA "codec package" section for planned uses (structured variable viewer/editor, .apla formatting, -json output).

## Structured Data Browser

`DataBrowserPane` (`data_browser.go`) implements `PaneContent` for structured viewing and editing of APLAN data. When an editor window has entityType 262144, the APLAN text is parsed via `codec.APLAN()` and shown in a type-specific view:

- **Namespace** — key-value list with type-glyph-prefixed previews (`#` namespace, `⊞` matrix, `≡` vector).
- **Matrix** — grid with row/column headers, 2D cell navigation.
- **Vector** — indexed list with 1-based APL indices.
- **Scalars** — leaf display.

View stack with breadcrumb title bar; Enter drills into compound cells, Esc/Backspace pops out.

### Editing

Enter on a scalar starts inline edit. The buffer is parsed with `codec.APLAN` on confirm — strings are taken raw, everything else goes through APLAN (which already handles ints, floats, complex, negatives, vectors, namespaces). Editing a cell with a vector literal like `7 8 9` promotes the cell to a sub-vector — APL's own value semantics. `widenToOriginalType` was tried and removed: APL itself collapses `5J0`→`5` even from APLAN literals (verified with `(x:5J0)` → `⎕DR` 83), so widening was theatre. See FACIENDA "Cell type collapse on edit" for the deferred design.

### Mutations (configurable bindings, `data-browser` context)

| Command | Default | Behaviour |
|---|---|---|
| `append-row` | `down` | Down on last row appends a new row; `zeroValueFor` recurses so a row of vectors stays a row of vectors of zeros. Repeated presses keep stacking — no pending guard (user explicitly wanted unconstrained append). |
| `append-column` | `right` | Right on last column extends a 2D matrix. No-op for vectors. |
| `delete-row` | `ctrl+d` | Removes selected row (vector or matrix), adjusts cursor when at end. |
| `delete-column` | `alt+d` | Removes selected column (matrix only). |
| `close-discard` | `ctrl+w` | Sets `Discard` flag; tui.go's Esc handler skips `SaveChanges` and just sends `CloseWindow`. |

For `[]any` root vectors the append/delete code mutates `db.stack[0].value` AND `db.root` — the save path serializes `db.root`, and a slice append may reallocate, so both must be kept in sync.

### Save / discard

`tui.go` Esc handler: if `db.modified && !db.Discard` → serialize `db.root` via `codec.Serialize`, set `w.Modified = true`, call `closeEditor` (sends `SaveChanges` then `CloseWindow`). Otherwise (unmodified or close-discard) → remove pane locally, send `CloseWindow`. The "remove pane locally for unmodified" path resolves the earlier interpreter-stuck-after-ShowAsArrayNotation symptom.

### Pending

- Integration tests that re-query the variable after save: `3 3⍴9` (column add, row delete), `(1 2 3)(4 5 6)` (append + drill-edit), close-discard verifying x is unchanged. Required for confidence — earlier UI-only assertions missed real bugs.
- Personal config `~/.config/gritt/gritt.json` doesn't yet have the four new binding entries (only `append-row` is there).
- `amicable.Unmarshal` could start producing `codec.Raw` (newly added next to `FnSource`, both serialize verbatim) for non-data 220⌶ values that APLAN can't represent. Container is in place; wiring is not.

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

APLcart and docs now use `os.UserCacheDir()/gritt/` (`~/Library/Caches/gritt/` on macOS, `~/.cache/gritt/` on Linux, `%LocalAppData%\gritt\` on Windows). Generic cache utilities in `cache/` package (`Dir()`, `Path()`, `IsStale()`). Feature-specific fetch/cache logic in `aplcart/` and `docs/` library packages (TUI pane code still in `package main`).

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
aplsock -l -sock :4200 -mode plain           # Plain text output (for netcat)
aplsock -l -sock :4200 -mode aplor           # 220⌶ binary output
aplsock -addr host:4502 -sock /tmp/apl.sock  # Connect to existing, Unix socket
```

**Three output modes** (input is always APL expression text):
- `-mode aplan` (default): APLAN text `(tag: 'ret' ⋄ val: 1 2 3)`. Parsed by `codec.APLAN` in Go.
- `-mode plain`: Display text `1 2 3`. APL side sends APLAN, Go handler decodes for the external client.
- `-mode aplor`: 220⌶ binary (signed int vector). APL side builds a response namespace (`ns.tag←'ret' ⋄ ns.val←result`), serializes with `1(220⌶)`. Go client gets `amicable.Raw`. Functions/⎕OR round-trip exactly. No stability guarantee across Dyalog versions.

aplan and aplor serialize the same logical structure in different formats and never mix. Errors and void use the same format as results in each mode.

**Tests:** Go integration tests in `prepl/modes_test.go` cover all three modes. Also `prepl/integration_test.go` for comprehensive type coverage in aplan mode. Shell tests in `grittles/aplsock/test.sh`.

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

**Namespace unmarshal:** `Unmarshal` returns `*codec.Namespace` for namespace blobs. Variable members are extracted as typed Go values (int, string, []any, *codec.Array, etc.). Function members are extracted as opaque `Raw` bytes — the namespace-embedded encoding differs from standalone `⎕OR` so they can't yet be decompiled. Sequential walk from `nameTableEnd` via `findNextSubArray` (variables) and `skipFnBlob` (functions). Tests: `TestUnmarshalNamespace` in `decompile_test.go` (6 cases).

**Special types:**
- `amicable.Decimal128` — 16-byte opaque IEEE 754 decimal (no Go equivalent)
- `amicable.Raw` — opaque blob for types we can't parse structurally (standalone ⎕OR). Preserves bytes exactly for round-tripping.

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

**Known limitations:** Multi-line dfns not yet tested. System functions beyond ⎕← and ⎕IO not mapped. Tradfn string literals not yet supported. Nested namespaces not tested. Embedded function members are extracted as opaque `Raw` bytes but can't yet be decompiled standalone — the namespace-embedded encoding differs from standalone `⎕OR` (different literal indices, tradfn-style bytecode structure). See `deliberanda/namespace-unmarshal.md`.

## ibeam package + TUI pane (new)

I-beam (⌶) lookup library at `ibeam/` with TUI pane integration. Two-tier search:

1. **Public Dyalog docs** — queries the cached docs DB (same as doc search). Full-text search over titles and content.
2. **Private/undocumented I-beams** — reads `~/.config/gritt/ibeams.csv`. CSV format: `number,name,signature,description`.

**TUI:** Command palette `ibeam` opens a searchable pane. Type to filter (number prefix match or text search). Enter on public entries opens the full docs page. Enter on private entries shows the description inline with word-wrap and scrolling. Escape from detail returns to list. Page up/down for navigation.

**CSV generator:** `ibeam/cmd/gen-ibeams-csv/` builds the CSV from a Dyalog internal wiki webarchive + a text file of known I-beam numbers. Numbers not in the wiki get "UNKNOWN" entries. 9000–9996 range marked "Allocated to RH".

## `-cfg` flag (new)

`gritt -cfg path` loads a specific config file. `gritt -cfg ''` uses embedded defaults only (no file). Without `-cfg`, the existing hierarchy applies: `./gritt.json` → `~/.config/gritt/gritt.json` → embedded defaults.

## Graceful kill flow (new)

Three-stage process — `)off` → SIGTERM → SIGKILL — applied identically in
TUI and non-TUI (`-e`/`-stdin`/`-sock`/`-fmt`) modes when gritt owns the
Dyalog process (`-l`). Stage 1→2 has a 5 s grace period; stage 2→3 uses
`kill_timeout` from `gritt.json` (default 10 s).

A single launch-time goroutine owns `cmd.Wait()` and closes a
`dyalogExited` channel — every stage races its deadline against this so
a Dyalog that exits during any tier short-circuits the rest. TUI shows a
modal during the SIGTERM countdown with `[esc]` (cancel — gritt and
Dyalog stay alive) and `[k]` (force SIGKILL now). Connect mode
(no `-l`) bypasses everything; gritt just disconnects.

`@pid` introspection command in the palette walks the process tree —
useful because gritt typically launches via `/usr/local/bin/dyalog` →
`mapl` wrapper, so `cmd.Process.Pid` is the wrapper, not the real
interpreter. Full write-up: `adnotata/0011-graceful-kill-and-protocol.md`.

`@`-prefixed introspection commands sort to the bottom of the command
palette via stable sort in `PaletteCommands`.

## Recent

- **Command-palette synonyms**: `CommandDef.Synonyms` ([]string), opt-in per command via `reg.alias(name, synonyms...)` after `reg.add(...)`. Palette `filter()` matches name → synonyms → help text, with `matchRank` ranking them 3/2/1 and a stable sort preserving original order within a tier. Synonyms are hidden — not rendered in the palette list. Seeded across ~45 commands (e.g. `vim`/`emacs`/`code` → external-edit, `idiom` → aplcart, `callstack` → stack, `bp` → breakpoint). Heuristic: skip synonyms that share the command name's first three characters (the user already reaches it by name). TUI test types `vim` and asserts external-edit appears in the filtered list.
- **External editor (`C-] e`)**: `external_edit.go` writes the focused editor pane's text to a temp file (`.aplf`/`.apln`/`.apla` per entityType), runs `$EDITOR <file>` via `tea.ExecProcess` (suspends bubbletea, resumes after exit), reads the file back and triggers `SaveChanges` if it differs. Falls back to `vi`. Splits `$EDITOR` with `strings.Fields` so `EDITOR="code --wait"` works. Refuses on tracer-trace and read-only-value panes — surfaces as `m.transientErr` (new field), rendered red in the status line and cleared on next keypress. New default leader binding `e`. TUI integration test in `tui_test.go` uses a stub `$EDITOR` script that rewrites the file and asserts the new body reaches Dyalog (`⎕CR`).
- **`-sock` extended on `socket-inject` branch**: `gritt -l -sock :PORT` still launches the TUI but also opens a socket server. Each accepted connection reads newline-delimited expressions, the TUI executes them in line with its own input, and the captured `AppendSessionOutput` is written back. The injected expression itself is mirrored into the visible session above the active input line (`drainSocketQueue` in `tui.go`) — so the user sees what produced any output that follows; `lastExecute` skip eats Dyalog's type=14 echo to avoid duplication. Tests with `nc`. Same RIDE channel as the TUI — no separate eval path. Implementation in `socket_inject.go`. Comment in that file flags potential unification with `grittles/aplsock`'s more complete socket implementation; deferred — the two solve different problems (TUI-attached vs. headless). Explicitly *not* extending this with mode-switching modelines (`⍝ MODE: aplor` etc.) — see `adnotata/0012-socket-inject-and-data-protocols.md` for why. Anyone wanting structured-data responses can `⎕FIX` the prepl from inside their gritt session and bypass `-sock` entirely.
- **Multiline input mode**: C-] l toggles multiline mode. When on, Enter adds a new line instead of executing. Title bar shows `[ML]`. Toggling off queues all accumulated lines and sends them one per SetPromptType (same drain pattern as RIDE). Auto-detects nabla vs namespace: nabla body lines get `[n]  ` prefixes, namespace/plain lines keep 6-space indent. Client-side line accumulation, interpreter-compatible sending. Variables pane moved from C-] l to C-] v.
- **History search pane + persistent history**: Ctrl+R opens an overlay pane showing all command history entries. Type to filter, Up/Down to navigate, Enter to select (places command on input line), Escape to close. Deduplicates entries in display. Command history persists across restarts via `~/.cache/gritt/history` (loaded in `NewModel`, saved on quit/`)off`). Capped at 500 entries. Also fixed: Ctrl+L no longer resets history navigation position — if you're scrolling through history with Ctrl+Shift+Up/Down and clear the screen, your position is preserved.
- **Autolocalise**: Three commands for tradfn variable localisation (`autolocalise.go`):
  - **Autolocalise mode**: Toggle via command palette (`autolocalise`). When enabled, updates header on Enter and save. Supports `⍝ GLOBALS: foo bar` comment to exclude intentional globals. Handles simple assignment (`x←`), modified assignment (`x+←`), chained (`x←y←`), destructuring (`(a b)←`), and `:For` loop variables. Skips comments, strings, system variables (`⎕IO←`), namespace members (`ns.x←`). Config option `"autolocalise": true` in `gritt.json` to default on (per-session, toggle doesn't persist). Title bar shows `[AL]` when active.
  - **Toggle localisation**: Command palette `toggle-local` (like RIDE's Ctrl+Up `TL`). Cursor on a variable name → toggles it in/out of the header. When removing: adds to `⍝ GLOBALS:` (creates comment if autolocalise on; adds to existing comment if autolocalise off). When adding: removes from `⍝ GLOBALS:`. Empty GLOBALS comment is kept as a signal.
  - **Localise**: Command palette `localise` — on-demand cleanup that adds missing locals AND removes stale ones.
- **Overlay focus restoration**: All overlay panes (command palette, symbol search, APLcart, doc search) now save/restore the previously focused pane. Commands dispatched from the palette return to the exact editor you were in. Symbol search and APLcart insert into the focused editor (not always the session).
- **FormatCode**: CLI `-fmt` flag for batch formatting APL files in place — works on both `.aplf` (functions) and `.apln` (namespaces/classes). TUI "format" command in command palette formats the focused editor/tracer. Uses RIDE `FormatCode`/`ReplyFormatCode` protocol messages. CLI opens a dummy editor window (function or namespace via `⎕FIX`) for the required window token. Multiline input (#5) is a prerequisite for creating namespaces interactively in the TUI.
- **Busy spinner**: Animated braille spinner in title bar (`gritt ⠋`) when interpreter is executing. Driven by `m.ready` / SetPromptType. Spinner tick via `tea.Tick` at 80ms. Also fixed Unicode width bug in `renderBox()` (`len(title)` → `len([]rune(title))`).
- **WaitForIdle**: New test helper in `uitest/runner.go` — checks for absence of all spinner braille frames. Replaced all `Sleep` calls after `SendLine` in `tui_test.go` with `WaitForIdle` (deterministic, ~3s faster).
- **Code review fixes**: Rune-safe truncation in `locals_pane.go`, HTTP status check in `aplcart.go`, mutex scope fix in `main.go`, tighter test assertions with negative checks.
- **History overhaul**: History file (`~/.cache/gritt/history`) is now append-only, oldest-first. Every TUI execute and every `-e`/`-stdin` expression appends immediately (single `write(2)` per entry, atomic under PIPE_BUF for concurrent processes). No rewrite on quit — crash-safe. Dedup and cap at 500 happen on load. New `-history` flag dumps history to stdout (`gritt -history | tail -5`).
- **Binding guard + warnings pane**: `buildIndexes()` detects single ASCII alphanumeric keys bound as direct (non-leader) commands. Warnings show in a new `WarningsPane` (bottom-right, red, unfocused, Escape to dismiss) and in the debug log. Outside TUI, warnings go to stderr at startup.
- **Help bar**: Now shows F1 docs, C-] : commands, C-] q quit, C-S-↑ history (was: F1, ctrl+l, C-] q).
- **uitest improvements**: `Contains`/`WaitFor` now strip ANSI codes before matching (reports keep colours). New `WaitForLine` for CLI tests — snapshots the screen, then waits for a new line containing the pattern (ignores input echo and stale output).

See FACIENDA.md for what's next.
