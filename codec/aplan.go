// A WIP parser for Dyalog APL's APLAN format.
// Tries to support the APL concept or arrays with shape.
package codec

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Array represents an APLAN matrix or higher-rank array with shape metadata.
type Array struct {
	Data  []any // flat or nested elements
	Shape []int // e.g. [2,3] for a 2x3 matrix
}

// Namespace represents an APLAN namespace (key-value object).
type Namespace struct {
	Keys   []string       // ordered keys
	Values map[string]any // key -> value
}

// FnSource holds raw APL source text for a function member of a namespace.
// It exists so Serialize can render function bodies inline without quoting
// (APLAN has no native function representation — this is a grittles-ism).
type FnSource string

// Zilde is the sentinel for APL's empty numeric vector (⍬).
var Zilde = &zilde{}

type zilde struct{}

// APLAN parses an APLAN (Array Notation) string into Go values.
//
// Returns:
//   - int or float64 for numeric scalars
//   - complex128 for complex numbers (unless imaginary is 0)
//   - string for character scalars and character vectors
//   - []any for vectors
//   - *Array for matrices and higher-rank arrays
//   - *Namespace for namespaces
//   - *zilde (Zilde) for ⍬
func APLAN(source string) (any, error) {
	tokens, err := aplanTokenise(source)
	if err != nil {
		return nil, err
	}
	p := &aplanParser{tokens: tokens}
	result, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipSep()
	if p.pos < len(p.tokens) {
		return nil, fmt.Errorf("unexpected token after value: %v", p.tokens[p.pos])
	}
	return result, nil
}

// --- Tokeniser ---

type aplanTokenKind int

const (
	tokNumber aplanTokenKind = iota
	tokString
	tokZilde
	tokName
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokColon
	tokSep // ⋄ or newline
)

type aplanToken struct {
	kind aplanTokenKind
	text string
}

func (t aplanToken) String() string {
	return fmt.Sprintf("{%d %q}", t.kind, t.text)
}

func aplanTokenise(source string) ([]aplanToken, error) {
	var tokens []aplanToken
	runes := []rune(source)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Whitespace (not newline)
		if ch == ' ' || ch == '\t' {
			i++
			continue
		}

		// Separators
		if ch == '⋄' || ch == '\n' || ch == '\r' || ch == '\u0085' {
			// Collapse consecutive separators into one token
			if len(tokens) == 0 || tokens[len(tokens)-1].kind == tokSep {
				i++
				continue
			}
			tokens = append(tokens, aplanToken{tokSep, string(ch)})
			i++
			continue
		}

		// Single-char tokens
		switch ch {
		case '(':
			tokens = append(tokens, aplanToken{tokLParen, "("})
			i++
			continue
		case ')':
			tokens = append(tokens, aplanToken{tokRParen, ")"})
			i++
			continue
		case '[':
			tokens = append(tokens, aplanToken{tokLBracket, "["})
			i++
			continue
		case ']':
			tokens = append(tokens, aplanToken{tokRBracket, "]"})
			i++
			continue
		case ':':
			tokens = append(tokens, aplanToken{tokColon, ":"})
			i++
			continue
		case '⍬':
			tokens = append(tokens, aplanToken{tokZilde, "⍬"})
			i++
			continue
		}

		// String literal
		if ch == '\'' {
			str, end, err := aplanReadString(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, aplanToken{tokString, str})
			i = end
			continue
		}

		// Number: starts with digit, ¯, or .
		if isNumberStart(ch) {
			num, end := aplanReadNumber(runes, i)
			tokens = append(tokens, aplanToken{tokNumber, num})
			i = end
			continue
		}

		// Name (identifier)
		if isNameStart(ch) {
			name, end := aplanReadName(runes, i)
			tokens = append(tokens, aplanToken{tokName, name})
			i = end
			continue
		}

		return nil, fmt.Errorf("unexpected character %q at position %d", string(ch), i)
	}

	// Remove trailing separator
	if len(tokens) > 0 && tokens[len(tokens)-1].kind == tokSep {
		tokens = tokens[:len(tokens)-1]
	}

	return tokens, nil
}

