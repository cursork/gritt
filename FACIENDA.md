# FACIENDA - Things to be done

## aplsock / Prepl
- [ ] **gritt as client (phase 2)** — `-prepl addr` connects to aplsock instead of RIDE
- [ ] **⎕← capture** — output stream (`tag: 'out'`). Solution is on the APL side.
- [ ] **Multi-line expressions** — framing for `:Namespace`/`:EndNamespace`, nabla
- [ ] **Multi-connection** — per-connection buffers on APL side (currently single `_buf`)
- [ ] **System commands** — `)ts`, `)vars` etc. may not serialize via `Serialise`
- [x] **aplsock transport modes** — `-mode plain`, `-mode aplan` (default), `-mode aplor` (220⌶ binary)
- [x] **`Unmarshal` namespace support** — `amicable.Unmarshal` returns `*codec.Namespace` for namespace blobs. Variable members extracted as typed Go values, function members as opaque `Raw` bytes. aplor mode scalar/string/error tests pass.

## amicable
- [ ] **decompiler: extend** — multi-line dfns, more system variables, tradfn string literals/locals, embedded function decompilation (different encoding from standalone ⎕OR — see §5.7), nested namespaces
- [ ] **bytecode synthesis** — generate/modify ⎕OR bytecode in Go, send to Dyalog via `0(220⌶)`
- [ ] **nested-namespace unmarshal: end-of-sub-blob detection** — current fix handles class-9 members at end of extraction order (i.e. earliest in name-table order). When a sub-namespace appears later in name-table order than another member, `pos` is left at the sub-blob start and subsequent variable extraction re-finds content from inside the nested ns. Need a way to determine sub-blob byte length (likely by recursively walking and counting trailing settings/translation/workspace `07 D5 50` blocks).
- [ ] **other 9.x classes** — instances (9.2), classes (9.4), interfaces (9.5), external classes (9.6) all hit the same code path as 9.1 namespaces but have different blob shapes. Currently the recursive `unmarshalNamespace` may misparse them.
- [ ] **generative round-trip tests** — randomly construct namespace/array structures in APL via gritt, capture the 220⌶ blob, unmarshal, re-marshal, and compare. Would surface boundary cases (deep nesting, large fan-out, mixed types per member) without hand-writing every shape. Drives both the bug fix above and confidence in marshal symmetry.

## GitHub Issues
- **#3 Multithreaded tracing** — switch between suspended functions in different threads
- **#4 Inline tracing** — `IT` command: left/right args, current fn, axis spec, previous result
- **#5 Proper multiline mode** — basic client-side multiline done (C-] l toggle). Still needed: interpreter-level multiline (nabla/namespace protocol with SetPromptType type=3)
- **#6 Syntax highlighting** — `)` commands, `]` commands, `⎕` fns, `:Keywords`, glyphs

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
