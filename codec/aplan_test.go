package codec

import (
	"testing"
)

// --- Scalars ---

func TestAPLANInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{"0", 0},
		{"¯5", -5},
		{"1000000", 1000000},
	}
	for _, tt := range tests {
		got, err := APLAN(tt.input)
		if err != nil {
			t.Errorf("APLAN(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("APLAN(%q) = %v (%T), want %d", tt.input, got, got, tt.want)
		}
	}
}

func TestAPLANFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"3.14", 3.14},
		{"¯2.5", -2.5},
		{"0.001", 0.001},
	}
	for _, tt := range tests {
		got, err := APLAN(tt.input)
		if err != nil {
			t.Errorf("APLAN(%q) error: %v", tt.input, err)
			continue
		}
		f, ok := got.(float64)
		if !ok || f != tt.want {
			t.Errorf("APLAN(%q) = %v (%T), want %g", tt.input, got, got, tt.want)
		}
	}
}

func TestAPLANExponential(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1E5", 1e5},
		{"2.5E¯3", 2.5e-3},
		{"1E10", 1e10},
	}
	for _, tt := range tests {
		got, err := APLAN(tt.input)
		if err != nil {
			t.Errorf("APLAN(%q) error: %v", tt.input, err)
			continue
		}
		f, ok := got.(float64)
		if !ok {
			// Could be int if exact
			if n, ok2 := got.(int); ok2 {
				f = float64(n)
			} else {
				t.Errorf("APLAN(%q) = %v (%T), want float", tt.input, got, got)
				continue
			}
		}
		if f != tt.want {
			t.Errorf("APLAN(%q) = %g, want %g", tt.input, f, tt.want)
		}
	}
}

func TestAPLANComplex(t *testing.T) {
	tests := []struct {
		input string
		want  complex128
	}{
		{"3J4", complex(3, 4)},
		{"¯2J¯3", complex(-2, -3)},
		{"0J5", complex(0, 5)},
	}
	for _, tt := range tests {
		got, err := APLAN(tt.input)
		if err != nil {
			t.Errorf("APLAN(%q) error: %v", tt.input, err)
			continue
		}
		c, ok := got.(complex128)
		if !ok || c != tt.want {
			t.Errorf("APLAN(%q) = %v (%T), want %v", tt.input, got, got, tt.want)
		}
	}
}

func TestAPLANComplexZeroImaginary(t *testing.T) {
	got, err := APLAN("1J0")
	if err != nil {
		t.Fatal(err)
	}
	// Should normalise to scalar 1, not complex
	if got != 1 {
		t.Errorf("APLAN(\"1J0\") = %v (%T), want int 1", got, got)
	}
}

// --- Strings ---

func TestAPLANString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'hello'", "hello"},
		{"''", ""},
		{"'it''s'", "it's"},
		{"'hello world'", "hello world"},
		{"'say ''hi'''", "say 'hi'"},
	}
	for _, tt := range tests {
		got, err := APLAN(tt.input)
		if err != nil {
			t.Errorf("APLAN(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("APLAN(%q) = %v, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Zilde ---

func TestAPLANZilde(t *testing.T) {
	got, err := APLAN("⍬")
	if err != nil {
		t.Fatal(err)
	}
	if got != Zilde {
		t.Errorf("APLAN(\"⍬\") = %v (%T), want Zilde", got, got)
	}
}

// --- Strands ---

func TestAPLANStrand(t *testing.T) {
	got, err := APLAN("1 2 3")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("APLAN(\"1 2 3\") = %v (%T), want 3-element slice", got, got)
	}
	if arr[0] != 1 || arr[1] != 2 || arr[2] != 3 {
		t.Errorf("APLAN(\"1 2 3\") = %v, want [1 2 3]", arr)
	}
}

func TestAPLANStrandNegative(t *testing.T) {
	got, err := APLAN("¯1 0 1")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("got %v (%T)", got, got)
	}
	if arr[0] != -1 || arr[1] != 0 || arr[2] != 1 {
		t.Errorf("got %v, want [-1 0 1]", arr)
	}
}

func TestAPLANStrandMixed(t *testing.T) {
	got, err := APLAN("1 2.5 3")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("got %v (%T)", got, got)
	}
	if arr[0] != 1 || arr[1] != 2.5 || arr[2] != 3 {
		t.Errorf("got %v", arr)
	}
}

