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
```

Or all at once:

```
go build ./grittles/...
```
