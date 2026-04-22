package amicable

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Decompile attempts to reconstruct APL source code from a Raw ⎕OR blob.
// Works for dfns, tradfns, and namespaces (including nested).
func (r Raw) Decompile() (string, error) {
	if len(r) < 0x23 {
		return "", fmt.Errorf("decompile: blob too short")
	}

	// Byte 0x22 distinguishes namespaces (high nibble ≥ 0xA0) from functions.
	if r[0x22]&0xF0 >= 0xA0 {
		return r.decompileNamespace()
	}

	// Dfn: has FF FF bytecode header in a char8 vector
	if bc, err := r.findBytecode(); err == nil {
		return r.decompileDfn(bc)
	}

	// Tradfn: same tokens but different framing
	return r.decompileTradfn()
}

// --- Shared token decoder ---

// tokenContext holds everything needed to decode a token stream.
type tokenContext struct {
	names    map[byte]string // name index → name string
	literals map[byte]any    // literal pool index → value
	tradfn   bool            // true: 0x70+ are name table refs; false: ASCII var names
}

// collectExprLiteralRefs walks a single expression's token stream with
// the same alignment rules as decodeTokens and records every literal
// pool reference (XX 57 pair) into out.
//
// Must be called on EXPRESSION tokens (from extractExpressions), not raw
// bytecode: the header region between markers contains byte pairs that
// look like 0x57 refs but are never walked by decodeTokens.
func collectExprLiteralRefs(toks []byte, out map[byte]bool) {
	i := 0
	for i < len(toks) {
		if i+1 < len(toks) {
			typ := toks[i+1]
			// Two-byte "index + type marker" forms consume exactly two
			// bytes. 0x57 literal pool, 0x4C name/arg, 0x3E system variable.
			if typ == 0x57 {
				out[toks[i]] = true
				i += 2
				continue
			}
			if typ == 0x4C || typ == 0x3E {
				i += 2
				continue
			}
		}
		i++
	}
}

// decodeTokens decodes a slice of expression tokens into APL source.
func decodeTokens(toks []byte, ctx *tokenContext) string {
	var b strings.Builder
	i := 0
	for i < len(toks) {
		tok := toks[i]

		// Two-byte references: index + type marker
		if i+1 < len(toks) {
			typ := toks[i+1]

			switch typ {
			case 0x4C: // name/arg reference
				switch tok {
				case 0x00:
					b.WriteString("⍺")
				case 0x01:
					b.WriteString("⍵")
				case 0x02:
					b.WriteString("∇")
				default:
					if name, ok := ctx.names[tok]; ok {
						b.WriteString(name)
					} else {
						b.WriteString(fmt.Sprintf("_v%d", tok))
					}
				}
				i += 2
				continue

			case 0x57: // literal pool reference
				if lit, ok := ctx.literals[tok]; ok {
					b.WriteString(formatLiteral(lit))
				} else {
					b.WriteString(fmt.Sprintf("_lit%d", tok))
				}
				i += 2
				continue

			case 0x3E: // system variable
				b.WriteString(sysVarName(tok))
				i += 2
				continue
			}

			// Operator applied to primitive: prim + operator
			if op, ok := operatorGlyph(typ); ok {
				if prim, ok := primitiveGlyph(tok); ok {
					b.WriteString(prim)
					b.WriteString(op)
					i += 2
					continue
				}
			}
		}

		// Single-byte primitives and syntax
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

		// Tradfn name references (0x70+) — must check BEFORE ASCII handler
		// since 0x70+ overlaps with ASCII lowercase ('p'=0x70, 'q'=0x71, etc.)
		if ctx.tradfn && tok >= 0x70 {
			// Insert space before name if previous was a letter
			if s := b.String(); len(s) > 0 {
				runes := []rune(s)
				last := runes[len(runes)-1]
				if last >= 'A' && last <= 'Z' || last >= 'a' && last <= 'z' || last >= 0x2300 {
					b.WriteByte(' ')
				}
			}
			if name, ok := ctx.names[tok]; ok {
				b.WriteString(name)
			} else {
				b.WriteString(fmt.Sprintf("_n%d", tok-0x70))
			}
			i++
			continue
		}

		// Dfn ASCII variable names (inline in bytecode)
		if !ctx.tradfn && (tok >= 'A' && tok <= 'Z' || tok >= 'a' && tok <= 'z' || tok == '_') {
			b.WriteByte(tok)
			i++
			continue
		}

		// Skip padding (0x00) and line-end markers (0x01) silently
		if tok != 0x00 && tok != 0x01 {
			b.WriteString(fmt.Sprintf("«%02X»", tok))
		}
		i++
	}
	return b.String()
}

