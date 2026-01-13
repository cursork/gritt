#!/usr/bin/env bash
# Run APL expression via gritt with ephemeral Dyalog instance

PORT=$((10000 + RANDOM % 50000))

# Start Dyalog in background
RIDE_INIT=SERVE:*:$PORT dyalog +s -q &
DYALOG_PID=$!

# Wait for it to be ready
sleep 1

# Run expression, then )off to shut down cleanly
{ echo "$@"; echo ")off"; } | ./gritt -addr "localhost:$PORT" -stdin 2>/dev/null

# Clean up if still running
kill $DYALOG_PID 2>/dev/null
