package amicable

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/cursork/gritt/codec"
)

// TestUnmarshalNested covers nested-namespace unmarshal across the layout
// variants we've identified:
//
//   - sequential layout: extraction order's first member is class-2/3
//     (`ns.x ⋄ ns.a←ns`) — values packed right after the name table
//   - relocated layout: extraction order's first member is class-9
//     (`ns.a←ns ⋄ ns.x`) — class-2/3 values moved to a tail region just
//     before the terminator D5_50 block
//   - sub-namespaces containing functions, strings, vars, or other
//     sub-namespaces (recursive unmarshal)
//   - multiple sub-namespaces side-by-side
//
// A single Dyalog session captures every case via gritt -l, so the test
// pays one launch cost regardless of case count.
func TestUnmarshalNested(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	type checkFn func(t *testing.T, ns *codec.Namespace)

	cases := []struct {
		name, setup string
		check       checkFn
	}{
		{
			"sub_with_var",
			`ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.x←42`,
			func(t *testing.T, ns *codec.Namespace) {
				a := requireSubNs(t, ns, "a")
				if a.Values["x"] != 42 {
					t.Errorf("a.x = %v, want 42", a.Values["x"])
				}
			},
		},
		{
			"sub_with_string",
			`ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.s←'hello'`,
			func(t *testing.T, ns *codec.Namespace) {
				a := requireSubNs(t, ns, "a")
				if a.Values["s"] != "hello" {
					t.Errorf("a.s = %v, want 'hello'", a.Values["s"])
				}
			},
		},
		{
			"sub_with_dfn",
			`ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.fn←{⍵+1}`,
			func(t *testing.T, ns *codec.Namespace) {
				a := requireSubNs(t, ns, "a")
				raw, ok := a.Values["fn"].(Raw)
				if !ok {
					t.Fatalf("a.fn = %T, want Raw", a.Values["fn"])
				}
				if len(raw) == 0 {
					t.Errorf("a.fn Raw is empty")
				}
			},
		},
		{
			// Sub-namespace with both class-2 (x) and class-3 (fn).
			// Stresses skipFnBlob's literal-pool boundary detection:
			// x=42's sub-array sits adjacent to the dfn's literal pool
			// entry for `1` (both are size=4 Int8 scalars). We rely on
			// counting `XX 57` literal-pool references in the bytecode
			// to know exactly how many trailing sub-arrays belong to
			// the function.
			"sub_with_var_and_dfn",
			`ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.x←42 ⋄ ns.a.fn←{⍵+1}`,
			func(t *testing.T, ns *codec.Namespace) {
				a := requireSubNs(t, ns, "a")
				if a.Values["x"] != 42 {
					t.Errorf("a.x = %v, want 42", a.Values["x"])
				}
				if _, ok := a.Values["fn"].(Raw); !ok {
					t.Errorf("a.fn = %T, want Raw", a.Values["fn"])
				}
			},
		},
		{
			"two_level_deep",
			`ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.b←⎕NS '' ⋄ ns.a.b.z←'deep'`,
			func(t *testing.T, ns *codec.Namespace) {
				a := requireSubNs(t, ns, "a")
				b := requireSubNs(t, a, "b")
				if b.Values["z"] != "deep" {
					t.Errorf("a.b.z = %v, want 'deep'", b.Values["z"])
				}
			},
		},
		{
			"sibling_subns",
			`ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.b←⎕NS '' ⋄ ns.a.x←1 ⋄ ns.b.y←2`,
			func(t *testing.T, ns *codec.Namespace) {
				a := requireSubNs(t, ns, "a")
				b := requireSubNs(t, ns, "b")
				if a.Values["x"] != 1 {
					t.Errorf("a.x = %v, want 1", a.Values["x"])
				}
				if b.Values["y"] != 2 {
					t.Errorf("b.y = %v, want 2", b.Values["y"])
				}
			},
		},
		{
			"var_before_sub_with_dfn",
			// x is created first → x is LAST in name table → x is FIRST in
			// extraction order → sequential layout. Exercises the class-9
			// case in the !relocated branch (sub-ns appears later).
			`ns←⎕NS '' ⋄ ns.x←42 ⋄ ns.a←⎕NS '' ⋄ ns.a.fn←{⍵×2}`,
			func(t *testing.T, ns *codec.Namespace) {
				if ns.Values["x"] != 42 {
					t.Errorf("x = %v, want 42", ns.Values["x"])
				}
				a := requireSubNs(t, ns, "a")
				if _, ok := a.Values["fn"].(Raw); !ok {
					t.Errorf("a.fn = %T, want Raw", a.Values["fn"])
				}
			},
		},
		{
			"sub_with_dfn_before_var",
			// a is created first → a is LAST in name table → a is FIRST in
			// extraction order → relocated layout. The dfn inside `a` plus
			// the relocated var b together stress both code paths.
			`ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.fn←{⍵+1} ⋄ ns.b←99`,
			func(t *testing.T, ns *codec.Namespace) {
				if ns.Values["b"] != 99 {
					t.Errorf("b = %v, want 99", ns.Values["b"])
				}
				a := requireSubNs(t, ns, "a")
				if _, ok := a.Values["fn"].(Raw); !ok {
					t.Errorf("a.fn = %T, want Raw", a.Values["fn"])
				}
			},
		},
	}

	args := []string{"-l"}
	for i, tc := range cases {
		args = append(args, "-e", tc.setup)
		args = append(args, "-e", fmt.Sprintf("'=%d=' ⋄ 1(220⌶)⎕OR'ns'", i))
		args = append(args, "-e", ")erase ns")
	}

	out, err := exec.Command("gritt", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("gritt: %v\n%s", err, out)
	}
	content := string(out)

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := parseDelim(t, content, fmt.Sprintf("=%d=", i))
			val, err := Unmarshal(data)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			ns, ok := val.(*codec.Namespace)
			if !ok {
				t.Fatalf("Unmarshal returned %T, want *codec.Namespace", val)
			}
			tc.check(t, ns)
		})
	}
}

