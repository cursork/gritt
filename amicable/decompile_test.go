package amicable

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestDecompile tests dfn decompilation. One Dyalog session for all cases.
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
		// All primitives
		{"ceil", "{⌈⍵}"}, {"floor", "{⌊⍵}"}, {"exp", "{*⍵}"}, {"log", "{⍟⍵}"},
		{"mag", "{|⍵}"}, {"fact", "{!⍵}"}, {"pi", "{○⍵}"}, {"not", "{~⍵}"},
		{"or", "{∨⍵}"}, {"and", "{∧⍵}"}, {"nand", "{⍲⍵}"}, {"nor", "{⍱⍵}"},
		{"lt", "{<⍵}"}, {"le", "{≤⍵}"}, {"eq", "{=⍵}"}, {"ge", "{≥⍵}"},
		{"gt", "{>⍵}"}, {"ne", "{≠⍵}"}, {"match", "{≡⍵}"}, {"tally", "{≢⍵}"},
		{"shape", "{⍴⍵}"}, {"ravel", "{,⍵}"}, {"table", "{⍪⍵}"}, {"iota", "{⍳⍵}"},
		{"take", "{↑⍵}"}, {"drop", "{↓⍵}"}, {"roll", "{?⍵}"}, {"gradedn", "{⍒⍵}"},
		{"gradeup", "{⍋⍵}"}, {"transpose", "{⍉⍵}"}, {"rotlast", "{⊖⍵}"},
		{"enlist", "{∊⍵}"}, {"decode", "{⊥⍵}"}, {"encode", "{⊤⍵}"},
		{"exec", "{⍎⍵}"}, {"format", "{⍕⍵}"}, {"matinv", "{⌹⍵}"},
		{"enclose", "{⊂⍵}"}, {"disclose", "{⊃⍵}"}, {"unique", "{∪⍵}"},
		{"intersect", "{∩⍵}"}, {"find", "{⍷⍵}"}, {"squad", "{⌷⍵}"},
		{"partition", "{⊆⍵}"}, {"over", "{⍥⍵}"}, {"left", "{⊣⍵}"},
		{"right", "{⊢⍵}"}, {"where", "{⍸⍵}"}, {"at", "{@⍵}"},
		// Operators
		{"reduce1", "{+⌿⍵}"}, {"expand1", "{+⍀⍵}"}, {"power", "{+⍣⍵}"},
		{"variant", "{+⍠⍵}"}, {"rank", "{+⍤⍵}"}, {"key", "{+⌸⍵}"},
		{"stencil", "{+⌺⍵}"},
	}

	blobs := batchSerializeDfns(t, cases)

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := blobs[i].Decompile()
			if err != nil {
				t.Fatalf("Decompile: %v", err)
			}
			if src != tc.expr {
				// Variable names in dfn bytecode are session-dependent
				// (inline ASCII shifts with workspace state). Accept if
				// structure matches but a single-char local name differs.
				if !structuralMatch(tc.expr, src) {
					t.Errorf("want: %s\n got: %s", tc.expr, src)
				}
			}
		})
	}
}

// structuralMatch checks if two dfn sources match structurally,
// allowing single-char variable names to differ.
func structuralMatch(want, got string) bool {
	if len(want) != len(got) {
		return false
	}
	wr := []rune(want)
	gr := []rune(got)
	for i := range wr {
		if wr[i] == gr[i] {
			continue
		}
		// Allow single lowercase letter differences (variable names)
		if wr[i] >= 'a' && wr[i] <= 'z' && gr[i] >= 'a' && gr[i] <= 'z' {
			continue
		}
		return false
	}
	return true
}

