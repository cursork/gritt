# FACIENDA - Things to be done

## aplsock / Prepl
- [ ] **prapl-style exploration UIs in gritt's TUI** — prapl (`~/dev/prapl`) is a PoC proving that text-in / 220⌶-out is a sufficient substrate for a rich data-inspection UI (Navigator with breadcrumb drill-down, Prints with tap channels, per-row "send value to navigator"). Not a thing to integrate or keep alive — it's an idea mine. The gritt-side work is to bring those exploration patterns into the TUI: data_browser already drills into compound values fed by APLAN; the prapl Navigator does the same against aplor (220⌶) responses from a prepl. Since amicable.Unmarshal returns the same Go types data_browser already navigates, an aplor-fed exploration pane is mostly plumbing — bootstrap a prepl on a side thread of gritt's own session (trivial `⎕FIX` + `Start`), route exploration-pane requests through it, hand the unmarshalled value to the existing pane code. Other patterns worth porting: tap channels for live `⎕←`-like output, per-result-row "explore this value" actions. Not urgent — list of ideas to mine, not a single shippable feature.
- [ ] **gritt as client (phase 2)** — `-prepl addr` connects to aplsock instead of RIDE
- [ ] **⎕← capture** — output stream (`tag: 'out'`). Solution is on the APL side.
- [ ] **Multi-line expressions** — framing for `:Namespace`/`:EndNamespace`, nabla
- [ ] **Multi-connection** — per-connection buffers on APL side (currently single `_buf`)
- [ ] **System commands** — `)ts`, `)vars` etc. may not serialize via `Serialise`
- [x] **aplsock transport modes** — `-mode plain`, `-mode aplan` (default), `-mode aplor` (220⌶ binary)
- [x] **`Unmarshal` namespace support** — `amicable.Unmarshal` returns `*codec.Namespace` for namespace blobs. Variable members extracted as typed Go values, function members as opaque `Raw` bytes. aplor mode scalar/string/error tests pass.

## TUI tests
- [ ] **CI: tracer pane doesn't open in `dyalog/dyalog` container** — branch `tests-readme`. TestTUI's tracer tests fail in the GH `dyalog` job (and `act -j dyalog`) because Dyalog *itself* declines to send `OpenWindow {debugger:1}` on a breakpoint hit. Captured protocol shows `HadError {error:1001}` + `AppendSessionOutput "B[2]"` (type 5) and `SetPromptType type:1` — then nothing. Local macOS Dyalog at the same version (20.0.52753) sends the `OpenWindow` immediately after. Handshake is identical (`Identify {apiVersion:1, identity:1}`, `Connect {remoteId:2}`); `RIDE_SPAWNED=1` is now set in `uitest.StartDyalog`; `dyalog.dcfg` with `TRACE_ON_ERROR:1` doesn't change behaviour. Adding `DYALOG=<dir>` + `LD_LIBRARY_PATH` (what `session.DyalogEnv` provides) made Dyalog hang silently at startup instead. Diagnosis needs Dyalog-internals knowledge: what does the macOS app provide (`default.dse` overlay? interpreter flag? env var?) that the container image's bare `dyalog +s -q` doesn't, that causes tracer-window emission. **Mitigation in place:** TestTUI now probes the capability at runtime — the first breakpoint run captures `runner.WaitFor("tracer")` into `tracerSupported`, and the three tracer-dependent blocks (B/breakpoint, X→Y→Z stack, variables-in-tracer) gate on it. When false, `runner.Skip(name, tracerSkipReason)` records each affected test as `SKIP` (distinct from PASS in the report) and `)reset` clears the suspended state so downstream session-level tests run clean. Locally the probe sees `OpenWindow` and the full suite runs as before. Reasoning: dyalog can't be redistributed, but the official dyalog docker container is probably fine.
- [ ] **harden weak test assertions — false positives whenever rendering is broken** — `uitest.Runner.Test` now gates every predicate on `IsAlive()` (gritt rendering its top border, no `connection refused` / `Press any key to exit` markers). This catches dead-UI runs that previously had dozens of tests trivially passing. But individual predicates are still weak: many use `!runner.Contains("X")`, which is true whenever nothing is rendering OR for any reason X happens not to appear. Each such test needs tightening — assert positive evidence of state-change (e.g. `Contains("X")` BEFORE the action, then `!Contains("X")` AFTER), not just absence at one moment. Audit needed across `tui_test.go`; expected scope ~30 tests.

## amicable
- [ ] **decompiler: extend** — multi-line dfns, more system variables, tradfn string literals/locals, embedded function decompilation (different encoding from standalone ⎕OR — see §5.7), nested namespaces
- [ ] **bytecode synthesis** — generate/modify ⎕OR bytecode in Go, send to Dyalog via `0(220⌶)`
- [ ] **nested-namespace unmarshal: deep nesting + interleaved class-9** — current fix handles two top-level layouts: (a) sequential when extraction-order's first member is class-2/3, (b) "relocated tail" when extraction-order's first member is class-9. Untested: multiple class-9 interleaved with class-2/3 (e.g. `[ns, var, ns, var]`); other 9.x classes (instances 9.2, classes 9.4, interfaces 9.5, external classes 9.6).
- [ ] **other 9.x classes** — instances (9.2), classes (9.4), interfaces (9.5), external classes (9.6) all hit the same code path as 9.1 namespaces but have different blob shapes. Currently the recursive `unmarshalNamespace` may misparse them.
- [ ] **generative round-trip tests** — randomly construct namespace/array structures in APL via gritt, capture the 220⌶ blob, unmarshal, re-marshal, and compare. Would surface boundary cases (deep nesting, large fan-out, mixed types per member) without hand-writing every shape. Drives both the bug fix above and confidence in marshal symmetry.