// --- Vectors (parenthesised) ---

func TestAPLANVectorSeparator(t *testing.T) {
	got, err := APLAN("(1 ⋄ 2 ⋄ 3)")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("got %v (%T)", got, got)
	}
	if arr[0] != 1 || arr[1] != 2 || arr[2] != 3 {
		t.Errorf("got %v", arr)
	}
}

func TestAPLANVectorNewline(t *testing.T) {
	got, err := APLAN("(1\n2\n3)")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("got %v (%T)", got, got)
	}
}

func TestAPLANVectorNested(t *testing.T) {
	got, err := APLAN("((1 ⋄ 2) ⋄ (3 ⋄ 4))")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T)", got, got)
	}
	inner0, ok := arr[0].([]any)
	if !ok || len(inner0) != 2 || inner0[0] != 1 || inner0[1] != 2 {
		t.Errorf("inner[0] = %v", arr[0])
	}
}

func TestAPLANGrouping(t *testing.T) {
	got, err := APLAN("(42)")
	if err != nil {
		t.Fatal(err)
	}
	// Single element, no separator = grouping, returns scalar
	if got != 42 {
		t.Errorf("APLAN(\"(42)\") = %v (%T), want 42", got, got)
	}
}

func TestAPLANVectorMixed(t *testing.T) {
	got, err := APLAN("(1 ⋄ 'two' ⋄ 3)")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("got %v (%T)", got, got)
	}
	if arr[0] != 1 || arr[1] != "two" || arr[2] != 3 {
		t.Errorf("got %v", arr)
	}
}

// --- Character vector collapse ---

func TestAPLANCharVectorCollapse(t *testing.T) {
	got, err := APLAN("('a' ⋄ 'b' ⋄ 'c')")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc" {
		t.Errorf("APLAN char vector collapse = %v (%T), want \"abc\"", got, got)
	}
}

func TestAPLANCharVectorNoCollapse(t *testing.T) {
	got, err := APLAN("('ab' ⋄ 'cd')")
	if err != nil {
		t.Fatal(err)
	}
	// Multi-char strings should NOT collapse
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T)", got, got)
	}
	if arr[0] != "ab" || arr[1] != "cd" {
		t.Errorf("got %v", arr)
	}
}

func TestAPLANCharVectorMixedLength(t *testing.T) {
	got, err := APLAN("('a' ⋄ 'ab')")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T)", got, got)
	}
}

// --- Matrices ---

func TestAPLANMatrix(t *testing.T) {
	got, err := APLAN("[1 2 ⋄ 3 4]")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*Array)
	if !ok {
		t.Fatalf("got %v (%T), want *Array", got, got)
	}
	if len(arr.Shape) != 2 || arr.Shape[0] != 2 || arr.Shape[1] != 2 {
		t.Errorf("shape = %v, want [2 2]", arr.Shape)
	}
	if len(arr.Data) != 2 {
		t.Errorf("data rows = %d, want 2", len(arr.Data))
	}
}

func TestAPLANMatrixColumn(t *testing.T) {
	got, err := APLAN("[1 ⋄ 2 ⋄ 3]")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*Array)
	if !ok {
		t.Fatalf("got %v (%T), want *Array", got, got)
	}
	if arr.Shape[0] != 3 || arr.Shape[1] != 1 {
		t.Errorf("shape = %v, want [3 1]", arr.Shape)
	}
}

func TestAPLANMatrixPadding(t *testing.T) {
	got, err := APLAN("[1 2 ⋄ 3 4 5]")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*Array)
	if !ok {
		t.Fatalf("got %v (%T)", got, got)
	}
	if arr.Shape[0] != 2 || arr.Shape[1] != 3 {
		t.Errorf("shape = %v, want [2 3]", arr.Shape)
	}
	// First row should be padded
	row0, ok := arr.Data[0].([]any)
	if !ok {
		t.Fatalf("row 0 type: %T", arr.Data[0])
	}
	if len(row0) != 3 || row0[2] != 0 {
		t.Errorf("row 0 = %v, want [1 2 0]", row0)
	}
}

