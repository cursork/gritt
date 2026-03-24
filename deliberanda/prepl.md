# aplsock / Prepl: APL Socket Server

Pure APL socket server inspired by Clojure's prepl. Accepts expressions over TCP, returns tagged APLAN namespaces. Intended to eventually replace RIDE as gritt's primary connection method.

## Architecture

```
Client (gritt/nc/tooling) → aplsock (Go, -sock) → APL prepl (Conga, internal port)
                                 ↑ bootstrapped via RIDE, RIDE kept alive for drain
```

## Protocol (pure APLAN)

Input: APL expression, newline-terminated. Optional `⍝ID:uuid` trailing comment for correlation.

```
→ ⍳5
← (tag: 'ret' ⋄ val: 1 2 3 4 5)

→ ⍳5 ⍝ID:019abc12-3456-7890-abcd-ef1234567890
← (id: '019abc12-3456-7890-abcd-ef1234567890' ⋄ tag: 'ret' ⋄ val: 1 2 3 4 5)

→ ÷0
← (tag: 'err' ⋄ en: 11 ⋄ message: 'Divide by zero' ⋄ dm: (...))

→ x←42
← (tag: 'ret' ⋄ val: 42)

→ f←{⍺+⍵}
← (tag: 'ret')
```

Default mode: raw APLAN passthrough (for tooling). `-repl` mode: decoded plain text (for interactive use).

## Open Decisions

### ⎕← Capture (parked)
`⎕←` output goes to RIDE drain, not returned to client. Solution will be on the APL side. The `out` tag is reserved for future use.

### Multi-Line Expressions
One expression per newline. Multi-line constructs need a framing mechanism.

### Multi-Connection Support
Single `_buf` on APL side — one connection at a time per prepl. aplsock serializes via mutex.

### gritt as Client (phase 2)
gritt adds `-prepl addr` to connect to aplsock. TUI would use `prepl.Client` instead of `ride.Client`.

### System Commands
`)ts`, `)vars` etc. may not serialize cleanly via `⎕SE.Dyalog.Array.Serialise`.

### 220⌶ / 219⌶ (binary serialization)
Alternative to APLAN — binary array serialization. Would need a new Go parser. Not pursued yet but noted.
