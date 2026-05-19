#!/usr/bin/env bash
# DCF round-trip proof:
#   1. Dyalog creates a .dcf with a known value.
#   2. gritt's TUI opens the file, edits a cell, ESC-saves.
#   3. A fresh Dyalog process reads the file and prints what it sees.
#
# Pure-Go DCF read+write path. The only Dyalog calls are steps 1 and 3
# — both spawned via `gritt -l` for convenience, both untied before the
# next step. gritt's TUI in step 2 uses Dyalog ONLY for its own session
# init (it must connect to something to start); the DCF edit itself is
# pure Go and runs without consulting that interpreter.
#
# Requirements: tmux, Dyalog v20, the `gritt` binary built in this repo.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT=$(pwd)
FIXTURE=/tmp/dcf-proof.dcf
SESSION=dcf-proof-tmux

GRITT=$ROOT/gritt
if [[ ! -x $GRITT ]]; then
  echo "building gritt..."
  go build -o gritt .
fi

cleanup() {
  tmux kill-session -t $SESSION 2>/dev/null || true
  pkill -9 dyalog 2>/dev/null || true
  pkill -9 mapl 2>/dev/null || true
}
trap cleanup EXIT

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
blue()  { printf '\033[34m%s\033[0m\n' "$*"; }

blue "=== Step 1: Dyalog creates the fixture ==="
rm -f "$FIXTURE"
pkill -9 dyalog 2>/dev/null || true
pkill -9 mapl 2>/dev/null || true
sleep 1
"$GRITT" -l -version 20.0 \
  -e "tie ← '$FIXTURE' (⎕FCREATE⍠1) 0" \
  -e "r ← 'C' 0 ⎕FPROPS tie" \
  -e "(⍳5) ⎕FAPPEND tie" \
  -e "⎕←'  Dyalog wrote: ',⍕⎕FREAD tie,1" \
  -e "⎕FUNTIE tie" 2>&1 | grep "Dyalog wrote"

if [[ ! -f $FIXTURE ]]; then
  red "FAIL: fixture not created"
  exit 1
fi
ls -la "$FIXTURE"

blue
blue "=== Step 2: gritt TUI opens the file, edits cell 1 to 99, ESC-saves ==="
pkill -9 dyalog 2>/dev/null || true
pkill -9 mapl 2>/dev/null || true
sleep 1

tmux kill-session -t $SESSION 2>/dev/null || true
tmux new-session -d -s $SESSION -x 200 -y 50 "cd $ROOT && $GRITT -l -version 20.0"
sleep 4  # let Dyalog start and gritt connect

send() { tmux send-keys -t $SESSION "$@"; }
type() { tmux send-keys -t $SESSION -l "$@"; }

# Open command palette
send "C-]"
sleep 0.2
send ":"
sleep 0.4

# Type the command and select
type "open-dcf"
sleep 0.3
send "Enter"
sleep 0.3

# Type the file path
type "$FIXTURE"
sleep 0.2
send "Enter"
sleep 0.6
echo "  (DCF opened in TUI)"

# Drill into component 1 (the vector)
send "Enter"
sleep 0.4
# Enter edit mode on cell 0 (current value: 1)
send "Enter"
sleep 0.3
# Clear and type 99
send "BSpace" "BSpace" "BSpace"
type "99"
sleep 0.2
send "Enter"   # confirm the edit
sleep 0.4
echo "  (cell 0 edited to 99)"

# ESC back to component list (commits edit visually)
send "Escape"
sleep 0.3
# ESC again — closes the DCF pane, triggers saveDCF
send "Escape"
sleep 0.8
echo "  (DCF pane closed; saveDCF should have written to disk)"

# Quit gritt
send "C-]"
sleep 0.2
send "q"
sleep 0.3
send "y"
sleep 1
tmux kill-session -t $SESSION 2>/dev/null || true

blue
blue "=== Step 3: fresh Dyalog reads the modified file ==="
pkill -9 dyalog 2>/dev/null || true
pkill -9 mapl 2>/dev/null || true
sleep 1

RESULT=$("$GRITT" -l -version 20.0 \
  -e "tie ← '$FIXTURE' ⎕FTIE 0" \
  -e "⎕←'  Dyalog now reads: ',⍕⎕FREAD tie,1" \
  -e "⎕FUNTIE tie" 2>&1 | grep "Dyalog now reads")

echo "$RESULT"

blue
if echo "$RESULT" | grep -q '99 2 3 4 5'; then
  green "=================================================="
  green " PROOF SUCCESS"
  green " Dyalog read back the value edited by gritt's TUI."
  green " Pure-Go DCF write verified end-to-end."
  green "=================================================="
  exit 0
else
  red "=================================================="
  red " PROOF FAILURE"
  red " Dyalog did not read '99 2 3 4 5'."
  red " File state may be inspected at: $FIXTURE"
  red "=================================================="
  exit 1
fi
