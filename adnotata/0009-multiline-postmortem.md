# 0009 — Multiline Input: Investigation Log

## Branch: `multiline-2-electric-boogaloo`

## Key Discovery: Nabla requires `[n]  ` line number prefixes

**RIDE MITM capture** (`/tmp/multiline.log`) shows exactly what RIDE sends for nabla:

```
→ Execute {"text":"      ∇r←Fn x\n"}       ← 6-space indent (opening line)
→ Execute {"text":"[1]  r←x×10\n"}          ← [1] prefix! NOT 6-space!
→ Execute {"text":"[2]  ∇\n"}               ← [2] prefix!
```

**Nabla body/close lines need `[n]  ` prefixes.** Namespace body lines use `"      "` (6-space indent). The interpreter SysErrors if you send nabla body with 6-space indent.

### Standalone test results (with `[n]  ` prefix fix)

All four patterns PASS individually on fresh Dyalog (`cmd/test-multiline/main.go`):
- Empty `:Namespace` — PASS (6-space indent for all lines)
- `:Namespace` with body — PASS (6-space indent for all lines)
- Nabla no body — PASS (`[1]  ∇\n` for close)
- Nabla with body — PASS (`[1]  r←x×10\n`, `[2]  ∇\n`)

Verified with and without `GetWindowLayout` + `SetPW` (when responses are properly drained).

### Prompt text comes from Dyalog

Dyalog sends the prompt via `AppendSessionOutput type=1` before `SetPromptType type=3`:
- For namespace: `"      "` (6 spaces)
- For nabla: `"[1]  "`, `"[2]  "`, etc.

The prompt text tells gritt what prefix to use for the next Execute.

### GetWindowLayout is NOT the problem

Initially suspected `GetWindowLayout` caused SysError. Testing showed:
- Without GWL + no drain: namespace works, nabla SysErrors (wrong prefix)
- With GWL + no drain: namespace SysErrors (undrained response confuses message stream)
- With GWL + proper drain (via `client.Execute("⍬")`): namespace works fine

The TUI's recv loop properly drains all responses, so GWL is not an issue in the TUI.

### BUT: TUI still SysErrors on nabla body lines

The TUI protocol log (`test-reports/protocol.log`) reveals a DIFFERENT prompt format:

```
[TUI]  ← AppendSessionOutput type=1 "      "     ← 6-space prompt for nabla!
[RIDE] ← AppendSessionOutput type=1 "[1]  "       ← [n] prompt for nabla
```

**Dyalog sends `"      "` to gritt but `"[1]  "` to RIDE for the same nabla operation.**

The difference: gritt sends `SetPW` (line 11 of protocol.log) on connect. RIDE does too, but something about gritt's init sequence causes Dyalog to use 6-space prompts instead of `[n]  ` line-numbered prompts for nabla. This needs investigation.

### The real problem (current state)

When Dyalog sends `"      "` as the nabla prompt:
1. User types body text → Execute sends `"      r←x×10\n"`
2. But Dyalog expects `"[1]  r←x×10\n"` → **SysError**

When Dyalog sends `"[1]  "` as the nabla prompt:
1. User types body text → Execute sends `"[1]  r←x×10\n"`
2. Dyalog accepts it → PASS

**The question is: why does Dyalog send different prompt formats to gritt vs RIDE?** Something in gritt's init sequence (SetPW? GetWindowLayout? Something else?) changes the prompt format. Need to test by adding SetPW to standalone test and seeing if it changes the prompt.

## TUI code changes made

### `tui.go` — SetPromptType type=3 fix

Changed the `SetPromptType` handler so that for type=3 (multiline), we DON'T add a new input line. Instead, we position the cursor at the end of the existing line (which was already added by `AppendSessionOutput type=1` with the prompt text). This means:
- The prompt from Dyalog (`"      "` or `"[1]  "`) IS the input line
- Whatever the user types gets appended to that prompt
- The full line (prompt + typed text) is sent as the Execute text

```go
if m.promptType == 3 {
    // Don't add new line — use the one from AppendSessionOutput type=1
    m.cursorRow = len(m.lines) - 1
    m.cursorCol = len([]rune(m.lines[m.cursorRow].Text))
} else if m.promptType == 1 {
    m.lines = append(m.lines, Line{Text: aplIndent})
    m.cursorCol = len(aplIndent)
} else {
    m.lines = append(m.lines, Line{Text: ""})
    m.cursorCol = 0
}
```

### Other changes from previous session (still on branch)

| File | Change |
|------|--------|
| `main.go` | `RIDE_SPAWNED=1` + `DYALOG_LINEEDITOR_MODE=1` in `launchDyalog()` |
| `uitest/tmux.go` | Same env vars in `StartDyalog()` |
| `tui.go` Model | `promptType int`, `pending []string` fields |
| `tui.go` SetPromptType | Full handler: tracks promptType, drains pending, multiline prompt fix |
| `tui.go` AppendSessionOutput | Skips type=11 echo (multiline body) |
| `tui.go` title bar | `[…]` (type=3), `[⎕]` (type=2), `[⍞]` (type=4) |
| `tui.go` Escape | `ExitMultilineInput` when promptType==3 |
| `tui.go` autocomplete | Disabled for type=2/4 |
| `tui.go` new handlers | `SysError`, `HadError` |
| `tui_test.go` | Multiline + quad tests (FAILING) |
| `cmd/test-multiline/` | Standalone protocol test (PASSING) |

## Next steps

1. **Add `SetPW` to standalone test** → check if it changes prompt from `[1]  ` to `      ` for nabla
2. If SetPW is the cause, either:
   a. Don't send SetPW (RIDE sends it too though, so it shouldn't matter)
   b. Check what OTHER init messages RIDE sends that restore the `[n]  ` prompt
   c. Look at RIDE's init.js for full list of init messages
3. If SetPW is NOT the cause, systematically test other init messages (GetWindowLayout, etc.)
4. Once nabla gets `[n]  ` prompts, the TUI code should work correctly
5. Fix multiline TUI tests