## GitHub Issues
- **#3 Multithreaded tracing** — switch between suspended functions in different threads
- **#4 Inline tracing** — `IT` command: left/right args, current fn, axis spec, previous result
- **#5 Proper multiline mode (covers `-stdin` too)** — basic client-side multiline done (C-] l toggle). Still needed: interpreter-level multiline (nabla/namespace protocol with SetPromptType type=3). Known break: pasting a nabla definition into the TUI's multiline mode leaves the interpreter apparently busy (spinner never clears) — state-tracking bug around the closing `∇`. `-stdin` (main.go:254) is the same problem at a different entry point: it `bufio.Scan`s one Execute per line, so `printf "∇foo\n body\n∇" | gritt -stdin` SYNTAX-errors on the bare `∇foo`. Both paths want one shared "I'm inside a definition, buffer body lines, release ready-state on close" helper.
- **#6 Syntax highlighting** — `)` commands, `]` commands, `⎕` fns, `:Keywords`, glyphs
- **#22 EWC demos don't update UI** — `gritt -l`, link EWC, run a demo in browser mode: logging works, but the UI never changes.

## Data browser
- [ ] **Cell type collapse on edit** — APL has no "complex slot": typing `5` into a cell that was `5J3` saves and reloads as int 5, because the interpreter collapses `5J0`→`5` even when written as an APLAN literal (verified with `foo←(x:5J0)` → `⎕DR foo.x` is 83/INT). Same situation for float→int when the value is whole. The data browser presents typed cells but the wire format doesn't honor that. Possible directions (none designed yet): show cell type as a glyph or colour so the disappearance is visible feedback; warn in the status bar when an edit will narrow on save; let users explicitly "lock" a cell as complex/float (would require a Go-side wrapper that doesn't round-trip through the interpreter). Deferred — no design capacity right now.

## Protocol
- [ ] OptionsDialog proper UI — currently auto-replies "No", needs yes/no/cancel prompt
- [ ] StringDialog (text input dialog)
- [ ] HadError message handling (better error display)

## Kill flow follow-ups
See `adnotata/0011-graceful-kill-and-protocol.md` for context. Today's
flow (`)off` → SIGTERM → SIGKILL) works correctly; these are smoothness
improvements.
- [ ] **Bypass mapl wrapper cleanly** — replicate the env mapl sets
  (`USERCONFIGFILE`, `APL_LANGUAGE_BAR_FILE`, `AUTO_PW`, …) directly in
  Go and launch the real binary. Earlier attempt broke the tracer pane;
  bisect which env var matters. Would give clean PID/wait/signal
  semantics (currently `cmd.Process.Pid` is the shell wrapper, not the
  interpreter).
- [ ] **Pre-emptive `CloseWindow`s on quit** — when gritt initiates
  quit, close every editor/tracer it tracks before sending `)off`.
  Dyalog tries to surface a debug during cleanup → we already told it
  we're done.
- [ ] **Reactive `CloseWindow` during shutdown** — while
  `m.killWaitActive`, immediately reply `CloseWindow` to any
  `OpenWindow {debugger:1}`. Lets Dyalog complete its
  destructor-error path even when no tty is around (the root cause of
  headless SIGTERM hanging).
- [ ] **Optional pty for the spawned interpreter** — restore SIGTERM
  responsiveness end-to-end. Adds significant launch complexity, but
  makes gritt robust against future Dyalog versions that double down on
  tty-gated handlers.

## Pane / UI
- [ ] Mouse drag edges to resize — code exists but buggy in practice
- [ ] Singleton panes should persist position/size after dismiss/recreate
- [ ] Symbol search / APLcart rendering cleanup (consistent gray colors)

## Tracer
- [ ] **Token reuse in nested tracer** — Dyalog sometimes reuses token=1 for all stack frames, sending Y/X as UpdateWindow (not OpenWindow) with ~6s delay. tracerStack only records OpenWindow, so stack depth shows 1 instead of 3. Need virtual tokens or UpdateWindow-aware stack tracking.
- [ ] Test: popping stack frame should update variables pane
- [ ] Test: large values in variables pane should truncate but still allow editing

## Grittles
- [ ] **aplor** — needs adding to grittles README and release workflow
- [ ] **aplfmt** — silent on unchanged files; should print changed filenames
- [ ] **aplcart** — syntax column needs display-width-aware padding

## codec
- [ ] **Structured variable viewer** — render matrices as tables, namespaces as trees
- [ ] **Structured variable editing** — cell-level navigation/editing of matrices
- [ ] **`-json` output for `-e`** — `gritt -l -e "⍳5" -json` for piping to jq

## Other
- [ ] APL keycode aliases (BK, FD, SR etc.)
- [ ] Clipboard support (Ctrl+C copy, Ctrl+V paste)
- [ ] Status bar (connection info, workspace name)
- [ ] Per-directory/project history — currently one global history file, might want separate histories per workspace

---

## Notes

### Session Behavior
The Dyalog session is append-only from the interpreter's perspective. Client shows editable history, but executing always appends.

### RIDE Protocol — Not Yet Implemented
- OptionsDialog — auto-replies "No", needs proper UI
- StringDialog — not handled
- HadError — not handled