func isNumberStart(ch rune) bool {
	return (ch >= '0' && ch <= '9') || ch == '¯' || ch == '.'
}

func isNameStart(ch rune) bool {
	return ch == '∆' || ch == '⍙' || ch == '_' ||
		(ch >= 'A' && ch <= 'Z') || ch >= 'a' && ch <= 'z' ||
		(ch > 127 && unicode.IsLetter(ch))
}

func isNameContinue(ch rune) bool {
	return isNameStart(ch) || (ch >= '0' && ch <= '9')
}

// aplanReadString reads a single-quoted string from position i (at the opening quote).
// Returns the unescaped content and the position after the closing quote.
func aplanReadString(runes []rune, i int) (string, int, error) {
	i++ // skip opening quote
	var buf strings.Builder
	for i < len(runes) {
		if runes[i] == '\'' {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				buf.WriteRune('\'')
				i += 2
				continue
			}
			return buf.String(), i + 1, nil
		}
		buf.WriteRune(runes[i])
		i++
	}
	return "", i, fmt.Errorf("unterminated string literal")
}

// aplanReadNumber reads a number token (may include ¯, ., E, J).
func aplanReadNumber(runes []rune, i int) (string, int) {
	start := i
	for i < len(runes) && isNumberChar(runes[i]) {
		i++
	}
	return string(runes[start:i]), i
}

func isNumberChar(ch rune) bool {
	return (ch >= '0' && ch <= '9') || ch == '.' || ch == '¯' ||
		ch == 'E' || ch == 'e' || ch == 'J' || ch == 'j'
}

func aplanReadName(runes []rune, i int) (string, int) {
	start := i
	for i < len(runes) && isNameContinue(runes[i]) {
		i++
	}
	return string(runes[start:i]), i
}

// --- Parser ---

type aplanParser struct {
	tokens []aplanToken
	pos    int
}

func (p *aplanParser) peek() (aplanToken, bool) {
	if p.pos >= len(p.tokens) {
		return aplanToken{}, false
	}
	return p.tokens[p.pos], true
}

func (p *aplanParser) advance() aplanToken {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *aplanParser) expect(kind aplanTokenKind) (aplanToken, error) {
	t, ok := p.peek()
	if !ok {
		return aplanToken{}, fmt.Errorf("unexpected end of input, expected %d", kind)
	}
	if t.kind != kind {
		return aplanToken{}, fmt.Errorf("expected token kind %d, got %v", kind, t)
	}
	p.advance()
	return t, nil
}

func (p *aplanParser) skipSep() {
	for {
		t, ok := p.peek()
		if !ok || t.kind != tokSep {
			return
		}
		p.advance()
	}
}

// skipSepAny skips separators and reports whether any were consumed.
func (p *aplanParser) skipSepAny() bool {
	skipped := false
	for {
		t, ok := p.peek()
		if !ok || t.kind != tokSep {
			return skipped
		}
		p.advance()
		skipped = true
	}
}

func (p *aplanParser) parseValue() (any, error) {
	p.skipSep()
	t, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("unexpected end of input")
	}

	switch t.kind {
	case tokZilde:
		p.advance()
		return Zilde, nil
	case tokLParen:
		return p.parseParenthesised()
	case tokLBracket:
		return p.parseBracketed()
	case tokNumber, tokString:
		return p.parseStrand()
	default:
		return nil, fmt.Errorf("unexpected token: %v", t)
	}
}