// --- Dfn decompiler ---

func (r Raw) decompileDfn(bc []byte) (string, error) {
	if len(bc) < 20 {
		return "", fmt.Errorf("decompile: bytecode too short")
	}

	literals := r.extractDfnLiterals()
	exprs := extractExpressions(bc[20:])

	ctx := &tokenContext{literals: literals}
	var b strings.Builder
	b.WriteByte('{')
	for i, expr := range exprs {
		if i > 0 {
			b.WriteString(" ⋄ ")
		}
		if expr.hasGuard {
			b.WriteString(decodeTokens(expr.tokens[:expr.guardPos], ctx))
			b.WriteByte(':')
			b.WriteString(decodeTokens(expr.tokens[expr.guardPos:], ctx))
		} else {
			b.WriteString(decodeTokens(expr.tokens, ctx))
		}
	}
	b.WriteByte('}')
	return b.String(), nil
}

// --- Tradfn decompiler ---

func (r Raw) decompileTradfn() (string, error) {
	data := []byte(r)

	tokenStart, tokenEnd := r.findTradfnTokens()
	if tokenStart < 0 {
		return "", fmt.Errorf("decompile: no token stream found (not a tradfn?)")
	}

	names := r.extractChar16Names(tokenStart)
	if len(names) == 0 {
		return "", fmt.Errorf("decompile: no name table found")
	}

	// Name map: indices from 0x70, reverse order
	nameMap := make(map[byte]string)
	for i, name := range names {
		nameMap[byte(0x70+len(names)-1-i)] = name
	}

	// Literal pool offset = number of names
	literals := r.extractTradfnLiterals()
	offsetLits := make(map[byte]any)
	for k, v := range literals {
		offsetLits[byte(len(names))+k] = v
	}

	ctx := &tokenContext{names: nameMap, literals: offsetLits, tradfn: true}

	// Split token stream at 0x67 boundaries for lines
	tokens := data[tokenStart:tokenEnd]
	var lines []string
	start := 0
	for i := 0; i <= len(tokens); i++ {
		if i == len(tokens) || tokens[i] == 0x67 {
			if i > start {
				line := decodeTradfnLine(tokens[start:i], ctx)
				if line != "" {
					lines = append(lines, line)
				}
			}
			start = i + 1
		}
	}

	return strings.Join(lines, "\n"), nil
}

// decodeTradfnLine decodes one line, handling keyword markers (XX YY 6F).
// Extracts keywords, then passes remaining tokens to the shared decoder.
func decodeTradfnLine(tokens []byte, ctx *tokenContext) string {
	var b strings.Builder
	// Strip keyword markers and structural 6F bytes, decode the rest
	var clean []byte
	i := 0
	for i < len(tokens) {
		// Skip padding
		if tokens[i] == 0x00 || tokens[i] == 0x01 {
			i++
			continue
		}
		// Keyword markers: XX YY 6F where YY is a keyword code
		if i+2 < len(tokens) && tokens[i+2] == 0x6F {
			if kw, ok := keywordGlyph(tokens[i+1]); ok {
				// Flush accumulated tokens
				if len(clean) > 0 {
					b.WriteString(decodeTokens(clean, ctx))
					clean = nil
				}
				b.WriteString(kw)
				i += 3
				continue
			}
		}
		// Structural XX 6F (skip)
		if i+1 < len(tokens) && tokens[i+1] == 0x6F {
			i += 2
			continue
		}
		clean = append(clean, tokens[i])
		i++
	}
	if len(clean) > 0 {
		// Space between keyword and expression (e.g. ":If b=0")
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(decodeTokens(clean, ctx))
	}
	return b.String()
}

// --- Namespace decompiler ---

