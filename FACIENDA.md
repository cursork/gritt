# FACIENDA - Things to be done

## aplsock / Prepl
- [ ] **gritt as client (phase 2)** — `-prepl addr` connects to aplsock instead of RIDE
- [ ] **⎕← capture** — output stream (`tag: 'out'`). Solution is on the APL side.
- [ ] **Multi-line expressions** — framing for `:Namespace`/`:EndNamespace`, nabla
- [ ] **Multi-connection** — per-connection buffers on APL side (currently single `_buf`)
- [ ] **System commands** — `)ts`, `)vars` etc. may not serialize via `Serialise`
- [x] **aplsock transport modes** — `-mode plain`, `-mode aplan` (default), `-mode aplor` (220⌶ binary)
- [x] **`Unmarshal` namespace support** — `amicable.Unmarshal` returns `*codec.Namespace` for namespace blobs. Variable members extracted as typed Go values, function members as opaque `Raw` bytes. aplor mode scalar/string/error tests pass.

## TUI tests
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
- **#5 Proper multiline mode** — basic client-side multiline done (C-] l toggle). Still needed: interpreter-level multiline (nabla/namespace protocol with SetPromptType type=3)
- **#6 Syntax highlighting** — `)` commands, `]` commands, `⎕` fns, `:Keywords`, glyphs

## Data browser
- [ ] **Cell type collapse on edit** — APL has no "complex slot": typing `5` into a cell that was `5J3` saves and reloads as int 5, because the interpreter collapses `5J0`→`5` even when written as an APLAN literal (verified with `foo←(x:5J0)` → `⎕DR foo.x` is 83/INT). Same situation for float→int when the value is whole. The data browser presents typed cells but the wire format doesn't honor that. Possible directions (none designed yet): show cell type as a glyph or colour so the disappearance is visible feedback; warn in the status bar when an edit will narrow on save; let users explicitly "lock" a cell as complex/float (would require a Go-side wrapper that doesn't round-trip through the interpreter). Deferred — no design capacity right now.

## Protocol
- [ ] OptionsDialog proper UI — currently auto-replies "No", needs yes/no/cancel prompt
- [ ] StringDialog (text input dialog)
- [ ] HadError message handling (better error display)

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
