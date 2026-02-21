# OPERANDA - Current State

## APL keycode support (BK/FD/SR style)

APLers expect standard keycodes like BK (Back), FD (Forward), SR (Redraw Screen). The Dyalog keycode table maps actions to keystrokes (see `$DYALOG/aplkeys/xterm`). We should support these as configurable aliases — e.g. allowing `"history_back": ["ctrl+shift+up", "BK"]` in gritt.json so APLers can use their muscle-memory bindings where terminal capabilities allow.

Terminal limitation: ctrl+shift+backspace and ctrl+shift+enter (BK/FD defaults) are not possible — backspace/enter are raw ASCII control chars that can't carry modifier info via escape sequences. We use ctrl+shift+up/down instead.

## Recent work

Implemented issues #15 (history paging with ctrl+shift+up/down), #9 (clear screen with ctrl+l), #7 (focus mode with C-] f). All three are client-side, touch tui.go/keys.go/config.go/gritt.default.json. All available via command palette (C-] :) with configured keybinding hints.

See FACIENDA.md for what's next.
