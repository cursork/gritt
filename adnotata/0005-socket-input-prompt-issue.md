# 0005 - Socket Mode Blocks on Input Prompts

*2026-01-24*

## Problem

Socket mode hangs when APL is waiting for `⎕` or `⍞` input.

## Reproduction

```bash
# Start gritt with socket
gritt -addr localhost:14502 -sock /tmp/apl.sock &

# This works
echo "1+1" | nc -U -w 2 /tmp/apl.sock
# → 2

# This hangs (timeout, no output)
echo "⎕" | nc -U -w 2 /tmp/apl.sock
# → (nothing, times out)

# Session now stuck - even simple expressions return nothing
echo "1+1" | nc -U -w 2 /tmp/apl.sock
# → (nothing)
```

## Cause

In `runExpr` (and presumably the socket equivalent):

```go
// Read until we get SetPromptType with type:1 (ready)
case "SetPromptType":
    if t, ok := msg.Args["type"].(float64); ok && t == 1 {
        return // Ready for next input
    }
```

When APL waits for input:
- `⎕` sends `SetPromptType` with type 0
- `⍞` sends `SetPromptType` with type 4 (probably)

The socket handler keeps waiting for type 1, which never comes until input is provided. But the client has already timed out and disconnected, so it never sees the prompt and can't send input.

## Expected Behavior

Socket mode should return output to the client when *any* prompt type is received:

1. Type 1 (ready) → expression complete, return output
2. Type 0 (⎕ input) → waiting for input, return output so far
3. Type 4 (⍞ input) → waiting for input, return output so far

The client can then:
1. Read the prompt text (e.g., "Enter date:")
2. See the timeout (indicating APL wants input)
3. Reconnect and send the input
4. Repeat until type 1 (expression complete)

## Use Case

Testing STARMAP's DISPLAY function, which uses `⎕` for interactive input:

```apl
∇ Z←GETDATE;D
  ⎕←'Enter date (month day year):'
  D←⎕
  Z←JNU D
∇
```

With fixed socket mode:
```bash
echo "DISPLAY" | nc -U -w 2 /tmp/apl.sock
# → Enter date (month day year):
# (times out - APL waiting for input)

echo "1 14 1974" | nc -U -w 2 /tmp/apl.sock
# → Enter time (hours, 0-24):
# (times out)

# ... continue until complete
```

## Suggested Fix

```go
case "SetPromptType":
    if t, ok := msg.Args["type"].(float64); ok {
        if t == 1 {
            return buf.String() // Complete
        }
        // Any other prompt type = waiting for input
        // Return what we have so far
        return buf.String()
    }
```

Or distinguish with a marker so the client knows whether expression is complete vs waiting:
- Complete: just return output
- Waiting: return output + some signal (but "just text" philosophy suggests avoid this)

Simplest: always return on any SetPromptType. Client uses timeout to distinguish "complete quickly" from "waiting for input".
