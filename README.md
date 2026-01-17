# gritt

![Iris](logo.png)

A terminal IDE for Dyalog APL.

Pronounced like "grit" (G from Go + German "Ritt" = ride).

## Features

- Full TUI with floating panes for editors, tracer, debug info
- **APL input**: backtick prefix (`` `i `` → `⍳`), symbol search, APLcart integration
- Command palette for quick access to all commands
- Connection resilience - stays alive on disconnect, allows reconnect
- Single-expression and stdin modes for scripting
- Link integration for source-controlled APL projects
- Tracer with stack navigation (single pane, not overlapping windows)

## Installation

### Download

Grab a binary from [Releases](https://github.com/cursork/gritt/releases):

- `gritt-darwin-arm64` - macOS Apple Silicon
- `gritt-darwin-amd64` - macOS Intel
- `gritt-linux-arm64` - Linux ARM64
- `gritt-linux-amd64` - Linux x86_64

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

Start Dyalog with RIDE:
```bash
RIDE_INIT=SERVE:*:4502 dyalog +s -q
```

Connect with gritt:
```bash
./gritt
```

### Non-interactive

```bash
# Single expression
./gritt -e "⍳5"

# Pipe from stdin
echo "1+1" | ./gritt -stdin

# Link a directory first
./gritt -link /path/to/src -e "MyFn 42"
./gritt -link "#:." -e "⎕nl -3"
```

### Ephemeral Dyalog

The `apl` script starts a temporary Dyalog instance:
```bash
./apl "⍳5"
```

## Key Bindings

Leader key: `Ctrl+]`

| Key | Action |
|-----|--------|
| Enter | Execute line |
| C-] : | Command palette (search all commands) |
| C-] d | Toggle debug pane |
| C-] s | Toggle stack pane |
| C-] m | Pane move mode (arrows move, shift+arrows resize) |
| C-] r | Reconnect to Dyalog |
| C-] ? | Show key mappings |
| C-] q | Quit |
| Tab | Cycle pane focus |
| Esc | Close pane / exit mode / pop tracer frame |

### Command Palette

Press `C-] :` to open the command palette. Type to filter, Enter to select:
- `debug` - Toggle debug pane
- `stack` - Toggle stack pane
- `keys` - Show key bindings
- `symbols` - Search APL symbols by name
- `aplcart` - Search APLcart idioms
- `reconnect` - Reconnect to Dyalog
- `save` - Save session to file
- `quit` - Quit gritt

### APL Input

**Backtick prefix**: Press `` ` `` then a key to insert APL symbols:
- `` `i `` → `⍳` (iota)
- `` `r `` → `⍴` (rho)
- `` `a `` → `⍺` (alpha)
- `` `w `` → `⍵` (omega)
- `` `1 `` → `¨` (each)
- And many more...

**Symbol search**: `C-] :` → type "symbols" → search by name (iota, rho, each, grade, etc.)

**APLcart**: `C-] :` → type "aplcart" → search 3000+ APL idioms from aplcart.info

## Configuration

gritt looks for config files in order:
1. `./gritt.json` (local override)
2. `~/.config/gritt/gritt.json` (user config)
3. Embedded default (always available)

## Testing

```bash
go test -v -run TestTUI
```

Requires Dyalog and tmux. Tests run in a tmux session and generate HTML reports with screenshots in `test-reports/`.

## Debugging

```bash
./gritt -log debug.log
```

Logs all RIDE protocol messages and TUI state changes.

## License

MIT
