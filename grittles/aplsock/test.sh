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
    check "nested vec"   "(1 2)(3 4)"    "(1 2 ⋄ 3 4)"       "$p"
    check "mixed types"  "1 'hello' 3"   "(1 ⋄ 'hello' ⋄ 3)" "$p"
    check "deep nest"    "(1 2)(3 (4 5))" "(1 2 ⋄ (3 ⋄ 4 5))" "$p"
    check "namespace"    "ns←⎕NS'' ⋄ ns.x←(1 2)(3 4) ⋄ ns.y←'hi' ⋄ ns" "(x: (1 2 ⋄ 3 4) ⋄ y: 'hi')" "$p"
    check "shy result"   "x←99"          "99"                 "$p"
    check "boolean vec"  "1 0 1 0 1"     "1 0 1 0 1"          "$p"
    check "complex"      "3J4"           "3J4"                "$p"
    check "dfn assign"   "f←{⍺×⍵}"      ""                   "$p"
    check "dfn call"     "3 f 4"         "12"                 "$p"
    check "error recov"  "÷0"            "Divide by zero"     "$p"
    check "after error"  "1+1"           "2"                  "$p"
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

# ── Mode 3: raw protocol with ⍝ID: ──
echo ""
echo "── raw protocol (⍝ID:) ──"
pass=0; fail=0
kill "${PIDS[-1]}" 2>/dev/null || true; sleep 1
kill "${PIDS[-2]}" 2>/dev/null || true; sleep 2

/tmp/aplsock -l -sock :14201 2>/tmp/aplsock_raw.log &
PIDS+=($!); sleep 5
IPORT=$(grep "internal port" /tmp/aplsock_raw.log | grep -o '[0-9]*$')

check_raw() {
    local desc="$1" expr="$2" expect="$3"
    local got
    got=$( (echo "$expr"; sleep 2) | nc localhost "$IPORT" | head -1)
    if [[ "$got" == "$expect"* ]]; then
        echo "  PASS: $desc"
        pass=$((pass + 1))
    else
        echo "  FAIL: $desc"
        echo "        expect: $expect..."
        echo "        got:    $got"
        fail=$((fail + 1))
    fi
}

UUID1="019abc12-3456-7890-abcd-ef1234567890"
UUID2="019abc12-3456-7890-abcd-ef1234567891"
check_raw "no id"    "1+2"                         "(tag: 'ret' ⋄ val: 3)"
check_raw "with id"  "⍳3 ⍝ID:$UUID1"              "(id: '$UUID1' ⋄ tag: 'ret' ⋄ val: 1 2 3)"
check_raw "err+id"   "÷0 ⍝ID:$UUID2"              "(id: '$UUID2' ⋄ tag: 'err'"
m3_pass=$pass; m3_fail=$fail

# ── Results ──
echo ""
echo "── Results ──"
echo "  -l mode:       $m1_pass/$((m1_pass + m1_fail))"
echo "  existing mode: $m2_pass/$((m2_pass + m2_fail))"
echo "  raw protocol:  $m3_pass/$((m3_pass + m3_fail))"
total=$((m1_fail + m2_fail + m3_fail))
[ $total -eq 0 ] && echo "  ALL PASSED" || echo "  $total FAILED"
exit $total
