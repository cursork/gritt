#!/usr/bin/env zsh
# Test aplsock in both modes: -l (launch) and against existing Dyalog.
# Only kills processes we start — never pkill dyalog.
set -e
cd "$(dirname "$0")/../.."
go build -o /tmp/aplsock ./grittles/aplsock/
go build -o /tmp/testdyalog ./grittles/aplsock/testdyalog/

PIDS=()
cleanup() {
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null
}
trap cleanup EXIT

pass=0; fail=0
check() {
    local desc="$1" expr="$2" expect="$3" port="$4"
    local got
    got=$( (echo "$expr"; sleep 2) | nc localhost "$port" | head -1)
    if [ "$got" = "$expect" ]; then
        echo "  PASS: $desc"
        pass=$((pass + 1))
    else
        echo "  FAIL: $desc"
        echo "        expect: $expect"
        echo "        got:    $got"
        fail=$((fail + 1))
    fi
}

suite() {
    local p="$1"
    check "scalar"       "1+2"           "3"                  "$p"
    check "negative"     "¯7"            "¯7"                 "$p"
    check "float"        "○1"            "3.141592653589793"  "$p"
    check "vector"       "⍳5"            "1 2 3 4 5"          "$p"
    check "string"       "'hello world'" "'hello world'"      "$p"
    check "empty"        "⍬"             "⍬"                  "$p"
    check "single elem"  ",42"           "(⋄ 42)"             "$p"
    check "matrix"       "2 3⍴⍳6"       "[1 2 3 ⋄ 4 5 6]"   "$p"
    check "char matrix"  "2 3⍴'abcdef'" "['abc' ⋄ 'def']"   "$p"
    check "error"        "1÷0"           "Divide by zero"     "$p"
    check "assign"       "x←42"          "42"                 "$p"
    check "use var"      "x+8"           "50"                 "$p"
    check "reduce"       "+⌿2 3⍴⍳6"     "5 7 9"              "$p"
    check "shape"        "⍴2 3⍴⍳6"      "2 3"                "$p"
}

# ── Mode 1: aplsock -l ──
echo "── aplsock -l ──"
/tmp/aplsock -l -sock :14201 2>/dev/null &
PIDS+=($!); sleep 4
suite 14201
m1_pass=$pass; m1_fail=$fail
kill "${PIDS[-1]}" 2>/dev/null || true; sleep 2

# ── Mode 2: existing Dyalog ──
echo ""
echo "── aplsock (existing Dyalog :14502) ──"
pass=0; fail=0
/tmp/testdyalog &
PIDS+=($!); sleep 3
/tmp/aplsock -addr localhost:14502 -sock :14201 2>/dev/null &
PIDS+=($!); sleep 4
suite 14201
m2_pass=$pass; m2_fail=$fail

# ── Results ──
echo ""
echo "── Results ──"
echo "  -l mode:       $m1_pass/14"
echo "  existing mode: $m2_pass/14"
total=$((m1_fail + m2_fail))
[ $total -eq 0 ] && echo "  ALL PASSED" || echo "  $total FAILED"
exit $total
