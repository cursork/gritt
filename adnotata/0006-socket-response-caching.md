# 0006 - Socket Mode Response Caching Issue

*2026-01-24*

## Problem

Responses from previous socket connections are delivered to subsequent connections. Each connection should only see responses to its own request.

## Reproduction

```bash
# Fresh start
pkill dyalog; pkill gritt
RIDE_INIT=SERVE:127.0.0.1:14502 dyalog +s -q &
sleep 2
gritt -addr localhost:14502 -sock /tmp/apl.sock &
sleep 1

# Request 1
echo "999" | nc -U /tmp/apl.sock
# → (nothing)

# Request 2
echo "888" | nc -U /tmp/apl.sock
# → 999   ← response from request 1!

# Request 3
echo "777" | nc -U /tmp/apl.sock
# → (nothing)

# Request 4
echo "666" | nc -U /tmp/apl.sock
# → 888   ← response from request 2!
```

## Cause

When a socket client disconnects before reading the response, the response is cached. The next client connection receives this stale cached response instead of (or in addition to) its own.

## Expected Behavior

Each socket connection should:
1. Clear any cached/pending output from previous connections
2. Send its expression to Dyalog
3. Receive only the response to its own expression
4. Disconnect cleanly

The response buffer should be cleared when a new connection is accepted, not preserved across connections.

## Impact

This makes the socket interface unusable for automation - you can't reliably match responses to requests. The timeout-based approach for interactive input (send expression, timeout, send input) breaks because responses arrive on the wrong connection.

## Analysis

The likely cause is a single shared buffer that accumulates Dyalog output regardless of whether a client is connected. When client A disconnects before response arrives, the response goes into this shared buffer. Client B then reads from the same buffer and gets stale data.

## Suggested Fix

Don't use a shared buffer. Two options:

1. **Write to active connection or discard** - Output only goes to the currently connected client. If no client is connected when output arrives, discard it. Simplest approach.

2. **Per-connection buffer** - Create buffer on accept, destroy on close. Each connection only sees its own output.

Option 1 is simpler and matches single-threaded usage. A client that disconnects before reading its response simply loses that response - that's expected behavior.
