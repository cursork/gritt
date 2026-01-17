# Key Bindings

**NOTA BENE:** gritt **intentionally** breaks with APL tradition. The goal is to make all of this configurable _and_ to be able to offer keybindings familiar to long-time APL programmers. For now, the bindings are based purely on the whim of [cursork](https://github.com/cursork). He would be happy for a PR for alternative bindings, but also like his setup as below...

Leader key: `Ctrl+]` (keeps all keys free for APL input)

## Global Keys

| Key | Action |
|-----|--------|
| Enter | Execute current line |
| C-] d | Toggle debug pane |
| C-] s | Toggle stack pane |
| C-] b | Toggle breakpoint (in editor/tracer) |
| C-] : | Command palette |
| C-] m | Pane move mode |
| C-] r | Reconnect to Dyalog |
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
| Esc | Save and close |

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

Press `C-] :` to open. Type to filter, Enter to select:

| Command | Action |
|---------|--------|
| debug | Toggle debug pane |
| stack | Toggle stack pane |
| breakpoint | Toggle breakpoint |
| keys | Show key bindings |
| symbols | Search APL symbols |
| aplcart | Search APLcart idioms |
| reconnect | Reconnect to Dyalog |
| save | Save session to file |
| close-all-windows | Clear stuck editors/tracers |
| quit | Quit gritt |

## Configuration

Key bindings can be customized in `gritt.json`:

```json
{
  "keys": {
    "leader": ["ctrl+]"],
    "toggle_debug": ["d"],
    "toggle_stack": ["s"],
    ...
  },
  "tracer_keys": {
    "step_over": "n",
    "step_into": "i",
    ...
  }
}
```

Config lookup order:
1. `./gritt.json` (local)
2. `~/.config/gritt/gritt.json` (user)
3. Embedded default
