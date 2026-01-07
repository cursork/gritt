# RIDE JS Implementation Notes

Reference: ~/dev/ride/src/

## Key Files

- `se.js` - Session editor (the main session buffer)
- `ide.js` - IDE orchestration
- `cn.js` - Connection/protocol handling

## Session Editor (se.js)

### Data Structures

```javascript
se.lines = []           // Array of {text, type, group}
se.dirty = {}           // lineNum -> original content (0 = inserted, string = modified)
se.lineEditor = {}      // lineNum -> true for editable input lines
se.multiLineBlocks = {} // groupId -> {start: lineNum, end: lineNum}
```

### Line Object

```javascript
{
  text: "      1+1\n",  // includes prompt spaces, ends with \n
  type: 14,             // numeric type code
  group: 0              // multiline group ID, 0 = not grouped
}
```

### Line Types

| Type | CSS Class | Meaning |
|------|-----------|---------|
| 1 | session-unpsec | Unsupported protocol |
| 2 | session-stdout | Standard output |
| 3 | session-stderr | Standard error |
| 4 | session-syscmd | System command output |
| 5 | session-aplerr | APL error |
| 7 | session-quad | ⎕ (quad) output |
| 8 | session-quotequad | ⍞ (quote-quad) output |
| 9 | session-info | Information message |
| 11 | session-echo-input | Echoed input in multiline mode |
| 12 | session-trace | Tracer output |
| 14 | session-input | Raw input echo |

Input detection: `type === 11 || type === 14`

### Prompt Types (from SetPromptType message)

| Type | Meaning |
|------|---------|
| 0 | Not ready, read-only |
| 1 | Ready for single-line input |
| 3 | Multiline input in progress |
| 4 | Quote-quad (⍞) string input |

### Dirty Tracking

```javascript
// Not in dirty map = unmodified
se.dirty[lineNum] = 0          // Line was inserted (new)
se.dirty[lineNum] = "original" // Line was modified, stores original text
```

Purpose:
- Visual highlighting of modified lines
- Determines what to execute
- Enables undo/reset to original

### Execution Logic (se.js ~line 637)

Priority order:
1. If dirty lines exist → execute all dirty lines
2. Else if text selected → execute selection
3. Else if cursor in multiline block → execute whole block
4. Else → execute current line

After execution, dirty lines are reset to original.

### Multiline Handling

When `promptType` changes to 3 (multiline):
- Current line marked as editable (`lineEditor[lineNum] = true`)
- Lines get assigned to a group

When `promptType` leaves 3:
- Multiline block is finalized with a group ID
- `multiLineBlocks[groupId] = {start, end}` records the span

Executing in a multiline block:
```javascript
const block = se.multiLineBlocks[groupId];
const lines = se.lines.slice(block.start - 1, block.end);
// Each line's text has newline stripped before sending
```

### AppendSessionOutput Handling

From protocol message `AppendSessionOutput`:
- `result` - the text to append
- `type` - line type (see table above)

Type 14 is input echo - often skipped in output display.

### Session Log Truncation

Configurable via `D.prf.sessionLogSize()`. Old lines removed when limit exceeded.

## Protocol Messages (relevant to session)

### Incoming (from Dyalog)

| Message | Purpose |
|---------|---------|
| SetPromptType | Change input mode (type: 0/1/3/4) |
| AppendSessionOutput | Add text to session (result, type) |
| EchoInput | Echo of executed input |
| UpdateSessionCaption | Window title update |

### Outgoing (to Dyalog)

| Message | Purpose |
|---------|---------|
| Execute | Run code (text, trace) |
| WeakInterrupt | Ctrl+C soft interrupt |
| StrongInterrupt | Hard interrupt |

## Editor Features

### Cursor Movement
- Free movement anywhere in buffer
- Home/End for line start/end
- Ctrl+Home/End for buffer start/end

### Line Editing
- Insert/delete characters
- Backspace/Delete keys
- Any line can be edited (input or output)

### Special Keys
- Enter: Execute current line/block/dirty lines
- Escape: Clear dirty state, reset modifications
- Ctrl+C: Interrupt execution

## Notes

- Lines always end with `\n` except possibly the last incomplete line
- Prompt is 6 spaces, part of the line text itself
- RIDE uses CodeMirror for the actual text editing
