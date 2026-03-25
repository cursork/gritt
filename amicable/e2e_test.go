package amicable

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestE2ERoundtrip serializes values in Dyalog with 1(220⌶), unmarshals and
// re-marshals in Go, then sends back to Dyalog via 0(220⌶) to verify identity.
func TestE2ERoundtrip(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	cases := []struct {
		name string
		expr string // APL expression that produces the test value
	}{
		{"scalar_int", "42"},
		{"scalar_neg", "¯5"},
		{"scalar_zero", "0"},
		{"scalar_float", "3.14"},
		{"scalar_char", "'X'"},
		{"scalar_complex", "1J2"},
		{"scalar_int16", "1000"},
		{"scalar_int32", "1000000"},
		{"vec_int", "1 2 3"},
		{"vec_char", "'hello world'"},
		{"vec_bool", "1 0 1 1 0"},
		{"vec_float", "1.1 2.2 3.3"},
		{"vec_int16", "200 300 400"},
		{"vec_int32", "100000 200000 300000"},
		{"vec_complex", "1J2 3J4"},
		{"vec_unicode", "'⍳⍴⍬'"},
		{"vec_empty_char", "''"},
		{"vec_empty_num", "⍬"},
		{"mat_int", "2 3⍴⍳6"},
		{"mat_char", "2 3⍴'abcdef'"},
		{"mat_bool", "2 3⍴1 0 1 0 1 0"},
		{"mat_float", "2 2⍴1.1 2.2 3.3 4.4"},
		{"rank3", "2 3 4⍴⍳24"},
		{"nested_simple", "(1 2)(3 4)"},
		{"nested_mixed", "1 'hello' (2 3⍴⍳6)"},
	}

	// Step 1: Serialize all values in one Dyalog session.
	// Use a delimiter between outputs so we can split reliably.
	serArgs := []string{"-l"}
	for _, tc := range cases {
		// Print delimiter, then the signed byte vector
		serArgs = append(serArgs, "-e", fmt.Sprintf("'----%s----'", tc.name))
		serArgs = append(serArgs, "-e", fmt.Sprintf("1(220⌶)%s", tc.expr))
	}
	serOut, err := runGritt(serArgs...)
	if err != nil {
		t.Fatalf("serialization gritt failed: %v", err)
	}

	// Parse the serialized bytes for each case.
	type caseBytes struct {
		name  string
		expr  string
		bytes []byte
	}
	var parsed []caseBytes

	for _, tc := range cases {
		delim := fmt.Sprintf("----%s----", tc.name)
		idx := strings.Index(serOut, delim)
		if idx < 0 {
			t.Fatalf("delimiter %q not found in output", delim)
		}
		// Content is between this delimiter and the next (or end)
		after := serOut[idx+len(delim):]
		after = strings.TrimLeft(after, " \n\r")

		// Find end: next delimiter or end of string
		endIdx := strings.Index(after, "----")
		var content string
		if endIdx >= 0 {
			content = strings.TrimSpace(after[:endIdx])
		} else {
			content = strings.TrimSpace(after)
		}

		signedInts, err := parseAPLIntVector(content)
		if err != nil {
			t.Fatalf("case %s: parsing APL output: %v\nraw: %q", tc.name, err, content)
		}

		bs := make([]byte, len(signedInts))
		for i, v := range signedInts {
			bs[i] = byte(int8(v))
		}
		parsed = append(parsed, caseBytes{tc.name, tc.expr, bs})
	}

	// Step 2: Unmarshal and re-marshal each in Go, then verify in Dyalog.
	verArgs := []string{"-l"}
	for _, cb := range parsed {
		// Unmarshal
		val, err := Unmarshal(cb.bytes)
		if err != nil {
			t.Fatalf("case %s: Unmarshal failed: %v", cb.name, err)
		}

		// Re-marshal
		reser, err := Marshal(val)
		if err != nil {
			t.Fatalf("case %s: Marshal failed: %v", cb.name, err)
		}

		// Format as APL signed int vector
		aplVec := formatAsAPLVector(reser)

		// Verify in Dyalog: original ≡ 0(220⌶) re-serialized bytes
		verArgs = append(verArgs, "-e", fmt.Sprintf("'%s:' , ⍕ (%s) ≡ 0(220⌶) %s", cb.name, cb.expr, aplVec))
	}

	verOut, err := runGritt(verArgs...)
	if err != nil {
		t.Fatalf("verification gritt failed: %v", err)
	}

	// Check each result
	lines := strings.Split(strings.TrimSpace(verOut), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Each line should end with ":1" (APL ⍕ catenates without space)
		parts := strings.SplitN(line, ":", 2)
		name := "unknown"
		if len(parts) > 0 {
			name = strings.TrimSpace(parts[0])
		}
		result := ""
		if len(parts) > 1 {
			result = strings.TrimSpace(parts[1])
		}
		if result != "1" {
			t.Errorf("case %s: Dyalog says ≢ (got %q)", name, result)
		}
	}
}

func runGritt(args ...string) (string, error) {
	cmd := exec.Command("gritt", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, out)
	}
	return string(out), nil
}

// parseAPLIntVector parses space-separated signed integers from APL output.
// Handles ¯ as negative sign and multi-line output.
func parseAPLIntVector(s string) ([]int, error) {
	// Replace ¯ with -
	s = strings.ReplaceAll(s, "¯", "-")
	// Collapse whitespace
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty vector output")
	}

	parts := strings.Split(s, " ")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", p, err)
		}
		if v < -128 || v > 127 {
			return nil, fmt.Errorf("value %d out of sint range", v)
		}
		result = append(result, v)
	}
	return result, nil
}

// formatAsAPLVector converts bytes to a signed APL integer vector literal.
func formatAsAPLVector(data []byte) string {
	parts := make([]string, len(data))
	for i, b := range data {
		v := int8(b)
		if v < 0 {
			parts[i] = fmt.Sprintf("¯%d", -int(v))
		} else {
			parts[i] = strconv.Itoa(int(v))
		}
	}
	return strings.Join(parts, " ")
}
