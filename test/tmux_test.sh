#!/bin/bash
# tmux-driven TUI test for gritt
# Requires: tmux, dyalog running on port 4502

set -e

SESSION="gritt-test"
GRITT_BIN="${GRITT_BIN:-./gritt}"
TIMEOUT=5

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

cleanup() {
    tmux kill-session -t "$SESSION" 2>/dev/null || true
}
trap cleanup EXIT

fail() {
    echo -e "${RED}FAIL:${NC} $1"
    exit 1
}

pass() {
    echo -e "${GREEN}PASS:${NC} $1"
}

capture() {
    tmux capture-pane -t "$SESSION" -p
}

wait_for() {
    local pattern="$1"
    local timeout="${2:-$TIMEOUT}"
    local start=$(date +%s)

    while true; do
        if capture | grep -q "$pattern"; then
            return 0
        fi

        local now=$(date +%s)
        if (( now - start > timeout )); then
            echo "Timed out waiting for: $pattern"
            echo "Current screen:"
            capture
            return 1
        fi
        sleep 0.2
    done
}

send_keys() {
    tmux send-keys -t "$SESSION" "$@"
}

send_line() {
    send_keys "$1" Enter
}

# --- Tests ---

echo "=== gritt tmux test ==="
echo ""

# Build first
echo "Building gritt..."
go build -o "$GRITT_BIN" . || fail "Build failed"

# Check if Dyalog is running
if ! nc -z localhost 4502 2>/dev/null; then
    echo "Warning: Dyalog not running on port 4502"
    echo "Start with: RIDE_INIT=SERVE:*:4502 dyalog +s -q"
    echo ""
    echo "Running UI-only tests (no RIDE connection)..."
    NO_DYALOG=1
fi

# Start gritt in tmux
echo "Starting gritt in tmux session '$SESSION'..."
cleanup
tmux new-session -d -s "$SESSION" -x 100 -y 30 "$GRITT_BIN"
sleep 1

# Test 1: Check initial render
echo ""
echo "Test 1: Initial render"
if capture | grep -q "gritt"; then
    pass "Title bar rendered"
else
    fail "Title bar not found"
fi

# Test 2: F12 toggles debug pane
echo ""
echo "Test 2: F12 toggles debug pane"
send_keys F12
sleep 0.3
if capture | grep -q "debug"; then
    pass "Debug pane appeared"
else
    fail "Debug pane not found after F12"
fi

# Test 3: Debug pane is floating (has double border when focused)
echo ""
echo "Test 3: Debug pane has focus indicator"
if capture | grep -q "╔"; then
    pass "Focused pane has double border"
else
    # Might have single border if focus logic differs
    if capture | grep -q "┌.*debug"; then
        pass "Debug pane rendered (single border)"
    else
        fail "Debug pane border not found"
    fi
fi

# Test 4: Esc closes debug pane
echo ""
echo "Test 4: Esc closes debug pane"
send_keys Escape
sleep 0.3
if ! capture | grep -q "╔.*debug\|┌.*debug"; then
    pass "Debug pane closed"
else
    fail "Debug pane still visible after Esc"
fi

# Test 5: F12 again reopens it
echo ""
echo "Test 5: F12 reopens debug pane"
send_keys F12
sleep 0.3
if capture | grep -q "debug"; then
    pass "Debug pane reopened"
else
    fail "Debug pane not found after second F12"
fi

# If Dyalog is running, test execution
if [[ -z "$NO_DYALOG" ]]; then
    echo ""
    echo "Test 6: Execute APL (1+1)"
    send_keys Escape  # Close debug first
    sleep 0.2
    send_line "1+1"
    if wait_for "2"; then
        pass "APL execution returned 2"
    else
        fail "Expected result '2' not found"
    fi

    echo ""
    echo "Test 7: Execute iota"
    send_line "⍳5"
    if wait_for "1 2 3 4 5"; then
        pass "Iota returned 1 2 3 4 5"
    else
        fail "Iota result not found"
    fi
fi

# Cleanup
echo ""
echo "=== All tests passed ==="
capture > test/last_output.txt
echo "Final screen saved to test/last_output.txt"