// parseStrand reads one or more consecutive number/string tokens as a strand.
// Single item returns a scalar; multiple items return []any.
func (p *aplanParser) parseStrand() (any, error) {
	var items []any
strandLoop:
	for {
		t, ok := p.peek()
		if !ok {
			break
		}
		switch t.kind {
		case tokNumber:
			p.advance()
			v, err := aplanParseNumber(t.text)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		case tokString:
			p.advance()
			items = append(items, t.text)
		default:
			break strandLoop
		}
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return items, nil
}

func (p *aplanParser) parseParenthesised() (any, error) {
	p.advance() // consume (
	hasLeadingSep := p.skipSepAny()

	// Empty parens = empty namespace
	if t, ok := p.peek(); ok && t.kind == tokRParen {
		p.advance()
		return &Namespace{Values: map[string]any{}}, nil
	}

	// Look ahead: NAME followed by COLON => namespace
	if p.isNamespace() {
		return p.parseNamespace()
	}

	return p.parseVector(hasLeadingSep)
}

func (p *aplanParser) isNamespace() bool {
	if p.pos+1 >= len(p.tokens) {
		return false
	}
	return p.tokens[p.pos].kind == tokName && p.tokens[p.pos+1].kind == tokColon
}

func (p *aplanParser) parseVector(hasLeadingSep bool) (any, error) {
	var elements []any
	hasSep := hasLeadingSep

	for {
		t, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated parenthesised expression")
		}
		if t.kind == tokRParen {
			p.advance()
			break
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		elements = append(elements, v)

		// Check for separator
		if t2, ok2 := p.peek(); ok2 && t2.kind == tokSep {
			hasSep = true
			p.skipSep()
		}
	}

	// No separator and single element: grouping
	if !hasSep && len(elements) == 1 {
		return elements[0], nil
	}

	// Character vector collapse: if all elements are single-char strings
	if collapsed, ok := tryCharCollapse(elements); ok {
		return collapsed, nil
	}

	return elements, nil
}

func (p *aplanParser) parseNamespace() (*Namespace, error) {
	ns := &Namespace{Values: map[string]any{}}

	for {
		p.skipSep()
		t, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated namespace")
		}
		if t.kind == tokRParen {
			p.advance()
			return ns, nil
		}

		key, err := p.expect(tokName)
		if err != nil {
			return nil, fmt.Errorf("namespace key: %w", err)
		}
		if _, err := p.expect(tokColon); err != nil {
			return nil, fmt.Errorf("namespace colon after %q: %w", key.text, err)
		}

		val, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("namespace value for %q: %w", key.text, err)
		}

		ns.Keys = append(ns.Keys, key.text)
		ns.Values[key.text] = val
	}
}

func (p *aplanParser) parseBracketed() (any, error) {
	p.advance() // consume [
	hasLeadingSep := p.skipSepAny()

	// Empty brackets are invalid APLAN
	if t, ok := p.peek(); ok && t.kind == tokRBracket {
		return nil, fmt.Errorf("empty brackets [] are invalid APLAN")
	}

	var rows []any
	hasSep := hasLeadingSep

	for {
		t, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated bracketed expression")
		}
		if t.kind == tokRBracket {
			p.advance()
			break
		}

		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		rows = append(rows, v)

		if t2, ok2 := p.peek(); ok2 && t2.kind == tokSep {
			hasSep = true
			p.skipSep()
		}
	}

	// Bracket stranding: no separator, single strand result => unpack
	if !hasSep && len(rows) == 1 {
		if arr, ok := rows[0].([]any); ok {
			rows = make([]any, len(arr))
			copy(rows, arr)
		}
	}

	return buildMatrix(rows)
}

