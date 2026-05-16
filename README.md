# gritt

![grhorse](logo-small.jpg)

A RIDE protocol client for Dyalog APL — terminal IDE, scripting CLI, and Go libraries.

Pronounced like "grit" (G from Go + German "Ritt" = ride).

## WARNING

This is an *alpha* level project. I will promote it to 0.1.0 when I think it's ready for broad use. **Here be dragons**.

I use it daily, but mostly for specific tasks. I revert to Ride otherwise, since it's complete.

## Features

gritt is a scripting CLI, a terminal IDE, and a small collection of reusable Go
libraries underneath both.

### Command line

- `-l` launches a dyalog instance with RIDE and connects to it
- `-e` runs a single expression and exits:
  - `gritt -l -e "⍳5"` - launch and run expressions
  - `gritt -e ... -e ...` - execute expressions against default 4502 port
  - `gritt -addr localhost:9502 -e ...` - execute expressions against any running Dyalog
- `-sock` opens a socket alongside the running session so external
  scripts can inject expressions into the same Dyalog the TUI is using
- `-history` dumps cross-session command history: `gritt -history | grep …`

### Terminal UI

- Discoverable — `Ctrl+]` `:` opens a fuzzy-searchable list of every
  command in gritt
- Structured data browser — drill into namespaces, matrices, vectors;
  edit cells inline; saves back as APLAN
- Inline search for APLcart idioms, Dyalog docs (`F1`), APL symbols, and
  I-beam reference
- Tracer with single-pane stack navigation, breakpoints, step
  into/over/out, edit-while-debugging
- Edit functions/namespaces/arrays in your `$EDITOR` — saved back to the
  interpreter on exit
