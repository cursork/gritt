# FACIENDA - Things to be done

## aplsock / Prepl
- [ ] **gritt as client (phase 2)** — `-prepl addr` connects to aplsock instead of RIDE
- [ ] **⎕← capture** — output stream (`tag: 'out'`). Solution is on the APL side.
- [ ] **Multi-line expressions** — framing for `:Namespace`/`:EndNamespace`, nabla
- [ ] **Multi-connection** — per-connection buffers on APL side (currently single `_buf`)
- [ ] **System commands** — `)ts`, `)vars` etc. may not serialize via `Serialise`
- [x] **aplsock transport modes** — `-mode plain`, `-mode aplan` (default), `-mode aplor` (220⌶ binary)
- [ ] **`Raw.Members()` API** — programmatic access to namespace members from `amicable.Raw` blobs. Currently only accessible via `Decompile()` (text). Needed for aplor consumers to extract `tag`/`val` from response namespaces without string parsing.

## amicable
- [ ] **decompiler: extend** — multi-line dfns, more system variables, tradfn string literals/locals, namespace member-value ordering for mixed namespaces, nested namespaces
- [ ] **bytecode synthesis** — generate/modify ⎕OR bytecode in Go, send to Dyalog via `0(220⌶)`

## GitHub Issues
- **#3 Multithreaded tracing** — switch between suspended functions in different threads
- **#4 Inline tracing** — `IT` command: left/right args, current fn, axis spec, previous result
- **#5 Proper multiline mode** — explicit multiline input, required for namespaces/classes
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

---

## Notes

### Session Behavior
The Dyalog session is append-only from the interpreter's perspective. Client shows editable history, but executing always appends.

### RIDE Protocol — Not Yet Implemented
- OptionsDialog — auto-replies "No", needs proper UI
- StringDialog — not handled
- HadError — not handled
