# aplsock / Prepl: APL Socket Server

Pure APL socket server inspired by Clojure's prepl. Accepts expressions over TCP, returns tagged JSON with APLAN values. Intended to eventually replace RIDE as gritt's primary connection method.

## Architecture

```
Client (gritt/nc/telnet) → aplsock (Go, serves on -sock) → APL prepl (in Dyalog, internal port)
                                ↑ bootstrapped via RIDE, then RIDE dropped
```

- `prepl/Prepl.apln` — APL namespace: Conga TCP server, eval in `#` context, APLAN serialization
- `prepl/client.go` — Go client library: `Connect`, `Eval` (parsed), `EvalRaw` (proxy)
- `prepl/embed.go` — `go:embed` of the APL source
- `grittles/aplsock/` — standalone binary: bootstrap + proxy server

## Open Decisions

### ⎕← Capture (parked)
The `out` stream tag is reserved but not implemented. Capturing `⎕←` output requires APL-side work — it's a side effect to the session, not a capturable return value. Solution will be on the APL side.

### Multi-Line Expressions
Current protocol: one expression per newline. Multi-line constructs (`:Namespace`/`:EndNamespace`, nabla definitions) need a framing mechanism. Options: length-prefixed messages, explicit begin/end markers, or a "raw mode" toggle.

### Multi-Connection Support
APL prepl v1 uses a single shared buffer — one connection at a time. aplsock serializes concurrent clients through one prepl.Client (mutex). For real multi-connection: per-connection buffers in APL (keyed by Conga object name), potentially threaded execution.

### gritt as Client (phase 2)
gritt would add `-prepl addr` to connect to a running aplsock as an alternative to RIDE. The TUI would need adapting — the prepl protocol is simpler than RIDE (no editors, tracing, etc. yet). Either the prepl protocol grows, or those features get implemented in APL.

### Execution Context
`Eval` uses `:With #` to execute in the root namespace. `⎕IO`, `⎕ML`, etc. inherit from `#`'s settings — should the prepl set/restore these?

### APLAN Serialization Completeness
`ToAPLAN` handles scalars, vectors, matrices, namespaces, and nested arrays. Not handled:
- Higher-rank arrays (≥3D) — falls back to `⍕`
- Nested matrices — best-effort, may lose structure
- Special values (`⎕NULL`, refs, etc.)

May be replaceable with a system function if Dyalog adds one.