- APL input via backtick prefix (`` `i `` → `⍳`) — no system keyboard
  remap required
- Auto-launch Dyalog (`-l`) — discovers installed versions

### Library + grittles

gritt libraries can be consumed on their own; import the packages directly to drive
Dyalog from your own code:

- `codec/` — APLAN parser/serializer
- `amicable/` — `220⌶` binary marshal/unmarshal
- `ride/` — RIDE protocol client
- `prepl/` — APL 'socket server' - alternative to RIDE, runs in-interpreter. See [aplsock](https://github.com/cursork/gritt/tree/main/grittles#aplsock)

Several utilities ship as standalone binaries — see
[grittles/README.md](grittles/README.md). If your use case is narrow, one of them
may be more appropriate for you.

See [example-test-report.html](example-test-report.html) or [example-test-report.txt](example-test-report.txt) for a walkthrough of features (snapshots from automated tests).

## Installation

### Download

Grab a binary from [Releases](https://github.com/cursork/gritt/releases):

- `gritt-darwin-arm64` - macOS Apple Silicon
- `gritt-darwin-amd64` - macOS Intel
- `gritt-linux-arm64` - Linux ARM64
- `gritt-linux-amd64` - Linux x86_64
- `gritt-windows-amd64.exe` - Windows x86_64
- `gritt-windows-arm64.exe` - Windows ARM64

**Windows note:** SmartScreen will block the download (click "Keep" in Edge, or "Keep anyway" in the dialog). Defender may also flag the binary — right-click → Properties → Unblock. Both are false positives from unsigned Go binaries.

### go install

```
go install github.com/cursork/gritt
```

### Build from source

Requires Go 1.21+:

```bash
go build -o gritt .
```

## Requirements

- Dyalog APL with RIDE enabled
- macOS, Linux, or Windows
- tmux (for running tests — macOS/Linux only)

## Usage

### Interactive TUI

Auto-launch Dyalog and connect (discovers installed versions automatically):
```bash
./gritt -l                          # Launch highest installed version
./gritt -l -version 20.0            # Launch specific version
./gritt -l -version /path/to/dyalog # Launch specific binary
```

Or connect to an existing Dyalog instance:
```bash
RIDE_INIT=SERVE:*:4502 dyalog +s -q  # Start Dyalog first
./gritt                               # Then connect
```

### Non-interactive

```bash
# Single expression (with auto-launch)
./gritt -l -e "⍳5"

# Single expression (connect to existing Dyalog)
./gritt -e "⍳5"

# Pipe from stdin
echo "1+1" | ./gritt -l -stdin

# Link a directory first
./gritt -l -link /path/to/src -e "MyFn 42"
```

### Socket injection (`-sock`)

Open a socket alongside a running gritt session that lets external clients inject expressions into it. Useful when you want to drive the same Dyalog session from multiple places — a script, another terminal, an editor — without having to juggle separate `multapl`/`gritt`/`dyalog` processes.

```bash
# TCP — bare port, ":port", or "host:port"
./gritt -l -sock :12345
./gritt -l -sock 12345           # equivalent

# Unix socket — any path with a "/"
./gritt -l -sock /tmp/gritt.sock
```

Each connection reads newline-delimited APL expressions; gritt executes them in the same session as the TUI and writes the captured output back to the connection. Both the injected expression and its output are mirrored into the on-screen session so you can see what an external client just ran.

```bash
$ echo '1+2' | nc -N localhost 12345
3
$ echo 'x←⍳5 ⋄ +/x' | nc -N localhost 12345
15
```

Multiple clients can connect simultaneously; expressions are queued and run one at a time as the interpreter goes idle, interleaved with whatever the TUI is doing.

The response format is the rendered session output — same display text you'd see in the TUI. For structured (parseable) responses use `aplsock` (`grittles/aplsock/`), which speaks APLAN or `220⌶` binary instead.

### Format APL files

```bash
# Format files in place (prints changed filenames)
./gritt -l -fmt file1.aplf file2.aplf utils.apln
```

Works on function files (`.aplf`) and namespace/class files (`.apln`). Uses Dyalog's `FormatCode` to normalize whitespace and indentation. Also available in the TUI via the command palette (`Ctrl+]` `:` → `format`).

### Command history

```bash
./gritt -history              # Dump full history
./gritt -history | tail -5    # Last 5 commands
./gritt -history | grep foo   # Search
```

History is an append-only log at `~/.cache/gritt/history` (`~/Library/Caches/gritt/history` on macOS). Both TUI sessions and `-e` expressions are recorded. Multiple concurrent gritt processes append safely.

## Caching

gritt caches APLcart idioms and Dyalog documentation locally for fast offline access. Caches live in your OS cache directory (`~/Library/Caches/gritt/` on macOS, `~/.cache/gritt/` on Linux, `%LocalAppData%\gritt\` on Windows).

Nothing is downloaded until you use a feature that needs it:
- **APLcart** (`Ctrl+]` `:` → `aplcart`): downloads on first open
- **Docs** (`F1` or `Ctrl+]` `:` → `docs`): downloads on first open

Caches auto-refresh in the background after 7 days (serving stale data instantly while updating). To force an immediate refresh: `Ctrl+]` `:` → `cache-refresh`.

## Key Bindings

Leader key: `Ctrl+]` (keeps other keys free for APL input, and I figured it wouldn't interfere with muscle memory)

See [KEYBINDINGS.md](KEYBINDINGS.md) for full reference. When in doubt: `Ctrl+]` then `:` brings up a pane with commands to choose from.

## Configuration

gritt looks for `gritt.json` in order:
1. `./gritt.json` (local)
2. `~/.config/gritt/gritt.json` (user — `%USERPROFILE%\.config\gritt\` on Windows)
3. Embedded default

These are not merged - first found, wins. Override with `-cfg path` to load a specific file, or `-cfg ''` for embedded defaults only.

Key bindings are configured via `bindings` (commands) and `navigation` (input primitives). Any command can be bound as leader-prefixed or direct:

```json
{
  "bindings": {
    "leader":    { "keys": ["ctrl+]"] },
    "debug":     { "keys": ["d"], "leader": true },
    "doc-help":  { "keys": ["f1"] },
    "step-into": { "keys": ["i"], "context": "tracer" },
    "symbols":   {}
  },
  "navigation": {
    "up": ["up"], "down": ["down"], "execute": ["enter"]
  }
}
```

The old `keys` + `tracer_keys` format is automatically migrated on load. See [KEYBINDINGS.md](KEYBINDINGS.md) for full details.

## Testing

```bash
go test -v -run TestTUI
```

Requires Dyalog and tmux. Tests run in a tmux session and generate HTML reports with screenshots in `test-reports/`.

## Debugging

```bash
./gritt -log debug.log
```

Logs RIDE protocol messages and TUI state changes.

## Sandboxing (Claude Code)

Some users want to run under a sandbox; however, gritt is often used to launch
a separate Dyalog process, so this is the setup I now use for the purpose of a
functioning `gritt -l` inside Claude.

gritt works inside Claude Code's sandbox with one config inside your project's
`.claude/settings.local.json`:

```json
{
  "sandbox": {
    "enabled": true,
    "autoAllowBashIfSandboxed": false,
    "allowUnsandboxedCommands": false,
    "network": {
      "allowLocalBinding": true
    }
  }
}
```

`allowLocalBinding` is required because gritt communicates with Dyalog via the
RIDE protocol over loopback TCP. Without it, connection is blocked.

Optionally, if confident about the sandboxing, add gritt to the permission allow
list so it runs without prompting:

```json
{
  "permissions": {
    "allow": ["Bash(gritt:*)"]
  }
}
```

### What the sandbox constrains:

The sandbox operates at the OS level, so it applies to everything gritt and Dyalog do — including `⎕SH`, `⎕NPUT`, `⎕NDELETE`, and other system functions:

| Operation | Result |
|-----------|--------|
| Read (`⎕NGET`, `⎕SH 'cat ...'`) | Allowed |
| Write (`⎕NPUT`, `⎕SH 'echo > ...'`) | Blocked outside allowed dirs |
| Delete (`⎕NDELETE`, `⎕SH 'rm ...'`) | Blocked outside allowed dirs |

This means Dyalog can read the filesystem freely but cannot write or delete outside the sandbox's allowed directories. The constraint applies regardless of whether the operation comes from APL system functions or shell commands via eg `⎕SH`.

### Disallowing reads

Use the following in your `.claude/settings.local.json` file for your project:

```json
{
  "filesystem": {
    "denyRead": ["/some/path"]
  }
}
```

## LLM Usage

This is a Claude Code project; unashamedly so. I have left all of the artifacts of that work in the project. I am prepared to take PRs generated by Claude, so they might be convenient references to initialise a new Claude for someone.

## License

MIT
