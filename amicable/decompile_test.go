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

func TestDecompileTradfn(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	cases := []struct {
		name string
		fix  []string // lines passed to ⎕FX
		want string   // expected decompiled output (lines joined with \n)
	}{
		{"add", []string{"r←add x", "r←x+1"}, "r←add x\nr←x+1"},
		{"halve", []string{"halve x", "⎕←x÷2"}, "halve x\n⎕←x÷2"},
		{"gcd",
			[]string{"r←a gcd b", ":If b=0", "r←a", ":Else", "r←b gcd b|a", ":EndIf"},
			"r←a gcd b\n:If b=0\nr←a\n:Else\nr←b gcd b|a\n:EndIf"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := tradfnFromDyalog(t, tc.name, tc.fix)
			src, err := raw.Decompile()
			if err != nil {
				t.Fatalf("Decompile: %v", err)
			}
			if src != tc.want {
				t.Errorf("want: %q\n got: %q", tc.want, src)
			}
		})
	}
}

// tradfnFromDyalog defines a tradfn via ⎕FX, then serializes its ⎕OR.
func tradfnFromDyalog(t *testing.T, name string, lines []string) Raw {
	t.Helper()
	// Build ⎕FX expression: ⎕FX 'line1' 'line2' ...
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(l, "'", "''"))
	}
	fixExpr := "sink←⎕FX " + strings.Join(parts, " ")

	out, err := exec.Command("gritt", "-l",
		"-e", fixExpr,
		"-e", fmt.Sprintf("1(220⌶)⎕OR'%s'", name),
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
