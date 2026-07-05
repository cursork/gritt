# apldap — Debug Adapter Protocol server for Dyalog APL

> **⚠️ Do not use this — use [vscode-apl](https://github.com/martanit/vscode-apl)
> instead. It is a far more well-developed project.** apldap is an
> experiment in bridging DAP to RIDE, kept here as a grittle.

Bridges VSCode (and any other DAP client) to Dyalog APL's RIDE protocol:
breakpoints, stepping, stack traces, variable inspection, and expression
evaluation from the Debug Console.

```
VSCode/DAP client <--DAP/JSON-RPC--> apldap <--RIDE--> Dyalog APL
```

## Layout

```
grittles/apldap/
├── main.go            # Entry point (stdio or -port TCP mode)
├── extension/         # Minimal VSCode extension shim
│   ├── package.json   # Registers apl language + debugger type
│   ├── extension.js   # Points VSCode at the bundled apldap binary
│   └── apldap         # Binary (build artifact, gitignored)
├── test-workspace/    # Sample workspace for extension development
│   ├── .vscode/       # launch.json (attach, loads ${file}) + build task
│   └── foo.aplf
└── scripts/           # start/stop-test-env.sh (Dyalog + multapl, ports 950x)
```

The library code lives in the main gritt tree: `dap/` (DAP protocol
types), `dap/adapter/` (the DAP↔RIDE bridge), `dap/daptest/` (test
client in the style of @vscode/debugadapter-testsupport). There is also
a raw manual test client at `cmd/test-dap`.

## Building

```bash
go build -o grittles/apldap/apldap ./grittles/apldap
cp grittles/apldap/apldap grittles/apldap/extension/
```

The VSCode build task in `test-workspace/.vscode/tasks.json` does both
steps and is wired as the `preLaunchTask`.

## Testing

Start the test environment:

```bash
./grittles/apldap/scripts/start-test-env.sh
go test ./dap/adapter
./grittles/apldap/scripts/stop-test-env.sh
```

### VSCode extension development

```bash
code --extensionDevelopmentPath=$PWD/grittles/apldap/extension $PWD/grittles/apldap/test-workspace
```

F5 to attach (the build task runs first). Open a `.aplf`/`.apln` file,
set gutter breakpoints, call functions from the Debug Console.

## What works

- Attach to Dyalog via RIDE (direct or through multapl)
- Debug Console evaluation
- Source loading on connect: `2⎕FIX'file://...'` of the active file
- Gutter breakpoints (file line → `⎕STOP`), function breakpoints,
  deferred breakpoints (queued until the program loads)
- Source display + current-line highlight when stopped
- Local variables (parsed from the tradfn header)
- Stack traces with file positions
- Step over / into / out, continue
- Detach leaves Dyalog running (Cutback + CloseWindow per tracer window;
  no RIDE Disconnect — see below)

## What doesn't work yet

Lots. Honestly.

## Key technical details

### Source mapping
- `parseSourceFile()` reads APL files, detects tradfn headers, records
  `funcName → {filePath, headerLine}`
- APL line 0 = header, line 1+ = body (1-based); blank lines and
  comments count
- RIDE's `currentRow` in OpenWindow is 0-indexed in the body (excludes
  header), so +1 to get the function line

### Breakpoint flow
1. VSCode sends `setBreakpoints` with file path + lines (BEFORE
   `configurationDone`)
2. Lines converted file-relative → function-relative via
   `fileLineToFuncLine`
3. If the program isn't loaded yet, breakpoints queue in `pendingStops`
   and replay after `2⎕FIX`

### Disconnect semantics
A DAP disconnect in attach mode must leave Dyalog running so the client
can detach/restart and reconnect. Sending a RIDE `Disconnect` message
ends the interpreter's session (it stops evaluating) — so the adapter
only sends `Cutback` + `CloseWindow` per tracer window to unwind
suspensions, then closes its TCP connection. `)RESET`/`)SIC` are
deliberately avoided.

### Variable evaluation
When stopped, the tradfn header is parsed for names (result, args,
locals after `;`); each is evaluated with a `:If 0<⎕NC` guard for
undefined names, via the same Execute mechanism as the Debug Console.

## Debug logging

In stdio mode (as VSCode runs it), logs go to `/tmp/apldap.log`.