// buildMatrix constructs an *Array from rows, computing shape and padding.
// Handles arbitrary rank: rows may be scalars, vectors, or nested Arrays.
func buildMatrix(rows []any) (*Array, error) {
	if len(rows) == 0 {
		return &Array{Shape: []int{0}}, nil
	}

	// Get the shape of each row (scalars treated as 1-element).
	rowShapes := make([][]int, len(rows))
	maxRank := 0
	for i, row := range rows {
		s := cellShape(row)
		if len(s) == 0 {
			s = []int{1} // scalar → 1-element cell
		}
		rowShapes[i] = s
		if len(s) > maxRank {
			maxRank = len(s)
		}
	}

	// Find the max dimension at each axis.
	maxShape := make([]int, maxRank)
	for dim := 0; dim < maxRank; dim++ {
		for _, s := range rowShapes {
			d := 1
			if dim < len(s) {
				d = s[dim]
			}
			if d > maxShape[dim] {
				maxShape[dim] = d
			}
		}
	}

	// Cell size is the product of maxShape.
	cellSize := 1
	for _, d := range maxShape {
		cellSize *= d
	}

	// Flatten each row, pad to cellSize, rebuild nested structure.
	data := make([]any, len(rows))
	for i, row := range rows {
		flat := flattenValue(row)
		for len(flat) < cellSize {
			flat = append(flat, 0)
		}
		if len(maxShape) <= 1 {
			data[i] = flat
		} else {
			data[i] = rebuildNested(flat, maxShape)
		}
	}

	shape := make([]int, 0, 1+len(maxShape))
	shape = append(shape, len(rows))
	shape = append(shape, maxShape...)

	return &Array{Data: data, Shape: shape}, nil
}

// cellShape returns the shape of a value as a matrix cell.
func cellShape(value any) []int {
	switch v := value.(type) {
	case *Array:
		return v.Shape
	case []any:
		if len(v) == 0 {
			return []int{0}
		}
		return []int{len(v)}
	case string:
		// In a bracketed matrix context, a string row like 'abc' has shape [3]
		return []int{len([]rune(v))}
	default:
		return nil // scalar
	}
}

// flattenValue recursively flattens a value to a 1D slice.
func flattenValue(value any) []any {
	switch v := value.(type) {
	case []any:
		var result []any
		for _, el := range v {
			result = append(result, flattenValue(el)...)
		}
		return result
	case *Array:
		var result []any
		for _, row := range v.Data {
			result = append(result, flattenValue(row)...)
		}
		return result
	case string:
		// In matrix context, flatten string to individual characters
		runes := []rune(v)
		result := make([]any, len(runes))
		for i, r := range runes {
			result[i] = string(r)
		}
		return result
	default:
		return []any{value}
	}
}

// rebuildNested reconstructs a nested slice structure from flat data and shape.
func rebuildNested(flat []any, shape []int) any {
	if len(shape) == 1 {
		result := make([]any, shape[0])
		copy(result, flat)
		return result
	}

	subShape := shape[1:]
	subSize := 1
	for _, d := range subShape {
		subSize *= d
	}

	result := make([]any, shape[0])
	for i := 0; i < shape[0]; i++ {
		start := i * subSize
		subFlat := flat[start : start+subSize]
		result[i] = rebuildNested(subFlat, subShape)
	}
	return result
}

// tryCharCollapse checks if all elements are single-character strings
// and collapses them into a single string.
func tryCharCollapse(elements []any) (string, bool) {
	if len(elements) == 0 {
		return "", false
	}
	var buf strings.Builder
	for _, e := range elements {
		s, ok := e.(string)
		if !ok {
			return "", false
		}
		if utf8.RuneCountInString(s) != 1 {
			return "", false
		}
		buf.WriteString(s)
	}
	return buf.String(), true
}

// aplanParseNumber parses an APLAN number token.
// Handles integers, floats, exponential notation, and complex (J).
func aplanParseNumber(s string) (any, error) {
	s = replaceHighMinus(s)

	// Complex: split on J/j
	if r, im, ok := splitComplex(s); ok {
		rr, err := strconv.ParseFloat(r, 64)
		if err != nil {
			return nil, fmt.Errorf("complex real part %q: %w", r, err)
		}
		ii, err := strconv.ParseFloat(im, 64)
		if err != nil {
			return nil, fmt.Errorf("complex imaginary part %q: %w", im, err)
		}
		// Normalise: zero imaginary => real scalar
		if ii == 0 {
			if rr == float64(int64(rr)) {
				return int(int64(rr)), nil
			}
			return rr, nil
		}
		return complex(rr, ii), nil
	}

	// Integer
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return int(n), nil
	}

	// Float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	return nil, fmt.Errorf("invalid number: %q", s)
}
