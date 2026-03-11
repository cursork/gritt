// Incomplete parsing of Dyalog's string output in the session.
// Very much a 'give it your best shot' parser and not really reviewed, as it
// doesn't have much use as of now.
//
// APL output conventions:
//   - Negative numbers use high-minus: ¯42 not -42
//   - Vectors are space-separated on a single line: 1 2 3
//   - Matrices are newline-separated rows: 1 2\n3 4
//   - Empty vector is ⍬
//   - Complex numbers use J notation: 1J2
//   - Strings are single-quoted with doubled escaping: 'it”s'
//   - Scientific notation uses E: 1.5E10, 1.5E¯3
//
// Limitation: nested arrays, objects, and rank>2 arrays cannot be reliably
// parsed from display form. Use ⎕JSON for those.
package codec

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Scalar parses a single APL scalar value.
// Returns int, float64, complex128, string, or nil.
func Scalar(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if s == "⍬" {
		return []any{}
	}
	if v, ok := tryQuotedString(s); ok {
		return v
	}
	return parseScalar(s)
}

// Int parses APL output as an integer.
func Int(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = replaceHighMinus(s)
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int: %w", err)
	}
	return int(n), nil
}

// Float parses APL output as a float64.
func Float(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = replaceHighMinus(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float: %w", err)
	}
	return f, nil
}

// Complex parses APL J-notation (e.g. "1J2", "3.14J¯2.5") as complex128.
func Complex(s string) (complex128, error) {
	s = strings.TrimSpace(s)
	s = replaceHighMinus(s)
	r, i, ok := splitComplex(s)
	if !ok {
		return 0, fmt.Errorf("parse complex: no J separator in %q", s)
	}
	rr, err := strconv.ParseFloat(r, 64)
	if err != nil {
		return 0, fmt.Errorf("parse complex real part: %w", err)
	}
	ii, err := strconv.ParseFloat(i, 64)
	if err != nil {
		return 0, fmt.Errorf("parse complex imaginary part: %w", err)
	}
	return complex(rr, ii), nil
}

// String parses APL quoted string output, removing enclosing quotes
// and unescaping doubled quotes: 'it”s' -> it's
func String(s string) (string, error) {
	s = strings.TrimSpace(s)
	v, ok := tryQuotedString(s)
	if !ok {
		// Not quoted — return as-is (APL sometimes outputs unquoted strings)
		return s, nil
	}
	return v, nil
}

// Ints parses a space-separated line of APL integers into a slice.
func Ints(s string) ([]int, error) {
	tokens := tokenise(strings.TrimSpace(s))
	if len(tokens) == 0 {
		return nil, nil
	}
	result := make([]int, len(tokens))
	for i, t := range tokens {
		n, err := Int(t)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		result[i] = n
	}
	return result, nil
}

// Floats parses a space-separated line of APL numbers into float64s.
func Floats(s string) ([]float64, error) {
	tokens := tokenise(strings.TrimSpace(s))
	if len(tokens) == 0 {
		return nil, nil
	}
	result := make([]float64, len(tokens))
	for i, t := range tokens {
		n, err := Float(t)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		result[i] = n
	}
	return result, nil
}

// IntMatrix parses newline-separated rows of space-separated integers.
func IntMatrix(s string) ([][]int, error) {
	rows := splitRows(s)
	if len(rows) == 0 {
		return nil, nil
	}
	result := make([][]int, len(rows))
	for i, row := range rows {
		r, err := Ints(row)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i, err)
		}
		result[i] = r
	}
	return result, nil
}

// FloatMatrix parses newline-separated rows of space-separated floats.
func FloatMatrix(s string) ([][]float64, error) {
	rows := splitRows(s)
	if len(rows) == 0 {
		return nil, nil
	}
	result := make([][]float64, len(rows))
	for i, row := range rows {
		r, err := Floats(row)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i, err)
		}
		result[i] = r
	}
	return result, nil
}

// Auto parses APL output with automatic type detection.
// Returns:
//   - nil for empty/whitespace input
//   - []any{} for ⍬
//   - string for quoted strings
//   - int or float64 for single scalars
//   - []any for single-line vectors
//   - [][]any for multi-line matrices
func Auto(s string) any {
	s = strings.TrimRight(s, "\n\r")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if s == "⍬" {
		return []any{}
	}
	if v, ok := tryQuotedString(s); ok {
		return v
	}
	rows := splitRows(s)
	if len(rows) == 1 {
		return parseLine(rows[0])
	}
	// Multi-line: matrix
	result := make([][]any, len(rows))
	for i, row := range rows {
		v := parseLine(row)
		switch vv := v.(type) {
		case []any:
			result[i] = vv
		default:
			// Single-element row: wrap
			result[i] = []any{vv}
		}
	}
	return result
}

// parseLine parses a single line as a scalar or vector.
func parseLine(s string) any {
	tokens := tokenise(s)
	if len(tokens) == 0 {
		return nil
	}
	if len(tokens) == 1 {
		return parseScalar(tokens[0])
	}
	result := make([]any, len(tokens))
	for i, t := range tokens {
		result[i] = parseScalar(t)
	}
	return result
}

// parseScalar tries complex, then int, then float, then falls back to string.
func parseScalar(s string) any {
	norm := replaceHighMinus(s)

	// Complex
	if r, i, ok := splitComplex(norm); ok {
		rr, errR := strconv.ParseFloat(r, 64)
		ii, errI := strconv.ParseFloat(i, 64)
		if errR == nil && errI == nil {
			return complex(rr, ii)
		}
	}

	// Integer
	if n, err := strconv.ParseInt(norm, 10, 64); err == nil {
		if n >= math.MinInt && n <= math.MaxInt {
			return int(n)
		}
		return n
	}

	// Float
	if f, err := strconv.ParseFloat(norm, 64); err == nil {
		return f
	}

	// String fallback
	return s
}

// replaceHighMinus converts APL high-minus (¯) to ASCII minus (-).
func replaceHighMinus(s string) string {
	return strings.ReplaceAll(s, "¯", "-")
}

// splitComplex splits "1J2" or "1j2" into ("1", "2", true).
// Returns ("", "", false) if no J separator found.
func splitComplex(s string) (string, string, bool) {
	// Find J/j that isn't at the start (to avoid confusing with a variable name).
	// Must handle negative imaginary: "1J-2" (already high-minus-replaced).
	for i := 1; i < len(s); i++ {
		if s[i] == 'J' || s[i] == 'j' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

// tryQuotedString checks if s is a single-quoted APL string and extracts it.
// Returns ("", false) if not quoted.
func tryQuotedString(s string) (string, bool) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "", false
	}
	// Unescape doubled quotes: 'it''s' -> it's
	inner := s[1 : len(s)-1]
	return strings.ReplaceAll(inner, "''", "'"), true
}

// tokenise splits a line on whitespace, returning non-empty tokens.
func tokenise(s string) []string {
	return strings.Fields(s)
}

// splitRows splits multi-line output into rows, trimming trailing empty lines.
func splitRows(s string) []string {
	s = strings.TrimRight(s, "\n\r")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
