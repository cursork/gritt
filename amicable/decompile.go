package amicable

import (
	"fmt"
	"strings"
)

// Decompile attempts to reconstruct APL source code from a Raw ⎕OR blob.
// Returns the source string and any error. Works for dfns; tradfns and
// namespaces are not yet supported.
func (r Raw) Decompile() (string, error) {
	if len(r) < 20 {
		return "", fmt.Errorf("decompile: blob too short")
	}

	// Find the bytecode char8 vector (1F 27 type_rank marker)
	bc, err := r.bytecode()
	if err != nil {
		return "", err
	}

	// Find literal sub-arrays in the blob
	literals := r.extractLiterals()

	// Skip the 20-byte bytecode header, decode the token stream
	if len(bc) < 20 {
		return "", fmt.Errorf("decompile: bytecode too short")
	}
	tokens := bc[20:]

	// Extract only the expression regions between markers.
	// Everything outside XX 1B 6F ... XX 1E 6F is metadata.
	exprs := extractExpressions(tokens)

	var b strings.Builder
	b.WriteByte('{')

	for exprIdx, expr := range exprs {
		if exprIdx > 0 {
			b.WriteString(" ⋄ ")
		}
		toks := expr.tokens
		i := 0
		for i < len(toks) {
			// Guard marker within expression
			if expr.hasGuard && i == expr.guardPos {
				b.WriteByte(':')
			}

			tok := toks[i]

			// Two-byte references: index + type marker
			if i+1 < len(toks) {
				idx := tok
				typ := toks[i+1]

				switch typ {
				case 0x4C: // name/arg reference
					switch idx {
					case 0x00:
						b.WriteString("⍺")
					case 0x01:
						b.WriteString("⍵")
					case 0x02:
						b.WriteString("∇")
					default:
						name := r.lookupName(idx)
						if name != "" {
							b.WriteString(name)
						} else {
							b.WriteString(fmt.Sprintf("_v%d", idx))
						}
					}
					i += 2
					continue

				case 0x57: // literal pool reference
					if lit, ok := literals[idx]; ok {
						b.WriteString(formatLiteral(lit))
					} else {
						b.WriteString(fmt.Sprintf("_lit%d", idx))
					}
					i += 2
					continue

				case 0x3E: // system variable
					name := sysVarName(idx)
					b.WriteString(name)
					i += 2
					continue
				}

				// Operators applied to primitives: prim + operator
				if op, ok := operatorGlyph(typ); ok {
					if prim, ok := primitiveGlyph(tok); ok {
						b.WriteString(prim)
						b.WriteString(op)
						i += 2
						continue
					}
				}
			}

			// Single-byte tokens
			if glyph, ok := primitiveGlyph(tok); ok {
				b.WriteString(glyph)
				i++
				continue
			}
			if glyph, ok := syntaxGlyph(tok); ok {
				b.WriteString(glyph)
				i++
				continue
			}

			// Variable names: single-byte ASCII letters used as local names.
			// In the observed bytecode, variable 'r' = 0x72 (ASCII 'r').
			if tok >= 'A' && tok <= 'Z' || tok >= 'a' && tok <= 'z' || tok == '_' {
				b.WriteByte(tok)
				i++
				continue
			}

			// Unknown token — emit placeholder
			// Skip 0x00 (padding) and 0x01 (line-end marker) silently.
			if tok != 0x00 && tok != 0x01 {
				b.WriteString(fmt.Sprintf("«%02X»", tok))
			}
			i++
		}
	}

	b.WriteByte('}')
	return b.String(), nil
}

// expression represents a single expression extracted from the bytecode.
type expression struct {
	tokens   []byte
	hasGuard bool
	guardPos int // position of guard within tokens
}

