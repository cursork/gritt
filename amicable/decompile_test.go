package amicable

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestDecompile(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	cases := []struct {
		name, expr string
	}{
		{"add1", "{⍵+1}"},
		{"dyadic", "{⍺+⍵}"},
		{"sub1", "{⍵-1}"},
		{"times2", "{⍵×2}"},
		{"reduce", "{+/⍵}"},
		{"scan", "{+\\⍵}"},
		{"selfie", "{+⍨⍵}"},
		{"guard", "{0=⍵:0 ⋄ ⍵}"},
		{"paren", "{(⍵+1)×2}"},
		{"index", "{⍵[1]}"},
		{"hello", "{⎕←'hello world'}"},
		{"sysio", "{⎕IO}"},
		{"multi", "{r←⍵+1 ⋄ r}"},
		{"collatz", "{0=2|⍵:⍵÷2 ⋄ 1+3×⍵}"},
		{"fib", "{⍵≤1:⍵ ⋄ (∇⍵-1)+∇⍵-2}"},
		{"gcd", "{0=⍵:⍺ ⋄ ⍵∇⍵|⍺}"},
		{"avg", "{(+/⍵)÷≢⍵}"},
		{"reverse", "{⌽⍵}"},
		{"pow", "{×/⍵⍴⍺}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := orFromDyalog(t, tc.expr)
			src, err := raw.Decompile()
			if err != nil {
				t.Fatalf("Decompile: %v", err)
			}
			if src != tc.expr {
				t.Errorf("want: %s\n got: %s", tc.expr, src)
			}
		})
	}
}

// orFromDyalog serializes a dfn via ⎕OR in a fresh Dyalog session.
func orFromDyalog(t *testing.T, expr string) Raw {
	t.Helper()
	out, err := exec.Command("gritt", "-l",
		"-e", "OR←{f←⍺⍺⋄⎕OR'f'}",
		"-e", fmt.Sprintf("1(220⌶)(%s)OR ⍬", expr),
	).CombinedOutput()
	if err != nil {
		t.Fatalf("gritt: %v\n%s", err, out)
	}

	s := strings.TrimSpace(string(out))
	s = strings.ReplaceAll(s, "¯", "-")
	fields := strings.Fields(s)

	data := make([]byte, len(fields))
	for i, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("parse byte %d %q: %v", i, f, err)
		}
		data[i] = byte(int8(v))
	}

	val, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	raw, ok := val.(Raw)
	if !ok {
		t.Fatalf("expected Raw, got %T", val)
	}
	return raw
}