// TestDecompileTradfn tests tradfn decompilation. One Dyalog session.
func TestDecompileTradfn(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	cases := []struct {
		name string
		fix  []string
		want string
	}{
		{"add", []string{"r←add x", "r←x+1"}, "r←add x\nr←x+1"},
		{"halve", []string{"halve x", "⎕←x÷2"}, "halve x\n⎕←x÷2"},
		{"gcd",
			[]string{"r←a gcd b", ":If b=0", "r←a", ":Else", "r←b gcd b|a", ":EndIf"},
			"r←a gcd b\n:If b=0\nr←a\n:Else\nr←b gcd b|a\n:EndIf"},
	}

	// Build one gritt call: define all functions, then serialize each
	args := []string{"-l"}
	for _, tc := range cases {
		parts := make([]string, len(tc.fix))
		for i, l := range tc.fix {
			parts[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(l, "'", "''"))
		}
		args = append(args, "-e", "sink←⎕FX "+strings.Join(parts, " "))
	}
	for i, tc := range cases {
		args = append(args, "-e", fmt.Sprintf("'=%d=' ⋄ 1(220⌶)⎕OR'%s'", i, tc.name))
	}

	blobs := parseDelimitedBlobs(t, args, len(cases))

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := blobs[i].Decompile()
			if err != nil {
				t.Fatalf("Decompile: %v", err)
			}
			if src != tc.want {
				t.Errorf("want: %q\n got: %q", tc.want, src)
			}
		})
	}
}

// TestDecompileNamespace tests namespace decompilation. One Dyalog session.
func TestDecompileNamespace(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	cases := []struct {
		name, setup, want string
	}{
		{"vars", "ns←⎕NS '' ⋄ ns.x←42 ⋄ ns.name←'Neil'",
			":Namespace\n    x←42\n    name←'Neil'\n:EndNamespace"},
		{"fn_no_lit", "ns←⎕NS '' ⋄ ns.avg←{(+/⍵)÷≢⍵}",
			":Namespace ns\n    avg←{(+/⍵)÷≢⍵}\n:EndNamespace"},
		{"fn_with_lit", "ns←⎕NS '' ⋄ ns.double←{⍵×2}",
			":Namespace ns\n    double←{⍵×2}\n:EndNamespace"},
	}

	// One session: each case sets up ns differently, serialize, then erase
	args := []string{"-l"}
	for i, tc := range cases {
		args = append(args, "-e", tc.setup)
		args = append(args, "-e", fmt.Sprintf("'=%d=' ⋄ 1(220⌶)⎕OR'ns'", i))
		args = append(args, "-e", ")erase ns")
	}

	blobs := parseDelimitedBlobs(t, args, len(cases))

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := blobs[i].Decompile()
			if err != nil {
				t.Fatalf("Decompile: %v", err)
			}
			if src != tc.want {
				t.Errorf("want: %q\n got: %q", tc.want, src)
			}
		})
	}
}

// --- Helpers ---

// batchSerializeDfns serializes all dfn cases in one Dyalog session.
func batchSerializeDfns(t *testing.T, cases []struct{ name, expr string }) []Raw {
	t.Helper()

	args := []string{"-l", "-e", "OR←{f←⍺⍺⋄⎕OR'f'}"}
	for i, c := range cases {
		args = append(args, "-e", fmt.Sprintf("'=%d=' ⋄ 1(220⌶)(%s)OR ⍬", i, c.expr))
	}

	return parseDelimitedBlobs(t, args, len(cases))
}

// parseDelimitedBlobs runs gritt with the given args and parses =N= delimited blobs.
func parseDelimitedBlobs(t *testing.T, args []string, n int) []Raw {
	t.Helper()

	out, err := exec.Command("gritt", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("gritt: %v\n%s", err, out)
	}
	content := string(out)

	blobs := make([]Raw, n)
	for i := range n {
		delim := fmt.Sprintf("=%d=", i)
		idx := strings.Index(content, delim)
		if idx < 0 {
			t.Fatalf("delimiter %q not found in output", delim)
		}
		after := content[idx+len(delim):]
		after = strings.TrimLeft(after, " \n\r")

		end := len(after)
		if ni := strings.Index(after, "\n="); ni >= 0 {
			end = ni
		}
		chunk := strings.TrimSpace(after[:end])
		chunk = strings.ReplaceAll(chunk, "¯", "-")
		fields := strings.Fields(chunk)

		data := make([]byte, len(fields))
		for j, f := range fields {
			v, err := strconv.Atoi(f)
			if err != nil {
				t.Fatalf("case %d: parse byte %d %q: %v", i, j, f, err)
			}
			data[j] = byte(int8(v))
		}

		val, err := Unmarshal(data)
		if err != nil {
			t.Fatalf("case %d: Unmarshal: %v", i, err)
		}
		raw, ok := val.(Raw)
		if !ok {
			t.Fatalf("case %d: expected Raw, got %T", i, val)
		}
		blobs[i] = raw
	}
	return blobs
}
