package codec

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// SerializeOptions controls APLAN output formatting.
type SerializeOptions struct {
	UseDiamond bool // use ⋄ instead of newlines
	Indent     int  // spaces per indent level (default 1)
}

// Serialize converts a Go value to an APLAN string.
//
// Accepted types:
//   - int, float64 → number (¯ for negative)
//   - complex128 → J-notation (3J4)
//   - string → single-quoted ('hello')
//   - []any → vector (strand for all-numeric, parenthesized otherwise)
//   - *Array → bracketed matrix
//   - *Namespace → parenthesized namespace
//   - *zilde / Zilde → ⍬
func Serialize(value any, opts ...SerializeOptions) string {
	opt := SerializeOptions{Indent: 1}
	if len(opts) > 0 {
		opt = opts[0]
		if opt.Indent == 0 {
			opt.Indent = 1
		}
	}
	return serializeValue(value, 0, &opt)
}

func serializeValue(value any, depth int, opt *SerializeOptions) string {
	switch v := value.(type) {
	case nil:
		return "⍬"
	case *zilde:
		return "⍬"
	case int:
		return serializeInt(v)
	case float64:
		return serializeFloat(v)
	case complex128:
		return serializeComplex(v)
	case string:
		return serializeString(v)
	case FnSource:
		// Raw APL source — pass through unquoted.
		return string(v)
	case *Array:
		return serializeMatrix(v, depth, opt)
	case *Namespace:
		return serializeNamespace(v, depth, opt)
	case []any:
		return serializeVector(v, depth, opt)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func serializeInt(n int) string {
	if n < 0 {
		return "¯" + strconv.Itoa(-n)
	}
	return strconv.Itoa(n)
}

func serializeFloat(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		// Can't represent these in APLAN
		return fmt.Sprintf("%v", f)
	}
	if f == 0 {
		return "0"
	}
	s := strconv.FormatFloat(f, 'G', -1, 64)
	s = strings.ReplaceAll(s, "-", "¯")
	// Go uses + for positive exponents; APLAN doesn't
	s = strings.ReplaceAll(s, "E+", "E")
	return s
}

func serializeComplex(c complex128) string {
	re := real(c)
	im := imag(c)
	if im == 0 {
		if re == float64(int64(re)) {
			return serializeInt(int(int64(re)))
		}
		return serializeFloat(re)
	}
	var reStr, imStr string
	if re == float64(int64(re)) {
		reStr = serializeInt(int(int64(re)))
	} else {
		reStr = serializeFloat(re)
	}
	if im == float64(int64(im)) {
		imStr = serializeInt(int(int64(im)))
	} else {
		imStr = serializeFloat(im)
	}
	return reStr + "J" + imStr
}

func serializeString(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}

func serializeVector(arr []any, depth int, opt *SerializeOptions) string {
	if len(arr) == 0 {
		return "⍬"
	}

	// 1-element vector: must use separator to distinguish from scalar.
	// In APL, (⋄ 42) is a 1-element vector, 42 is a scalar.
	if len(arr) == 1 {
		return "(⋄ " + serializeValue(arr[0], depth+1, opt) + ")"
	}

	// All numbers, 2+ elements → strand notation (no parens needed)
	if allNumbers(arr) {
		parts := make([]string, len(arr))
		for i, el := range arr {
			parts[i] = serializeValue(el, depth+1, opt)
		}
		return strings.Join(parts, " ")
	}

	// Mixed content → parenthesized
	items := make([]string, len(arr))
	for i, el := range arr {
		items[i] = serializeValue(el, depth+1, opt)
	}

	if opt.UseDiamond || hasNewlines(items) {
		return "(" + strings.Join(items, " ⋄ ") + ")"
	}

	return applyIndent("(", ")", items, depth, opt)
}

func serializeMatrix(m *Array, depth int, opt *SerializeOptions) string {
	if len(m.Shape) == 0 || m.Shape[0] == 0 {
		return "[]"
	}

	// Normalise Data via Shape so both layouts are supported:
	//   - nested rows: Data[i] is a row slice (codec.APLAN parser output)
	//   - fully flat:  Data holds all elements in row-major order
	//                  (amicable.Unmarshal output for rank ≥ 2 arrays)
	// flattenValue on an *Array walks Data recursively, giving a uniform
	// flat view in either case.
	flat := flattenValue(m)

	numRows := m.Shape[0]
	cellSize := 1
	for _, d := range m.Shape[1:] {
		cellSize *= d
	}

	rows := make([]string, numRows)

	if len(m.Shape) > 2 {
		// Higher rank: each major cell is itself an array with shape Shape[1:]
		subShape := make([]int, len(m.Shape)-1)
		copy(subShape, m.Shape[1:])
		for i := 0; i < numRows; i++ {
			start := i * cellSize
			end := start + cellSize
			if end > len(flat) {
				end = len(flat)
			}
			sub := &Array{Data: flat[start:end], Shape: subShape}
			rows[i] = serializeValue(sub, depth+1, opt)
		}
	} else {
		// Rank 2: each row is cellSize contiguous scalars
		for i := 0; i < numRows; i++ {
			parts := make([]string, cellSize)
			for j := 0; j < cellSize; j++ {
				idx := i*cellSize + j
				if idx >= len(flat) {
					parts[j] = ""
					continue
				}
				parts[j] = serializeValue(flat[idx], depth+1, opt)
			}
			rows[i] = strings.Join(parts, " ")
		}
	}

	if opt.UseDiamond {
		return "[" + strings.Join(rows, " ⋄ ") + "]"
	}

	return applyIndent("[", "]", rows, depth, opt)
}

func serializeNamespace(ns *Namespace, depth int, opt *SerializeOptions) string {
	if len(ns.Keys) == 0 {
		return "()"
	}

	items := make([]string, len(ns.Keys))
	for i, k := range ns.Keys {
		items[i] = k + ": " + serializeValue(ns.Values[k], depth+1, opt)
	}

	if opt.UseDiamond {
		return "(" + strings.Join(items, " ⋄ ") + ")"
	}

	return applyIndent("(", ")", items, depth, opt)
}

func applyIndent(open, close string, items []string, depth int, opt *SerializeOptions) string {
	inner := strings.Repeat(" ", (depth+1)*opt.Indent)
	outer := strings.Repeat(" ", depth*opt.Indent)
	return open + "\n" + inner + strings.Join(items, "\n"+inner) + "\n" + outer + close
}

func allNumbers(arr []any) bool {
	for _, el := range arr {
		switch el.(type) {
		case int, float64, complex128:
			continue
		default:
			return false
		}
	}
	return true
}

func hasNewlines(items []string) bool {
	for _, s := range items {
		if strings.Contains(s, "\n") {
			return true
		}
	}
	return false
}
