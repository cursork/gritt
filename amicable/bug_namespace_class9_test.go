package amicable

// Reproduction for the class-9-member-filtering bug in unmarshalNamespace.
// See BUG_namespace_class9.md.
//
// This test currently FAILS — class-9 members of a namespace are silently
// dropped during Unmarshal because of the filter at amicable.go:111. When
// the bug is fixed, this test should pass without modification.

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/cursork/gritt/codec"
)

// TestUnmarshalNestedNamespace asserts that a namespace whose member is
// itself a namespace round-trips through Unmarshal with the nested
// namespace intact. Mirrors the style of TestUnmarshalNamespace in
// decompile_test.go.
//
// APL setup:
//
//	ns←⎕NS '' ⋄ ns.x←42 ⋄ ns.y←⎕NS '' ⋄ ns.y.z←'hello'
//
// Expected:
//
//	ns.Values["x"] == 42
//	ns.Values["y"] is *codec.Namespace with Values["z"] == "hello"
//
// Actual today:
//
//	ns.Values["y"] is missing — the class-9 filter at amicable.go:111
//	drops it before values are extracted.
func TestUnmarshalNestedNamespace(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	const delim = "=NS="
	args := []string{
		"-l",
		"-e", `ns←⎕NS '' ⋄ ns.x←42 ⋄ ns.y←⎕NS '' ⋄ ns.y.z←'hello'`,
		"-e", fmt.Sprintf("'%s' ⋄ 1(220⌶)⎕OR'ns'", delim),
	}

	out, err := exec.Command("gritt", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("gritt: %v\n%s", err, out)
	}

	content := string(out)
	idx := strings.Index(content, delim)
	if idx < 0 {
		t.Fatalf("delimiter %q not found in output:\n%s", delim, content)
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

	val, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	ns, ok := val.(*codec.Namespace)
	if !ok {
		t.Fatalf("Unmarshal returned %T, want *codec.Namespace", val)
	}

	// `x` is a class-2 variable; the existing code handles this correctly.
	if ns.Values["x"] != 42 {
		t.Errorf("Values[\"x\"] = %v (%T), want 42", ns.Values["x"], ns.Values["x"])
	}

	// `y` is the bug. Today this fails — y is missing from Values entirely.
	yAny, present := ns.Values["y"]
	if !present {
		t.Fatalf("Values[\"y\"] missing — class-9 member dropped (the bug). See BUG_namespace_class9.md.")
	}
	yNs, ok := yAny.(*codec.Namespace)
	if !ok {
		t.Fatalf("Values[\"y\"] = %T, want *codec.Namespace", yAny)
	}
	if yNs.Values["z"] != "hello" {
		t.Errorf("Values[\"y\"].Values[\"z\"] = %v, want \"hello\"", yNs.Values["z"])
	}
}