// extractExpressions finds expression regions between 1B 6F .. 1E 6F markers.
// Also handles guard (:) markers (1C 6F) and diamond (⋄) markers (1D 6F).
func extractExpressions(tokens []byte) []expression {
	var exprs []expression

	// Find all marker positions: XX YY 6F where YY is 1B/1C/1D/1E/1F
	type marker struct {
		pos  int  // position of XX byte
		kind byte // 1B, 1C, 1D, 1E, 1F
	}
	var markers []marker
	for i := 0; i < len(tokens)-2; i++ {
		if tokens[i+2] == 0x6F {
			kind := tokens[i+1]
			if kind >= 0x1B && kind <= 0x1F {
				markers = append(markers, marker{i, kind})
			}
		}
	}

	// Extract tokens between consecutive markers
	for i := 0; i < len(markers); i++ {
		m := markers[i]
		if m.kind == 0x1B { // expression start
			// Collect tokens until next 1E (end), 1D (diamond), or 1C (guard)
			start := m.pos + 3 // skip the XX 1B 6F
			var expr expression

			for j := i + 1; j < len(markers); j++ {
				next := markers[j]
				switch next.kind {
				case 0x1C: // guard — split here
					expr.tokens = append([]byte{}, tokens[start:next.pos]...)
					expr.hasGuard = true
					expr.guardPos = len(expr.tokens)
					start = next.pos + 3
					continue
				case 0x1D: // diamond — this expression ends, next starts
					remaining := tokens[start:next.pos]
					if expr.hasGuard {
						expr.tokens = append(expr.tokens, remaining...)
					} else {
						expr.tokens = append([]byte{}, remaining...)
					}
					exprs = append(exprs, expr)
					expr = expression{}
					start = next.pos + 3
					continue
				case 0x1E, 0x1F: // expression end
					remaining := tokens[start:next.pos]
					if expr.hasGuard {
						expr.tokens = append(expr.tokens, remaining...)
					} else {
						expr.tokens = append([]byte{}, remaining...)
					}
					exprs = append(exprs, expr)
					i = j // advance outer loop
					goto nextExpr
				}
			}
		}
	nextExpr:
	}

	return exprs
}


// bytecode finds and returns the bytecode char8 vector from the blob.
func (r Raw) bytecode() ([]byte, error) {
	data := []byte(r)
	for j := 10; j < len(data)-1; j++ {
		if data[j] == 0x1F && data[j+1] == 0x27 { // char8 rank-1
			off := j - 8
			if off < 2 || off+24 > len(data) {
				continue
			}
			shape := int(le64(data[off+16:]))
			end := off + 24 + shape
			if end > len(data) {
				continue
			}
			bc := data[off+24 : end]
			// Bytecode starts with FF FF header
			if len(bc) >= 2 && bc[0] == 0xFF && bc[1] == 0xFF {
				return bc, nil
			}
		}
	}
	return nil, fmt.Errorf("decompile: no bytecode vector found in blob")
}

// extractLiterals scans the blob for numeric/string sub-arrays that serve
// as the literal pool. The bytecode references them via XX 57 where XX is
// a pool index (01, 03, 05... incrementing by 2).
//
// Heuristic: find all parseable sub-arrays after the bytecode, filter out
// known metadata (int16 220 = ⎕OR version marker, char8 = names/bytecode),
// and assign remaining values as literals in order.
func (r Raw) extractLiterals() map[byte]any {
	data := []byte(r)
	result := make(map[byte]any)

	// Find bytecode end
	bcEnd := 0
	for j := 10; j < len(data)-1; j++ {
		if data[j] == 0x1F && data[j+1] == 0x27 {
			off := j - 8
			shape := int(le64(data[off+16:]))
			bcEnd = off + 24 + ((shape+7)/8)*8
			break
		}
	}
	if bcEnd == 0 {
		return result
	}

	// Collect all parseable sub-arrays
	type found struct {
		off int
		val any
		end int
	}
	var candidates []found

	for j := bcEnd; j < len(data)-17; j++ {
		rankFlags := data[j+8]
		typeCode := data[j+9]
		flags := rankFlags & 0x0F
		rank := int(rankFlags >> 4)

		if flags != 0x0F || rank > 4 {
			continue
		}
		size := le64(data[j:])
		if size == 0 || size > 500 {
			continue
		}
		allZero := true
		for _, b := range data[j+10 : min(j+16, len(data))] {
			if b != 0 {
				allZero = false
			}
		}
		if !allZero {
			continue
		}

		subR := &reader{data: data, pos: j, ptrSize: 8}
		val, err := subR.readArray()
		if err != nil {
			continue
		}

		// Skip the bytecode char vector (already found above) and scalar chars (names)
		if typeCode == 0x27 && rank == 1 {
			s, isStr := val.(string)
			if isStr && len(s) >= 2 && s[0] == 0xFF && s[1] == 0xFF {
				// This is the bytecode, skip it
				j = subR.pos - 1
				continue
			}
			// Other char vectors are string literals — keep them
		}
		if (typeCode == 0x27 || typeCode == 0x28 || typeCode == 0x29) && rank == 0 {
			// Scalar chars are variable names, skip
			j = subR.pos - 1
			continue
		}

		candidates = append(candidates, found{j, val, subR.pos})
		j = subR.pos - 1
	}

	// The literal pool is stored in REVERSE order in the blob.
	// All sub-arrays (including int16(220) metadata) are part of the pool.
	// The LAST sub-array maps to pool index 0, second-to-last to index 1, etc.
	for i, c := range candidates {
		revIdx := byte(len(candidates) - 1 - i)
		result[revIdx] = c.val
	}

	return result
}