type nsMember struct {
	name  string
	class int // 2=variable, 3=function, 9=namespace name
}

func (r Raw) decompileNamespace() (string, error) {
	members := r.extractNsMembers()

	nsName := ""
	var memberList []nsMember
	for _, m := range members {
		if m.class == 9 {
			nsName = m.name
		} else {
			memberList = append(memberList, m)
		}
	}

	values := r.extractNsValues(memberList)

	var b strings.Builder
	if nsName != "" {
		fmt.Fprintf(&b, ":Namespace %s\n", nsName)
	} else {
		b.WriteString(":Namespace\n")
	}
	for i, m := range memberList {
		b.WriteString("    ")
		b.WriteString(m.name)
		b.WriteString("←")
		if i < len(values) {
			b.WriteString(values[i])
		} else {
			b.WriteString("???")
		}
		b.WriteByte('\n')
	}
	b.WriteString(":EndNamespace")
	return b.String(), nil
}

// --- Sub-structure finders ---

// findBytecode finds the dfn bytecode char8 vector (FF FF header) in the blob.
func (r Raw) findBytecode() ([]byte, error) {
	data := []byte(r)
	for j := 10; j < len(data)-1; j++ {
		if data[j] == 0x1F && data[j+1] == 0x27 {
			off := j - 8
			if off < 2 || off+24 > len(data) {
				continue
			}
			allZero := true
			for _, b := range data[j+2 : min(j+8, len(data))] {
				if b != 0 {
					allZero = false
				}
			}
			if !allZero {
				continue
			}
			shape := int(le64(data[off+16:]))
			if shape <= 0 || off+24+shape > len(data) {
				continue
			}
			bc := data[off+24 : off+24+shape]
			if len(bc) >= 2 && bc[0] == 0xFF && bc[1] == 0xFF {
				return bc, nil
			}
		}
	}
	return nil, fmt.Errorf("decompile: no bytecode vector found")
}

// findBytecodeNear finds a bytecode vector near a given offset in the blob.
func (r Raw) findBytecodeNear(hint int) []byte {
	data := []byte(r)
	for j := max(hint-60, 10); j <= hint && j < len(data)-1; j++ {
		if data[j] == 0x1F && data[j+1] == 0x27 {
			off := j - 8
			if off < 0 {
				continue
			}
			allZero := true
			for _, b := range data[j+2 : min(j+8, len(data))] {
				if b != 0 {
					allZero = false
				}
			}
			if !allZero {
				continue
			}
			shape := int(le64(data[off+16:]))
			if shape > 0 && off+24+shape <= len(data) {
				bc := data[off+24 : off+24+shape]
				if len(bc) >= 2 && bc[0] == 0xFF && bc[1] == 0xFF {
					return bc
				}
			}
		}
	}
	return nil
}

// findTradfnTokens locates the tradfn token stream (starts with 0x67 + name byte).
func (r Raw) findTradfnTokens() (int, int) {
	data := []byte(r)
	for j := 0; j < len(data)-4; j++ {
		if data[j] == 0x67 && data[j+1] >= 0x70 {
			end := len(data)
			zeros := 0
			for k := j; k < len(data); k++ {
				if data[k] == 0x00 {
					zeros++
					if zeros >= 3 {
						end = k - 2
						break
					}
				} else {
					zeros = 0
				}
			}
			return j, end
		}
	}
	return -1, -1
}

// --- Name extraction ---

// extractChar16Names finds char16 name entries before a given offset.
// Pattern: 01 XX 00 88 00 00 00 00 [char16 data]
func (r Raw) extractChar16Names(limit int) []string {
	data := []byte(r)
	var names []string
	for j := 0; j < limit-12 && j < len(data)-12; j++ {
		if data[j] == 0x01 && data[j+2] == 0x00 && data[j+3] == 0x88 &&
			data[j+4] == 0x00 && data[j+5] == 0x00 && data[j+6] == 0x00 && data[j+7] == 0x00 {
			var runes []rune
			for k := j + 8; k < len(data)-1; k += 2 {
				ch := rune(data[k]) | rune(data[k+1])<<8
				if ch == 0 {
					break
				}
				runes = append(runes, ch)
			}
			if len(runes) > 0 {
				names = append(names, string(runes))
				j += 8 + len(runes)*2 - 1
			}
		}
	}
	return names
}

