# Grittles

Choose your own reason for the name: 'gritt tools', 'little gritts' or
'gritt-less' (gritt without gritt).

Standalone CLI tools built on gritt's libraries. Each is its own binary.

## Tools

### aplanconv

Converts between APLAN and JSON. Auto-detects input format from the first
non-whitespace character (`(` `'` `⍬` `¯` → APLAN; `{` `"` → JSON).
Ambiguous cases like `[` (used by both formats) guess JSON first, then
try APLAN if parsing fails. Use `-from`/`-to` to override detection with
explicit formats.

```
echo '(1 2 3)' | aplanconv          # APLAN → JSON (auto-detected)
echo '[1,2,3]' | aplanconv          # JSON → APLAN (auto-detected)
echo '[1 2 ⋄ 3 4]' | aplanconv     # APLAN matrix (JSON fails, APLAN succeeds)
aplanconv data.aplan                # read from file
aplanconv -from aplan -to json      # explicit formats
aplanconv -lossy < shaped.aplan     # nested arrays, no shape metadata
```

Flags: `-from FORMAT`, `-to FORMAT` (aplan or json), `-lossy`.

### aplcart

Search APLcart entries from the terminal.

```
aplcart "matrix inverse"
aplcart -refresh                    # force cache update from GitHub
```

Shares cache with gritt at `~/.cache/gritt/aplcart.db`.

### apldocs

Search and read Dyalog documentation from the terminal.

```
apldocs "each"                      # search and display first match
apldocs -list "each"                # list all matching titles
apldocs -n 2 "each"                 # display 2nd match
apldocs -refresh                    # download latest docs DB
```

Renders markdown with glamour, same as gritt's doc pane.
Shares cache with gritt at `~/.cache/gritt/dyalog-docs.db`.

### aplfmt

Format APL source files using a Dyalog interpreter.

```
aplfmt file.aplf                    # auto-launches Dyalog
aplfmt -addr localhost:4502 *.aplf  # use running Dyalog
aplfmt -version 20.0 src/          # specific version, directory
```

### aplsock

Bootstrap a pure APL socket server inside Dyalog and serve the prepl
protocol over TCP or Unix sockets. Default mode returns raw APLAN for
tooling; `-repl` mode decodes to plain text.

```
aplsock -l -sock :4200              # launch Dyalog, serve on TCP 4200
aplsock -addr host:4502 -sock :4200 # connect to existing Dyalog
aplsock -l -sock /tmp/apl.sock      # Unix socket
aplsock -l -sock :4200 -repl        # plain text mode for interactive use
```

Protocol (raw mode — each response is a single-line APLAN namespace):

```
→ ⍳5
← (tag: 'ret' ⋄ val: 1 2 3 4 5)

→ 1÷0
← (tag: 'err' ⋄ en: 11 ⋄ message: 'Divide by zero' ⋄ dm: (...))

→ f←{⍺+⍵}
← (tag: 'ret')

→ ⍳3 ⍝ID:019abc12-3456-7890-abcd-ef1234567890
← (id: '019abc12-...' ⋄ tag: 'ret' ⋄ val: 1 2 3)
```

Optional `⍝ID:uuid` trailing comment for correlation — `⍎` ignores APL
comments, so the expression evaluates normally. IDs are mirrored in the
response for tooling to match requests with responses.

The APL server code (`prepl/Prepl.apln`) is also standalone-testable
without aplsock:

```apl
2 ⎕FIX 'file:///path/to/Prepl.apln'
Prepl.Start 4200
```

Tests: `grittles/aplsock/test.sh`

Flags: `-l` (launch Dyalog), `-addr HOST:PORT`, `-sock :PORT` or
`-sock /path`, `-version VERSION`, `-repl`.

### aplmcp

MCP server for LLM-driven APL interaction over stdio.

```json
{ "mcpServers": { "apl": { "command": "aplmcp" } } }
```

Tools: `launch`, `connect`, `disconnect`, `eval`, `batch`, `link`, `names`, `get`, `fix`, `alive`.

## Building

From the gritt root:

```
go build ./grittles/aplanconv
go build ./grittles/aplcart
go build ./grittles/apldocs
go build ./grittles/aplfmt
go build ./grittles/aplmcp
go build ./grittles/aplsock
```

Or all at once:

```
go build ./grittles/...
```
