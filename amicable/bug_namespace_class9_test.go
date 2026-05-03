package amicable

// Tests for the class-9-member-filtering bug in unmarshalNamespace.
// See BUG_namespace_class9.md.
//
// TestUnmarshalNestedNamespace covers the easy case (nested ns is the LAST
// member processed in extraction order, so end-of-sub-blob detection isn't
// needed). TestUnmarshalNsThenVar covers the harder case where a variable
// follows the nested ns in extraction order — requires correctly advancing
// past the entire sub-blob.

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

// TestUnmarshalNsThenVar covers the case where a variable follows a sub-
// namespace in extraction order. APL setup:
//
//	ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.z←'hi' ⋄ ns.b←99
//
// In Dyalog's name-table layout, b appears before a. Extraction order
// (reverse) is therefore [a, b]: the nested namespace `a` is parsed FIRST,
// then `b`. The initial bug fix does not pass this test.
//
// Investigation: in this case Dyalog does NOT lay out values sequentially
// after the name table the way it does for all-class-2 namespaces. With
// `ns←⎕NS '' ⋄ ns.b←99`, b=99 sits at offset 0xDA (right after name table
// end 0xD0). With this test's setup, b=99 sits at offset 0x8B2 — far past
// the parent's settings/translation blocks (which start at 0x1BA, 0x2AA,
// 0x2F2). The simple "advance pos past nested ns sub-blob" approach fails
// because b's value isn't immediately after a's sub-blob; it's relocated
// somewhere inside the parent's metadata region.
//
// Resolving this needs more reverse-engineering of Dyalog's value-table
// layout when class-9 members are present. Tracked in FACIENDA.md.
func TestUnmarshalNsThenVar(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	const delim = "=NS="
	args := []string{
		"-l",
		"-e", `ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.z←'hi' ⋄ ns.b←99`,
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

	// b is class 2 (variable). It must be readable AFTER the nested namespace
	// has been correctly traversed and its end position found.
	bAny, present := ns.Values["b"]
	if !present {
		t.Fatalf("Values[\"b\"] missing — extraction did not advance past nested ns sub-blob")
	}
	if bAny != 99 {
		t.Errorf("Values[\"b\"] = %v (%T), want 99", bAny, bAny)
	}

	// a is class 9 (sub-namespace). Same checks as TestUnmarshalNestedNamespace.
	aAny, present := ns.Values["a"]
	if !present {
		t.Fatalf("Values[\"a\"] missing")
	}
	aNs, ok := aAny.(*codec.Namespace)
	if !ok {
		t.Fatalf("Values[\"a\"] = %T, want *codec.Namespace", aAny)
	}
	if aNs.Values["z"] != "hi" {
		t.Errorf("Values[\"a\"].Values[\"z\"] = %v, want \"hi\"", aNs.Values["z"])
	}
}

// TestUnmarshalNsThenTwoVars covers the relocated-tail layout with multiple
// class-2 values. APL setup:
//
//	ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.z←'hi' ⋄ ns.b←99 ⋄ ns.c←77
//
// Values must come back in the right name → value association: b=99, c=77.
// Catches off-by-one bugs in the tail-walk + reversal.
func TestUnmarshalNsThenTwoVars(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	const delim = "=NS="
	args := []string{
		"-l",
		"-e", `ns←⎕NS '' ⋄ ns.a←⎕NS '' ⋄ ns.a.z←'hi' ⋄ ns.b←99 ⋄ ns.c←77`,
		"-e", fmt.Sprintf("'%s' ⋄ 1(220⌶)⎕OR'ns'", delim),
	}

	out, err := exec.Command("gritt", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("gritt: %v\n%s", err, out)
	}

	content := string(out)
	idx := strings.Index(content, delim)
	if idx < 0 {
		t.Fatalf("delimiter %q not found", delim)
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
			t.Fatalf("parse byte %d: %v", i, err)
		}
		data[i] = byte(int8(v))
	}

	val, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	ns, ok := val.(*codec.Namespace)
	if !ok {
		t.Fatalf("Unmarshal returned %T", val)
	}

	if ns.Values["b"] != 99 {
		t.Errorf("Values[\"b\"] = %v, want 99", ns.Values["b"])
	}
	if ns.Values["c"] != 77 {
		t.Errorf("Values[\"c\"] = %v, want 77", ns.Values["c"])
	}
	aNs, ok := ns.Values["a"].(*codec.Namespace)
	if !ok {
		t.Fatalf("Values[\"a\"] = %T, want *codec.Namespace", ns.Values["a"])
	}
	if aNs.Values["z"] != "hi" {
		t.Errorf("Values[\"a\"].Values[\"z\"] = %v, want \"hi\"", aNs.Values["z"])
	}
}
