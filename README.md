# gritt

![grhorse](logo-small.jpg)

A terminal IDE for Dyalog APL.

Pronounced like "grit" (G from Go + German "Ritt" = ride).

## WARNING

This is an *alpha* level project. I will promote it to 0.1.0 when I think it's ready for broad use. **Here be dragons**.

I use it daily, but mostly for specific tasks. I revert to Ride otherwise, since it's complete.

## Features

- Full TUI with floating panes for editors, tracer, debug info
- APL input: backtick prefix (`` `i `` → `⍳`), symbol search, APLcart integration
- Debugging: breakpoints, stepping (into/over/out), stack trace, variables pane, edit while debugging (very much a 'maybe' - don't trust it)
- Command palette for quick access to all commands
- Connection resilience - stays alive on disconnect, allows reconnect
- Single-expression and stdin modes for scripting
- Link integration for source-controlled APL projects
- Tracer with stack navigation (single pane, not overlapping windows)

See [example-test-report.html](example-test-report.html) or [example-test-report.txt](example-test-report.txt) for a walkthrough of features (snapshots from automated tests).

## Installation

### Download

Grab a binary from [Releases](https://github.com/cursork/gritt/releases):

- `gritt-darwin-arm64` - macOS Apple Silicon
- `gritt-darwin-amd64` - macOS Intel
- `gritt-linux-arm64` - Linux ARM64
- `gritt-linux-amd64` - Linux x86_64

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
- tmux (for running tests)

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

### Format APL files

```bash
# Format files in place (prints changed filenames)
./gritt -l -fmt file1.aplf file2.aplf utils.apln
```

Works on function files (`.aplf`) and namespace/class files (`.apln`). Uses Dyalog's `FormatCode` to normalize whitespace and indentation. Also available in the TUI via the command palette (`Ctrl+]` `:` → `format`).

## Caching

gritt caches APLcart idioms and Dyalog documentation locally for fast offline access. Caches live in your OS cache directory (`~/Library/Caches/gritt/` on macOS, `~/.cache/gritt/` on Linux).

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
2. `~/.config/gritt/gritt.json` (user)
3. Embedded default

These are not merged - first found, wins. Override with `-cfg path` to load a specific file, or `-cfg ''` for embedded defaults only.

The `accent` field sets the UI accent color (borders, highlights, selections). Default is Dyalog orange (`#F2A74F`). For a neutral grey:

```json
{
  "accent": "#808080"
}
```

Any `#RRGGBB` hex color works. Omit or leave empty for the default.

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

## LLM Usage

This is a Claude Code project; unashamedly so. I have left all of the artifacts of that work in the project. I am prepared to take PRs generated by Claude, so they might be convenient references to initialise a new Claude for someone.

## License

MIT
