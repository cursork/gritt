# Session Buffer Design

## Initial Thoughts

### How Dyalog Session Works

The session is a single editable text buffer:

1. **Output is appended** - execution results appear at the end
2. **Cursor moves freely** - navigate anywhere in the buffer
3. **Any line can be edited** - modify previous inputs
4. **Enter executes current line** - regardless of cursor position
5. **Original is preserved** - edited lines reset, new copy + result appended

### Example Flow

Initial state after executing `1+1`:
```
      1+1
2
█
```

User moves up to `1+1`, edits to `1+2`, presses Enter:
```
      1+1       <- reset to original
2
      1+2       <- new line appended
3               <- result appended
█
```

### Design Questions

1. **Line Types** - How to distinguish input lines from output lines?
2. **Data Structure** - What do we store per line?
3. **Cursor Behavior** - How does movement work?
4. **Enter Key** - Execute and reset logic
5. **Multiline** - RIDE handles this poorly, opportunity to do better
6. **Scrolling** - Viewport management for long sessions

---

## RIDE Source Analysis (2024-01-07)

Investigated ~/dev/ride/src/se.js and ide.js. Key findings:

### Line Structure
Each line has three properties:
- `text` - content (ends with `\n`)
- `type` - numeric flag for line purpose
- `group` - ID for multiline grouping (0 = not grouped)

### Line Types
```
1  = session-unpsec      (unsupported protocol)
2  = session-stdout      (standard output)
3  = session-stderr      (standard error)
4  = session-syscmd      (system command output)
5  = session-aplerr      (APL error)
7  = session-quad        (⎕ quad system output)
8  = session-quotequad   (⍞ quote-quad output)
9  = session-info        (information message)
11 = session-echo-input  (echoed user input in multiline mode)
12 = session-trace       (tracer output)
14 = session-input       (raw input echo)
```

Input lines are type 11 or 14.

### Dirty Tracking
```javascript
dirty[lineNum] = undefined  // unmodified
dirty[lineNum] = 0          // inserted (new line)
dirty[lineNum] = "original" // modified, stores original
```

Used to:
- Highlight modified lines
- Know what to execute (dirty lines, or current line if none)
- Reset lines via `undoChanges()`

### Prompt Types
- `0` = not ready (read-only)
- `1` = normal single-line input ready
- `3` = multiline input in progress
- `4` = string input (quote-quad ⍞)

### Multiline Blocks
```javascript
multiLineBlocks[groupId] = {start: lineNumber, end: lineNumber}
```

When in multiline mode (promptType === 3):
- Each input line gets a group ID
- Enter executes the whole block
- Lines are tracked for block execution

### Execution Logic
1. If dirty lines exist → execute those
2. Else if selection exists → execute selection
3. Else if in multiline block → execute whole block
4. Else → execute current line

---

## Implications for Our Design

Answer to question 1: RIDE uses numeric types. We should too - matches protocol.

Answer to question 2: Need `{Text, Type, Group}` per line, plus separate dirty map.

Answer to question 5 (multiline): This is baked into the design from the start via:
- `group` field on lines
- `promptType` state tracking
- Block execution logic

Key insight: Multiline isn't a special mode we add later - it's fundamental to how the session works. The `group` field and prompt type handling need to be there from day one.

---

## Next: Proposed Go Implementation

```go
type Line struct {
    Text  string
    Type  int  // matches protocol types
    Group int  // 0 = not grouped
}

type Session struct {
    Lines      []Line
    Dirty      map[int]string // lineIdx -> original ("" = inserted)
    CursorLine int
    CursorCol  int
    PromptType int
    NextGroup  int
}
```