// TestNestedDfnRoundtrip verifies that a dfn inside a sub-namespace,
// extracted as Raw bytes, marshals back to the same byte sequence.
// This is the round-trip property promised by 220⌶: take bytes from
// Dyalog, parse them, regenerate them, get the original bytes back.
func TestNestedDfnRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	out, err := exec.Command("gritt",
		"-l",
		"-e", `ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.fn←{⍵+1}`,
		"-e", "'=NS=' ⋄ 1(220⌶)⎕OR'ns'",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("gritt: %v\n%s", err, out)
	}
	original := parseDelim(t, string(out), "=NS=")

	val, err := Unmarshal(original)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Walk down to the dfn's Raw bytes.
	ns, ok := val.(*codec.Namespace)
	if !ok {
		t.Fatalf("top is %T, want *codec.Namespace", val)
	}
	a, ok := ns.Values["a"].(*codec.Namespace)
	if !ok {
		t.Fatalf("a is %T, want *codec.Namespace", ns.Values["a"])
	}
	fn, ok := a.Values["fn"].(Raw)
	if !ok {
		t.Fatalf("fn is %T, want Raw", a.Values["fn"])
	}

	// Marshal the dfn Raw — it should round-trip byte-for-byte since Raw
	// values are returned verbatim by Marshal.
	out2, err := Marshal(fn)
	if err != nil {
		t.Fatalf("Marshal(fn): %v", err)
	}
	if !bytes.Equal(out2, []byte(fn)) {
		t.Errorf("Marshal(fn) didn't round-trip: %d vs %d bytes", len(out2), len(fn))
	}
}

// --- Helpers ---

// requireSubNs asserts ns.Values[key] is a *codec.Namespace and returns it.
func requireSubNs(t *testing.T, ns *codec.Namespace, key string) *codec.Namespace {
	t.Helper()
	v, present := ns.Values[key]
	if !present {
		t.Fatalf("missing key %q in namespace; keys=%v", key, ns.Keys)
	}
	sub, ok := v.(*codec.Namespace)
	if !ok {
		t.Fatalf("Values[%q] = %T, want *codec.Namespace", key, v)
	}
	return sub
}

// parseDelim extracts the bytes of a 220⌶ blob between a delimiter and
// the next "=" delimiter or end of output.
func parseDelim(t *testing.T, content, delim string) []byte {
	t.Helper()
	idx := strings.Index(content, delim)
	if idx < 0 {
		t.Fatalf("delimiter %q not found in output", delim)
	}
	after := strings.TrimLeft(content[idx+len(delim):], " \n\r")
	end := len(after)
	if ni := strings.Index(after, "\n="); ni >= 0 {
		end = ni
	}
	chunk := strings.TrimSpace(after[:end])
	chunk = strings.ReplaceAll(chunk, "¯", "-")
	fields := strings.Fields(chunk)
	data := make([]byte, len(fields))
	for i, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("parse byte %d %q: %v", i, f, err)
		}
		data[i] = byte(int8(v))
	}
	return data
}