func TestAPLANBracketStranding(t *testing.T) {
	got, err := APLAN("[1 2]")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*Array)
	if !ok {
		t.Fatalf("got %v (%T), want *Array", got, got)
	}
	// Strand is unpacked: each element becomes a row
	if arr.Shape[0] != 2 || arr.Shape[1] != 1 {
		t.Errorf("shape = %v, want [2 1]", arr.Shape)
	}
}

func TestAPLANEmptyBrackets(t *testing.T) {
	_, err := APLAN("[]")
	if err == nil {
		t.Error("APLAN(\"[]\") should return error")
	}
}

// --- Namespaces ---

func TestAPLANEmptyNamespace(t *testing.T) {
	got, err := APLAN("()")
	if err != nil {
		t.Fatal(err)
	}
	ns, ok := got.(*Namespace)
	if !ok {
		t.Fatalf("got %v (%T), want *Namespace", got, got)
	}
	if len(ns.Keys) != 0 {
		t.Errorf("empty namespace has %d keys", len(ns.Keys))
	}
}

func TestAPLANNamespaceSingle(t *testing.T) {
	got, err := APLAN("(x: 42)")
	if err != nil {
		t.Fatal(err)
	}
	ns, ok := got.(*Namespace)
	if !ok {
		t.Fatalf("got %v (%T), want *Namespace", got, got)
	}
	if ns.Values["x"] != 42 {
		t.Errorf("ns[\"x\"] = %v, want 42", ns.Values["x"])
	}
}

func TestAPLANNamespaceMultiple(t *testing.T) {
	got, err := APLAN("(x: 1 ⋄ y: 2)")
	if err != nil {
		t.Fatal(err)
	}
	ns, ok := got.(*Namespace)
	if !ok {
		t.Fatalf("got %v (%T), want *Namespace", got, got)
	}
	if ns.Values["x"] != 1 || ns.Values["y"] != 2 {
		t.Errorf("ns = %v", ns.Values)
	}
	if len(ns.Keys) != 2 || ns.Keys[0] != "x" || ns.Keys[1] != "y" {
		t.Errorf("key order = %v, want [x y]", ns.Keys)
	}
}

func TestAPLANNamespaceNested(t *testing.T) {
	got, err := APLAN("(outer: (inner: 42))")
	if err != nil {
		t.Fatal(err)
	}
	ns, ok := got.(*Namespace)
	if !ok {
		t.Fatalf("got %v (%T)", got, got)
	}
	inner, ok := ns.Values["outer"].(*Namespace)
	if !ok {
		t.Fatalf("outer value %v (%T)", ns.Values["outer"], ns.Values["outer"])
	}
	if inner.Values["inner"] != 42 {
		t.Errorf("inner = %v", inner.Values)
	}
}

func TestAPLANNamespaceWithVector(t *testing.T) {
	got, err := APLAN("(data: (1 ⋄ 2 ⋄ 3))")
	if err != nil {
		t.Fatal(err)
	}
	ns, ok := got.(*Namespace)
	if !ok {
		t.Fatalf("got %v (%T)", got, got)
	}
	arr, ok := ns.Values["data"].([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("data = %v (%T)", ns.Values["data"], ns.Values["data"])
	}
}

func TestAPLANNamespaceWithMatrix(t *testing.T) {
	got, err := APLAN("(name: 'data' ⋄ matrix: [1 2 ⋄ 3 4])")
	if err != nil {
		t.Fatal(err)
	}
	ns, ok := got.(*Namespace)
	if !ok {
		t.Fatalf("got %v (%T)", got, got)
	}
	if ns.Values["name"] != "data" {
		t.Errorf("name = %v", ns.Values["name"])
	}
	mat, ok := ns.Values["matrix"].(*Array)
	if !ok {
		t.Fatalf("matrix = %v (%T)", ns.Values["matrix"], ns.Values["matrix"])
	}
	if mat.Shape[0] != 2 || mat.Shape[1] != 2 {
		t.Errorf("matrix shape = %v", mat.Shape)
	}
}

// --- Complex nesting ---

func TestAPLANVectorOfMatrices(t *testing.T) {
	got, err := APLAN("([1 2 ⋄ 3 4] ⋄ [5 6 ⋄ 7 8])")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T)", got, got)
	}
	m0, ok := arr[0].(*Array)
	if !ok {
		t.Fatalf("element 0 = %v (%T)", arr[0], arr[0])
	}
	if m0.Shape[0] != 2 || m0.Shape[1] != 2 {
		t.Errorf("matrix 0 shape = %v", m0.Shape)
	}
}

