# gritt

A terminal IDE for Dyalog APL.

Pronounced like "grit" (G from Go + German "Ritt" = ride).

## Features

- Full TUI with floating panes for editors, tracer, debug info
- Single-expression and stdin modes for scripting
- Link integration for source-controlled APL projects
- Tracer with stack navigation (single pane, not overlapping windows)

## Requirements

- Go 1.21+
- Dyalog APL with RIDE enabled
- tmux (for running tests)

## Build

```bash
go build -o gritt .
```

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
| C-] ? | Show key mappings |
| C-] d | Toggle debug pane |
| C-] s | Toggle stack pane |
| C-] q | Quit |
| Tab | Cycle pane focus |
| Esc | Close pane / pop tracer frame |

## Testing

```bash
go test -v -run TestTUI
```

Requires Dyalog and tmux. Tests run in a tmux session and generate HTML reports with screenshots in `test-reports/`. See [example-test-report.html](example-test-report.html) for sample output.

## Debugging

```bash
./gritt -log debug.log
```

Logs all RIDE protocol messages and TUI state changes.

