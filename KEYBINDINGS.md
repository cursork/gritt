# Key Bindings

**NOTA BENE:** gritt **intentionally** breaks with APL tradition. The goal is to make all of this configurable _and_ to be able to offer keybindings familiar to long-time APL programmers. For now, the bindings are based purely on the whim of [cursork](https://github.com/cursork). He would be happy for a PR for alternative bindings, but also likes his setup as below...

Leader key: `Ctrl+]` (keeps all keys free for APL input)

## Global Keys

| Key | Action |
|-----|--------|
| Enter | Execute current line |
| C-] d | Toggle debug pane |
| C-] s | Toggle stack pane |
| C-] l | Toggle multiline mode (Enter adds lines, toggle off sends) |
| C-] v | Toggle variables pane (~ toggles [local]/[all]) |
| C-] b | Toggle breakpoint (in editor/tracer) |
| C-] : | Command palette |
| C-] m | Pane move mode |
| C-] r | Reconnect to Dyalog |
| C-] e | Open focused editor pane in `$EDITOR` |
| C-] ? | Show key mappings |
| C-] q | Quit (with confirmation) |
| Tab | Cycle pane focus |
| Esc | Close pane / exit mode / pop tracer frame |
| Ctrl+C | Shows "Type C-] q to quit" hint |

## Navigation

| Key | Action |
|-----|--------|
| Up/Down | Navigate lines |
| Left/Right | Move cursor |
| Home/End | Start/end of line |
| PgUp/PgDn | Scroll page |
| Ctrl+R | Search command history (overlay pane) |

## Tracer Keys (when tracer pane focused)

Single-key commands in tracer mode (no leader needed):

| Key | Action | RIDE Message |
|-----|--------|--------------|
| n / Enter | Step over (next line) | RunCurrentLine |
| i | Step into | StepInto |
| o | Step out | ContinueTrace |
| c | Continue execution | Continue |
| r | Resume all threads | RestartThreads |
| p | Trace backward | TraceBackward |
| f | Trace forward (skip) | TraceForward |
| e | Enter edit mode | (local toggle) |
| Esc | Exit edit mode / pop frame | CloseWindow |

## Editor Keys

| Key | Action |
|-----|--------|
| C-] b | Toggle breakpoint on current line |
| C-] e | Edit in `$EDITOR` (vim, emacs, code, …) and save on exit |
| Esc | Save and close |

### External editor (`C-] e`)

Writes the focused pane's text to a temp file (`.aplf`/`.apln`/`.apla` per entity type), launches `$EDITOR <file>`, and on exit reads the file back. If the content changed, `SaveChanges` is sent to Dyalog so the new body is persisted. Falls back to `vi` if `$EDITOR` is unset.

For VS Code, set `EDITOR="code --wait"` — without `--wait`, the `code` binary returns before you've finished editing.

Refused on tracer panes (in trace mode) and read-only value windows — a red transient error in the status line tells you which key to press first (`e` to enter tracer edit mode, `Enter` to convert a read-only value to APLAN).

## Variables Pane Keys

| Key | Action |
|-----|--------|
| Up/Down | Select variable |
| Enter | Open variable in editor |
| ~ | Toggle [local]/[all] mode (• marks locals in all mode) |
| Esc | Close pane |

## APL Input

**Backtick prefix**: Press `` ` `` then a key:

| Input | Symbol | Name |
|-------|--------|------|
| `` `i `` | `⍳` | iota |
| `` `r `` | `⍴` | rho |
| `` `a `` | `⍺` | alpha |
| `` `w `` | `⍵` | omega |
| `` `o `` | `∘` | jot |
| `` `e `` | `∊` | epsilon |
| `` `1 `` | `¨` | each |
| `` `/ `` | `⌿` | replicate first |
| `` `\ `` | `⍀` | expand first |

Use `C-] :` → `symbols` to search all APL symbols by name.

## Pane Move Mode (C-] m)

| Key | Action |
|-----|--------|
| Arrows | Move pane |
| Shift+Arrows | Resize pane |
| Esc / Enter | Exit move mode |

## Command Palette

Press `C-] :` to open. Type to filter, Enter to select. All commands are available here.

The filter matches against the command name, hidden synonyms, and the help text — in that order. Synonyms aren't shown in the list, but typing one surfaces the command: e.g. `vim` finds `external-edit`, `idiom` finds `aplcart`, `callstack` finds `stack`, `bp` finds `breakpoint`.

## Configuration

Key bindings are configured in `gritt.json` using the `bindings` + `navigation` format:

```json
{
  "bindings": {
    "leader":          { "keys": ["ctrl+]"] },
    "debug":           { "keys": ["d"], "leader": true },
    "stack":           { "keys": ["s"], "leader": true },
    "doc-help":        { "keys": ["f1"] },
    "clear":           { "keys": ["ctrl+l"] },
    "symbols":         {},
    "step-into":       { "keys": ["i"], "context": "tracer" }
  },
  "navigation": {
    "up": ["up"], "down": ["down"],
    "execute": ["enter"]
  }
}
```

- **`"leader": true`** — requires leader prefix. Absent/false = direct (always available).
- **`"context": "tracer"`** — only active when tracer pane is focused in tracer mode.
- **`{}`** — command exists (appears in palette) but has no keybinding.
- **`navigation`** — separate section for input primitives (arrow keys, backspace, etc.).
- Command names use kebab-case everywhere.

The old `keys` + `tracer_keys` format is automatically migrated on load.

Config lookup order:
1. `./gritt.json` (local)
2. `~/.config/gritt/gritt.json` (user — `%USERPROFILE%\.config\gritt\` on Windows)
3. Embedded default

### Runtime Rebinding

Open command palette (`C-] :`) → `rebind` to interactively change keybindings. Changes are ephemeral (session only) until saved.

### Saving Config

Open command palette → `save-config` to write the current config (including any rebindings) to disk. If `./gritt.json` or the user config exists, it overwrites that file. If neither exists, prompts to choose [l]ocal or [g]lobal.
