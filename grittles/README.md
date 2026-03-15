# Grittles

Choose your own reason for the name: 'gritt tools', 'little gritts' or
'gritt-less' (gritt without gritt).

Standalone CLI tools built on gritt's libraries. Each is its own binary.

## Tools

### aplan2json

Reads APLAN from stdin, writes JSON to stdout.

```
echo '(1 2 3)' | aplan2json
echo '(name: ''Bob'' age: 42)' | aplan2json
aplan2json -lossy < shaped.aplan    # nested arrays, no shape metadata
```

### json2aplan

Reads JSON from stdin, writes APLAN to stdout.

```
echo '[1,2,3]' | json2aplan
echo '{"a":1}' | json2aplan
json2aplan -lossy < data.json       # skip shape reconstruction
```

### aplcart

Search APLcart entries from the terminal.

```
aplcart "matrix inverse"
aplcart -refresh                    # force cache update from GitHub
```

Shares cache with gritt at `~/.cache/gritt/aplcart.db`.

### apldocs

Search Dyalog documentation from the terminal.

```
apldocs "each"
apldocs -refresh                    # download latest docs DB
```

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
go build ./grittles/aplan2json
go build ./grittles/json2aplan
go build ./grittles/aplcart
go build ./grittles/apldocs
go build ./grittles/aplfmt
go build ./grittles/aplmcp
```

Or all at once:

```
go build ./grittles/...
```