// extractNsMembers finds namespace member entries from the initial name table.
func (r Raw) extractNsMembers() []nsMember {
	data := []byte(r)
	var members []nsMember
	lastEnd := 0

	for j := 0; j < len(data)-12; j++ {
		if data[j] == 0x01 && data[j+2] == 0x00 && data[j+3] == 0x88 &&
			data[j+4] == 0x00 && data[j+5] == 0x00 && data[j+6] == 0x00 && data[j+7] == 0x00 {

			if lastEnd > 0 && j-lastEnd > 40 {
				break
			}

			classByte := data[j+1]
			class := int(classByte >> 4)

			var runes []rune
			for k := j + 8; k < len(data)-1; k += 2 {
				ch := rune(data[k]) | rune(data[k+1])<<8
				if ch == 0 {
					break
				}
				runes = append(runes, ch)
			}
			lastEnd = j + 8 + len(runes)*2 + 2

			// Skip sentinel (FFFF data)
			if len(runes) > 0 && runes[0] == 0xFFFF {
				j = lastEnd - 1
				continue
			}

			actualClass := class
			if classByte == 0x08 {
				actualClass = 9
			}

			if len(runes) > 0 {
				members = append(members, nsMember{name: string(runes), class: actualClass})
			}
			j = lastEnd - 1
		}
	}
	return members
}

// --- Literal pool extraction ---

// extractDfnLiterals finds the literal pool for a dfn.
// All sub-arrays after the bytecode, in reverse order.
func (r Raw) extractDfnLiterals() map[byte]any {
	data := []byte(r)

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
		return nil
	}

	candidates := scanSubArrays(data, bcEnd, len(data), true)

	// Reverse order
	result := make(map[byte]any)
	for i, c := range candidates {
		result[byte(len(candidates)-1-i)] = c
	}
	return result
}

// extractTradfnLiterals finds the literal pool for a tradfn.
func (r Raw) extractTradfnLiterals() map[byte]any {
	data := []byte(r)
	candidates := scanSubArrays(data, 2, len(data), false)

	result := make(map[byte]any)
	for i, c := range candidates {
		result[byte(len(candidates)-1-i)] = c
	}
	return result
}

// extractNsValues finds member values for namespace members.
func (r Raw) extractNsValues(members []nsMember) []string {
	data := []byte(r)

	var fnNames, varNames []string
	for _, m := range members {
		switch m.class {
		case 3:
			fnNames = append(fnNames, m.name)
		case 2:
			varNames = append(varNames, m.name)
		}
	}

	nameTableEnd := r.findNameTableEnd()

	// Find function values: scan for FF FF bytecodes after name table
	fnValues := make(map[string]string)
	pos := nameTableEnd
	for _, fnName := range fnNames {
		for j := pos; j < len(data)-1; j++ {
			if data[j] == 0xFF && data[j+1] == 0xFF {
				bc := r.findBytecodeNear(j)
				if bc != nil && len(bc) >= 20 {
					bcEndPos := j + len(bc)
					padded := ((bcEndPos + 7) / 8) * 8

					tokens := bc[20:]
					exprs := extractExpressions(tokens)

					// Walk only the expression token spans (not the full
					// bc[20:]) to discover literal pool references.
					// Embedded dfns use an offset of 2+num_locals — we
					// observe the minimum index actually referenced and
					// use it as the base.
					refIndices := make(map[byte]bool)
					for _, expr := range exprs {
						collectExprLiteralRefs(expr.tokens, refIndices)
					}
					var xBase int
					nLits := len(refIndices)
					if nLits > 0 {
						xBase = 255
						for idx := range refIndices {
							if int(idx) < xBase {
								xBase = int(idx)
							}
						}
					}

					// Wider scan window than the old 100-byte one: dfns
					// with several literals span up to ~40 bytes each and
					// the metadata tail sits ~250 bytes past the last
					// literal. 300 covers the realistic range.
					lits := scanSubArrays(data, padded, min(padded+300, len(data)), false)
					if nLits > 0 && len(lits) > nLits {
						lits = lits[:nLits]
					}
					litMap := make(map[byte]any)
					for i, c := range lits {
						litMap[byte(xBase+len(lits)-1-i)] = c
					}

					ctx := &tokenContext{literals: litMap}
					var sb strings.Builder
					sb.WriteByte('{')
					for ei, expr := range exprs {
						if ei > 0 {
							sb.WriteString(" ⋄ ")
						}
						if expr.hasGuard {
							sb.WriteString(decodeTokens(expr.tokens[:expr.guardPos], ctx))
							sb.WriteByte(':')
							sb.WriteString(decodeTokens(expr.tokens[expr.guardPos:], ctx))
						} else {
							sb.WriteString(decodeTokens(expr.tokens, ctx))
						}
					}
					sb.WriteByte('}')
					fnValues[fnName] = sb.String()
					pos = bcEndPos + 64
					break
				}
				j++ // skip this FF FF
			}
		}
	}

	// Find variable values: take exactly len(varNames) sub-arrays after functions
	varCandidates := scanSubArrays(data, pos, len(data), true)
	// Take the first N candidates (they come before metadata in the blob)
	// then reverse to match name-table order (blob stores values in reverse)
	var varValues []string
	for i := 0; i < len(varCandidates) && len(varValues) < len(varNames); i++ {
		varValues = append(varValues, formatLiteral(varCandidates[i]))
	}
	for i, j := 0, len(varValues)-1; i < j; i, j = i+1, j-1 {
		varValues[i], varValues[j] = varValues[j], varValues[i]
	}

	// Build final values matching member order
	var values []string
	varIdx := 0
	for _, m := range members {
		switch m.class {
		case 2:
			if varIdx < len(varValues) {
				values = append(values, varValues[varIdx])
				varIdx++
			} else {
				values = append(values, "???")
			}
		case 3:
			if v, ok := fnValues[m.name]; ok {
				values = append(values, v)
			} else {
				values = append(values, "{???}")
			}
		}
	}
	return values
}