// lookupName searches the blob for char8 scalars that might be variable names.
func (r Raw) lookupName(idx byte) string {
	// Variable names appear as char8 scalars or short char vectors in the blob.
	// This is a heuristic scan — not all names may be found.
	data := []byte(r)
	nameIdx := byte(0)
	for j := 10; j < len(data)-17; j++ {
		rankFlags := data[j+8]
		typeCode := data[j+9]

		// Look for char8 scalars (0F 27) — single-char variable names
		if rankFlags == 0x0F && typeCode == 0x27 {
			size := le64(data[j:])
			if size == 4 { // scalar char: size=4
				allZero := true
				for _, b := range data[j+10 : min(j+16, len(data))] {
					if b != 0 {
						allZero = false
					}
				}
				if allZero && j+16 < len(data) {
					ch := data[j+16]
					if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_' {
						nameIdx++
						if nameIdx == idx {
							return string(ch)
						}
					}
				}
			}
		}
	}
	return ""
}

func le64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatLiteral renders a Go value as APL source.
func formatLiteral(v any) string {
	switch val := v.(type) {
	case int:
		if val < 0 {
			return fmt.Sprintf("¯%d", -val)
		}
		return fmt.Sprintf("%d", val)
	case float64:
		if val < 0 {
			return fmt.Sprintf("¯%g", -val)
		}
		return fmt.Sprintf("%g", val)
	case complex128:
		return fmt.Sprintf("%gJ%g", real(val), imag(val))
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
	case []any:
		parts := make([]string, len(val))
		for i, e := range val {
			parts[i] = formatLiteral(e)
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// primitiveGlyph maps a bytecode token to its APL primitive glyph.
func primitiveGlyph(tok byte) (string, bool) {
	m := map[byte]string{
		0x02: "+", 0x03: "-", 0x04: "×", 0x05: "÷",
		0x06: "⌈", 0x07: "⌊", 0x08: "*", 0x09: "⍟",
		0x0A: "|", 0x0B: "!", 0x0C: "○", 0x0E: "~",
		0x0F: "∨", 0x10: "∧", 0x11: "⍱", 0x12: "⍲",
		0x13: "<", 0x14: "≤", 0x15: "=", 0x16: "≥",
		0x17: ">", 0x18: "≠",
		0x1E: "≡", 0x1F: "≢",
		0x20: "⍴", 0x21: ",", 0x22: "⍪", 0x23: "⍳",
		0x24: "↑", 0x25: "↓", 0x26: "?", 0x27: "⍒",
		0x28: "⍋", 0x29: "⍉", 0x2A: "⌽", 0x2B: "⊖",
		0x2C: "∊", 0x2D: "⊥", 0x2E: "⊤", 0x2F: "⍎",
		0x30: "⍕", 0x31: "⌹", 0x32: "⊂", 0x33: "⊃",
		0x36: "⍷", 0x37: "⌷", 0x4F: "⊆",
	}
	g, ok := m[tok]
	return g, ok
}

// operatorGlyph maps a bytecode token to its APL operator glyph.
func operatorGlyph(tok byte) (string, bool) {
	m := map[byte]string{
		0x40: "/", 0x42: "\\", 0x44: ".",
		0x47: "¨", 0x4A: "⍨",
	}
	g, ok := m[tok]
	return g, ok
}

// syntaxGlyph maps a bytecode token to its APL syntax glyph.
func syntaxGlyph(tok byte) (string, bool) {
	m := map[byte]string{
		0x3A: "←", 0x3B: "⎕",
		0x38: "∘",
		0x60: "(", 0x61: ")", 0x62: "[", 0x63: "]",
	}
	g, ok := m[tok]
	return g, ok
}

// sysVarName maps a system variable index to its name.
func sysVarName(idx byte) string {
	m := map[byte]string{
		0x02: "⎕IO",
	}
	if name, ok := m[idx]; ok {
		return name
	}
	return fmt.Sprintf("⎕_sys%d", idx)
}