func TestAPLANVectorOfNamespaces(t *testing.T) {
	got, err := APLAN("((x: 1) ⋄ (y: 2))")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T)", got, got)
	}
	ns0, ok := arr[0].(*Namespace)
	if !ok {
		t.Fatalf("element 0 = %v (%T)", arr[0], arr[0])
	}
	if ns0.Values["x"] != 1 {
		t.Errorf("ns0 = %v", ns0.Values)
	}
}

// --- Whitespace and separator edge cases ---

func TestAPLANLeadingTrailingWhitespace(t *testing.T) {
	got, err := APLAN("  42  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Errorf("got %v, want 42", got)
	}
}

func TestAPLANLeadingTrailingSeparators(t *testing.T) {
	got, err := APLAN("(⋄ 1 ⋄ 2 ⋄)")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T), want [1 2]", got, got)
	}
}

// --- Leading separator semantics ---

func TestAPLANLeadingSepMakesVector(t *testing.T) {
	// (⋄ 42) is a 1-element vector, not a scalar grouping.
	got, err := APLAN("(⋄ 42)")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 1 || arr[0] != 42 {
		t.Errorf("APLAN(\"(⋄ 42)\") = %v (%T), want [42]", got, got)
	}
}

func TestAPLANTrailingSepMakesVector(t *testing.T) {
	// (42 ⋄) is a 1-element vector.
	got, err := APLAN("(42 ⋄)")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 1 || arr[0] != 42 {
		t.Errorf("APLAN(\"(42 ⋄)\") = %v (%T), want [42]", got, got)
	}
}

func TestAPLANLeadingSepBracket(t *testing.T) {
	// [⋄ 1 2] keeps the strand as one row (1×2), not unpacked to (2×1).
	got, err := APLAN("[⋄ 1 2]")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*Array)
	if !ok {
		t.Fatalf("got %v (%T), want *Array", got, got)
	}
	if arr.Shape[0] != 1 || arr.Shape[1] != 2 {
		t.Errorf("shape = %v, want [1 2]", arr.Shape)
	}
}

// --- Higher-rank matrices ---

func TestAPLANHigherRankMatrix(t *testing.T) {
	// 3D array: 2 major cells, each a 2×2 matrix.
	got, err := APLAN("[[1 2 ⋄ 3 4] ⋄ [5 6 ⋄ 7 8]]")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*Array)
	if !ok {
		t.Fatalf("got %v (%T), want *Array", got, got)
	}
	if len(arr.Shape) != 3 || arr.Shape[0] != 2 || arr.Shape[1] != 2 || arr.Shape[2] != 2 {
		t.Errorf("shape = %v, want [2 2 2]", arr.Shape)
	}
	// Verify data: first major cell should be [[1,2],[3,4]]
	cell0, ok := arr.Data[0].(any)
	if !ok {
		t.Fatalf("cell 0 type: %T", arr.Data[0])
	}
	rows, ok := cell0.([]any)
	if !ok || len(rows) != 2 {
		t.Fatalf("cell 0 = %v (%T), want 2-row nested slice", cell0, cell0)
	}
	row0, ok := rows[0].([]any)
	if !ok || len(row0) != 2 || row0[0] != 1 || row0[1] != 2 {
		t.Errorf("cell 0 row 0 = %v, want [1 2]", rows[0])
	}
}
