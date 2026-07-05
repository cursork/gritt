#!/bin/bash
# Start test environment for apldap
# Ports: 9501 = Dyalog RIDE, 9502 = multapl, 9503 = reserved

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
MULTAPL_DIR="$HOME/dev/multapl"

# Build multapl if needed
if [ ! -f "$MULTAPL_DIR/multapl" ]; then
    echo "Building multapl..."
    (cd "$MULTAPL_DIR" && go build -o multapl .)
fi

# Nuclear cleanup - kill any existing dyalog/multapl
echo "Cleaning up existing processes..."
pkill -9 dyalog 2>/dev/null || true
pkill -9 multapl 2>/dev/null || true
lsof -ti :9501 2>/dev/null | xargs kill -9 2>/dev/null || true
lsof -ti :9502 2>/dev/null | xargs kill -9 2>/dev/null || true
lsof -ti :9503 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 0.5

# Start Dyalog with RIDE server on 9501
echo "Starting Dyalog on port 9501..."
RIDE_INIT="SERVE:*:9501" dyalog +s -q &
DYALOG_PID=$!
echo "Dyalog PID: $DYALOG_PID"

# Wait for Dyalog to start listening
sleep 1
for i in {1..10}; do
    if lsof -i :9501 >/dev/null 2>&1; then
        echo "Dyalog is listening on 9501"
        break
    fi
    sleep 0.5
done

# Start multapl connecting to Dyalog on 9501, serving primary on 9502, secondary on 9503
echo "Starting multapl (primary=9502, secondary=9503, connecting to 9501)..."
"$MULTAPL_DIR/multapl" -host 127.0.0.1 -port 9501 -primary 9502 -secondary 9503 &
MULTAPL_PID=$!
echo "multapl PID: $MULTAPL_PID"

# Wait for multapl to start
sleep 1
for i in {1..10}; do
    if lsof -i :9502 >/dev/null 2>&1; then
        echo "multapl is listening on 9502 (primary) and 9503 (secondary)"
        break
    fi
    sleep 0.5
done

echo ""
echo "Test environment ready!"
echo "  Dyalog RIDE: 127.0.0.1:9501"
echo "  multapl:     127.0.0.1:9502"
echo ""
echo "To stop: kill $DYALOG_PID $MULTAPL_PID"
echo "Or run: $SCRIPT_DIR/stop-test-env.sh"

# Save PIDs for stop script
echo "$DYALOG_PID" > "$PROJECT_DIR/.dyalog.pid"
echo "$MULTAPL_PID" > "$PROJECT_DIR/.multapl.pid"