func (r Raw) findNameTableEnd() int {
	data := []byte(r)
	end := 0
	for j := 0; j < len(data)-12; j++ {
		if data[j] == 0x01 && data[j+2] == 0x00 && data[j+3] == 0x88 &&
			data[j+4] == 0x00 && data[j+5] == 0x00 && data[j+6] == 0x00 && data[j+7] == 0x00 {
			if end > 0 && j-end > 40 {
				break
			}
			for k := j + 8; k < len(data)-1; k += 2 {
				ch := rune(data[k]) | rune(data[k+1])<<8
				if ch == 0 {
					end = k + 2
					break
				}
			}
			j = end - 1
		}
	}
	if end%8 != 0 {
		end += 8 - (end % 8)
	}
	return end
}

// scanSubArrays finds parseable sub-arrays in a byte range.
// If skipStrings is true, char8 vectors that look like bytecode are excluded.
// Only collects numeric scalars and (optionally) string values.
func scanSubArrays(data []byte, from, to int, includeStrings bool) []any {
	var result []any
	for j := from; j < to-17 && j < len(data)-17; j++ {
		rf := data[j+8]
		tc := data[j+9]
		flags := rf & 0x0F
		rank := int(rf >> 4)
		if flags != 0x0F || rank > 4 {
			continue
		}
		size := binary.LittleEndian.Uint64(data[j:])
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

		// Skip bytecode char8 vectors (start with FF FF)
		if tc == 0x27 && rank == 1 {
			if s, ok := val.(string); ok && len(s) >= 2 && s[0] == 0xFF && s[1] == 0xFF {
				j = subR.pos - 1
				continue
			}
		}
		// Skip char scalars (names)
		if (tc == 0x27 || tc == 0x28 || tc == 0x29) && rank == 0 {
			j = subR.pos - 1
			continue
		}
		// Include or skip string vectors
		if tc == 0x27 || tc == 0x28 || tc == 0x29 {
			if includeStrings {
				result = append(result, val)
			}
			j = subR.pos - 1
			continue
		}

		switch val.(type) {
		case int, float64, complex128:
			result = append(result, val)
		case string:
			if includeStrings {
				result = append(result, val)
			}
		}
		j = subR.pos - 1
	}
	return result
}

// --- Expression extraction ---

