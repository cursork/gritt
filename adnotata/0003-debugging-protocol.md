# Debugging RIDE Protocol Issues

## The `-log` Flag

Run gritt with protocol logging enabled:

```bash
./gritt -log debug.log
```

This logs both:
- **Protocol messages**: All RIDE messages sent/received with timestamps
- **TUI actions**: Execute commands, window open/close, state changes

## Log Format

```
[HH:MM:SS.mmm] → outgoing message
[HH:MM:SS.mmm] ← incoming message
[HH:MM:SS.mmm] internal state change
```

Example:
```
[01:32:04.148] → SaveChanges win=1
[01:32:04.148]   (waiting for ReplySaveChanges before CloseWindow)
[01:32:04.152] ← ["ReplySaveChanges",{"win":1,"err":0}]
[01:32:04.154] → CloseWindow win=1
[01:32:04.155] ← CloseWindow {"win":1}
[01:32:04.155]   closed editor: token=1
```

## Test Logging

Tests automatically log to `test-reports/protocol.log`:

```go
runner, err := uitest.NewRunner(t, sessionName, screenW, screenH,
    "./gritt -log test-reports/protocol.log", "test-reports")
```

## Common Issues

### CloseWindow Ignored

**Symptom**: Editor doesn't close, subsequent commands typed into wrong window

**Cause**: Sending `CloseWindow` before `ReplySaveChanges` arrives

**Solution**: Wait for `ReplySaveChanges` before sending `CloseWindow`

The log makes this obvious:
```
→ SaveChanges win=1
→ CloseWindow win=1     ← sent too early!
← ReplySaveChanges      ← arrives, but CloseWindow was ignored
```

### Tracer vs Editor Windows

Check the `debugger` field in `OpenWindow`:
- `debugger: 0` = regular editor
- `debugger: 1` = tracer window (part of call stack)

### Message Timing

When debugging timing issues, compare timestamps between:
1. When we send a command
2. When Dyalog responds
3. When we process the response

## Using dyctl for Protocol Capture

For deeper protocol analysis, use the dyctl multiplexer which logs all traffic:

```clojure
(require '[dyctl.multiplexer :as mux])
(mux/start! {:dyalog-port 4501
             :primary-port 4502
             :secondary-port 4503
             :log-file "ride-traffic.log"})
```

Then connect gritt to port 4502 and view raw protocol in the log.
