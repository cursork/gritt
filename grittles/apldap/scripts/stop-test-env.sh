#!/bin/bash
# Stop test environment for apldap
# Uses pkill -9 for clean slate - TEST ONLY

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Nuclear option - kill all dyalog and multapl processes
pkill -9 dyalog 2>/dev/null && echo "Killed dyalog"
pkill -9 multapl 2>/dev/null && echo "Killed multapl"

# Clean up PID files
rm -f "$PROJECT_DIR/.dyalog.pid" "$PROJECT_DIR/.multapl.pid"

# Also kill by port in case anything else is listening
lsof -ti :9501 2>/dev/null | xargs kill -9 2>/dev/null || true
lsof -ti :9502 2>/dev/null | xargs kill -9 2>/dev/null || true
lsof -ti :9503 2>/dev/null | xargs kill -9 2>/dev/null || true

sleep 0.5
echo "Test environment stopped"