type expression struct {
	tokens   []byte
	hasGuard bool
	guardPos int
}

// extractExpressions finds the first expression group between 1B 6F and 1E 6F markers.
func extractExpressions(tokens []byte) []expression {
	type marker struct {
		pos  int
		kind byte
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

	var exprs []expression
	for i := 0; i < len(markers); i++ {
		m := markers[i]
		if m.kind != 0x1B {
			continue
		}
		start := m.pos + 3
		var expr expression

		for j := i + 1; j < len(markers); j++ {
			next := markers[j]
			switch next.kind {
			case 0x1C: // guard
				expr.tokens = append([]byte{}, tokens[start:next.pos]...)
				expr.hasGuard = true
				expr.guardPos = len(expr.tokens)
				start = next.pos + 3
				continue
			case 0x1D: // diamond
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
			case 0x1E, 0x1F: // end — stop after first group
				remaining := tokens[start:next.pos]
				if expr.hasGuard {
					expr.tokens = append(expr.tokens, remaining...)
				} else {
					expr.tokens = append([]byte{}, remaining...)
				}
				exprs = append(exprs, expr)
				return exprs
			}
		}
	}
	return exprs
}

// --- Helpers ---

func le64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

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

func primitiveGlyph(tok byte) (string, bool) {
	m := map[byte]string{
		0x02: "+", 0x03: "-", 0x04: "×", 0x05: "÷",
		0x06: "⌈", 0x07: "⌊", 0x08: "*", 0x09: "⍟",
		0x0A: "|", 0x0B: "!", 0x0C: "○", 0x0E: "~",
		0x0F: "∨", 0x10: "∧", 0x11: "⍱", 0x12: "⍲",
		0x13: "<", 0x14: "≤", 0x15: "=", 0x16: "≥",
		0x17: ">", 0x18: "≠", 0x1C: ".",
		0x1E: "≡", 0x1F: "≢",
		0x20: "⍴", 0x21: ",", 0x22: "⍪", 0x23: "⍳",
		0x24: "↑", 0x25: "↓", 0x26: "?", 0x27: "⍒",
		0x28: "⍋", 0x29: "⍉", 0x2A: "⌽", 0x2B: "⊖",
		0x2C: "∊", 0x2D: "⊥", 0x2E: "⊤", 0x2F: "⍎",
		0x30: "⍕", 0x31: "⌹", 0x32: "⊂", 0x33: "⊃",
		0x34: "∪", 0x35: "∩", 0x36: "⍷", 0x37: "⌷",
		0x4F: "⊆", 0x50: "⍥", 0x52: "⊣", 0x53: "⊢",
		0x5C: "⍸", 0x5D: "@",
	}
	g, ok := m[tok]
	return g, ok
}

func operatorGlyph(tok byte) (string, bool) {
	m := map[byte]string{
		0x40: "/", 0x41: "⌿", 0x42: "\\", 0x43: "⍀",
		0x44: ".", 0x47: "¨", 0x48: "⍣", 0x4A: "⍨",
		0x54: "⍠", 0x55: "⍤", 0x59: "⌸", 0x5B: "⌺",
	}
	g, ok := m[tok]
	return g, ok
}

func syntaxGlyph(tok byte) (string, bool) {
	m := map[byte]string{
		0x3A: "←", 0x3B: "⎕",
		0x38: "∘",
		0x60: "(", 0x61: ")", 0x62: "[", 0x63: "]",
	}
	g, ok := m[tok]
	return g, ok
}

func sysVarName(idx byte) string {
	m := map[byte]string{
		0x02: "⎕IO",
	}
	if name, ok := m[idx]; ok {
		return name
	}
	return fmt.Sprintf("⎕_sys%d", idx)
}

func keywordGlyph(code byte) (string, bool) {
	m := map[byte]string{
		0x00: ":If", 0x01: ":While", 0x02: ":Repeat", 0x03: ":For",
		0x04: ":Else", 0x05: ":EndIf", 0x06: ":EndWhile",
		0x07: ":EndRepeat", 0x08: ":EndFor",
		0x09: ":Select", 0x0A: ":Case", 0x0B: ":EndSelect",
	}
	g, ok := m[code]
	return g, ok
}
