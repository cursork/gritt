package codec

import "testing"

// --- Serialize scalars ---

func TestSerializeInt(t *testing.T) {
	if got := Serialize(42); got != "42" {
		t.Errorf("Serialize(42) = %q, want \"42\"", got)
	}
}

func TestSerializeNegativeInt(t *testing.T) {
	if got := Serialize(-5); got != "¯5" {
		t.Errorf("Serialize(-5) = %q, want \"¯5\"", got)
	}
}

func TestSerializeFloat(t *testing.T) {
	if got := Serialize(3.14); got != "3.14" {
		t.Errorf("Serialize(3.14) = %q, want \"3.14\"", got)
	}
}

func TestSerializeString(t *testing.T) {
	if got := Serialize("hello"); got != "'hello'" {
		t.Errorf("Serialize(\"hello\") = %q, want \"'hello'\"", got)
	}
}

func TestSerializeStringWithQuote(t *testing.T) {
	if got := Serialize("it's"); got != "'it''s'" {
		t.Errorf("Serialize(\"it's\") = %q, want \"'it''s'\"", got)
	}
}

func TestSerializeComplex(t *testing.T) {
	if got := Serialize(complex(3, 4)); got != "3J4" {
		t.Errorf("Serialize(3+4i) = %q, want \"3J4\"", got)
	}
}

func TestSerializeComplexNegative(t *testing.T) {
	if got := Serialize(complex(-2, -3)); got != "¯2J¯3" {
		t.Errorf("Serialize(-2-3i) = %q, want \"¯2J¯3\"", got)
	}
}

func TestSerializeComplexZeroIm(t *testing.T) {
	// Zero imaginary → plain number
	if got := Serialize(complex(5, 0)); got != "5" {
		t.Errorf("Serialize(5+0i) = %q, want \"5\"", got)
	}
}

// --- Serialize zilde ---

func TestSerializeZilde(t *testing.T) {
	if got := Serialize(Zilde); got != "⍬" {
		t.Errorf("Serialize(Zilde) = %q, want \"⍬\"", got)
	}
}

func TestSerializeEmptySlice(t *testing.T) {
	if got := Serialize([]any{}); got != "⍬" {
		t.Errorf("Serialize([]any{}) = %q, want \"⍬\"", got)
	}
}

func TestSerializeNil(t *testing.T) {
	if got := Serialize(nil); got != "⍬" {
		t.Errorf("Serialize(nil) = %q, want \"⍬\"", got)
	}
}

// --- Serialize vectors ---

func TestSerializeNumberStrand(t *testing.T) {
	got := Serialize([]any{1, 2, 3})
	if got != "1 2 3" {
		t.Errorf("Serialize([1,2,3]) = %q, want \"1 2 3\"", got)
	}
}

func TestSerializeMixedVectorDiamond(t *testing.T) {
	got := Serialize([]any{1, "two", 3}, SerializeOptions{UseDiamond: true})
	if got != "(1 ⋄ 'two' ⋄ 3)" {
		t.Errorf("got %q", got)
	}
}

// --- Serialize matrices ---

func TestSerializeMatrixDiamond(t *testing.T) {
	m := &Array{
		Data:  []any{[]any{1, 2}, []any{3, 4}},
		Shape: []int{2, 2},
	}
	got := Serialize(m, SerializeOptions{UseDiamond: true})
	if got != "[1 2 ⋄ 3 4]" {
		t.Errorf("got %q", got)
	}
}

func TestSerializeEmptyMatrix(t *testing.T) {
	m := &Array{Shape: []int{0}}
	if got := Serialize(m); got != "[]" {
		t.Errorf("got %q, want \"[]\"", got)
	}
}

// --- Serialize namespaces ---

func TestSerializeNamespaceDiamond(t *testing.T) {
	ns := &Namespace{
		Keys:   []string{"x", "y"},
		Values: map[string]any{"x": 1, "y": 2},
	}
	got := Serialize(ns, SerializeOptions{UseDiamond: true})
	if got != "(x: 1 ⋄ y: 2)" {
		t.Errorf("got %q", got)
	}
}

func TestSerializeEmptyNamespace(t *testing.T) {
	ns := &Namespace{Values: map[string]any{}}
	if got := Serialize(ns); got != "()" {
		t.Errorf("got %q, want \"()\"", got)
	}
}

// --- Round-trips ---

func roundTrip(t *testing.T, source string) {
	t.Helper()
	parsed, err := APLAN(source)
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	serialized := Serialize(parsed, SerializeOptions{UseDiamond: true})
	reparsed, err := APLAN(serialized)
	if err != nil {
		t.Fatalf("reparse %q (from %q): %v", serialized, source, err)
	}
	if !Equal(parsed, reparsed) {
		t.Errorf("round-trip failed for %q\n  serialized: %q\n  parsed:   %v\n  reparsed: %v",
			source, serialized, parsed, reparsed)
	}
}

func TestRoundTripNumber(t *testing.T)        { roundTrip(t, "42") }
func TestRoundTripNegative(t *testing.T)      { roundTrip(t, "¯123") }
func TestRoundTripFloat(t *testing.T)         { roundTrip(t, "3.14") }
func TestRoundTripString(t *testing.T)        { roundTrip(t, "'hello world'") }
func TestRoundTripStringQuote(t *testing.T)   { roundTrip(t, "'it''s'") }
func TestRoundTripVector(t *testing.T)        { roundTrip(t, "(1 ⋄ 2 ⋄ 3)") }
func TestRoundTripMixedVector(t *testing.T)   { roundTrip(t, "(1 ⋄ 'two' ⋄ 3)") }
func TestRoundTripNestedVector(t *testing.T)  { roundTrip(t, "((1 ⋄ 2) ⋄ (3 ⋄ 4))") }
func TestRoundTripSimpleMatrix(t *testing.T)  { roundTrip(t, "[1 2 ⋄ 3 4]") }
func TestRoundTripColumnMatrix(t *testing.T)  { roundTrip(t, "[1 ⋄ 2 ⋄ 3]") }
func TestRoundTripStringMatrix(t *testing.T)  { roundTrip(t, "['a' 'b' ⋄ 'c' 'd']") }
func TestRoundTripZilde(t *testing.T)         { roundTrip(t, "⍬") }
func TestRoundTripNamespace(t *testing.T)     { roundTrip(t, "(x: 1 ⋄ y: 2)") }
func TestRoundTripNestedNS(t *testing.T)      { roundTrip(t, "(outer: (inner: 42))") }
func TestRoundTripNSWithVector(t *testing.T)  { roundTrip(t, "(data: (1 ⋄ 2 ⋄ 3))") }
func TestRoundTripNSWithMatrix(t *testing.T)  { roundTrip(t, "(name: 'data' ⋄ matrix: [1 2 ⋄ 3 4])") }
func TestRoundTripComplex(t *testing.T)       { roundTrip(t, "3J4") }
func TestRoundTripDeeplyNested(t *testing.T)  { roundTrip(t, "(((1 ⋄ 2) ⋄ (3 ⋄ 4)) ⋄ ((5 ⋄ 6) ⋄ (7 ⋄ 8)))") }
func TestRoundTripVectorMatrices(t *testing.T) { roundTrip(t, "([1 2 ⋄ 3 4] ⋄ [5 6 ⋄ 7 8])") }
