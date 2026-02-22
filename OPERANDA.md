# OPERANDA - Current State

Core TUI is functional with session, editors, tracer, variables, autocomplete, docs, command palette, focus mode, history paging, clear screen, save/load session.

## APL keycode support (BK/FD/SR style)

APLers expect standard keycodes like BK (Back), FD (Forward), SR (Redraw Screen). The Dyalog keycode table maps actions to keystrokes (see `$DYALOG/aplkeys/xterm`). We should support these as configurable aliases.

Terminal limitation: ctrl+shift+backspace and ctrl+shift+enter (BK/FD defaults) are not possible — backspace/enter are raw ASCII control chars that can't carry modifier info via escape sequences. We use ctrl+shift+up/down instead.

## Recent

- **Dyalog discovery**: `-l` flag now auto-discovers Dyalog from standard install paths when not in `$PATH`. New `-version` flag to target specific version (e.g. `-version 20.0`). Searches macOS `/Applications/Dyalog-*.app/`, Linux `/opt/mdyalog/`, Windows `Program Files` + `%LOCALAPPDATA%`. Implementation in `dyalog.go`, tests in `dyalog_test.go`.
- Variable editing: read-only numeric arrays convert to editable APLAN via Enter (ShowAsArrayNotation protocol message)
- Load session: command palette `load` with smart default to most recent `session-*` file

See FACIENDA.md for what's next.
