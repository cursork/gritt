package codec

import (
	"math"
	"testing"
)

func TestInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{"0", 0},
		{"¯42", -42},
		{"¯1", -1},
		{"  42  ", 42},
		{"1000000", 1000000},
	}
	for _, tt := range tests {
		got, err := Int(tt.input)
		if err != nil {
			t.Errorf("Int(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Int(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestIntErrors(t *testing.T) {
	for _, input := range []string{"3.14", "abc", "1J2", ""} {
		_, err := Int(input)
		if err == nil {
			t.Errorf("Int(%q) should have returned error", input)
		}
	}
}

func TestFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"3.14", 3.14},
		{"42", 42.0},
		{"¯3.14", -3.14},
		{"1.5E10", 1.5e10},
		{"1.5E¯3", 1.5e-3},
		{"0.001", 0.001},
	}
	for _, tt := range tests {
		got, err := Float(tt.input)
		if err != nil {
			t.Errorf("Float(%q) error: %v", tt.input, err)
			continue
		}
		if math.Abs(got-tt.want) > 1e-15 {
			t.Errorf("Float(%q) = %g, want %g", tt.input, got, tt.want)
		}
	}
}

func TestComplex(t *testing.T) {
	tests := []struct {
		input string
		want  complex128
	}{
		{"1J2", complex(1, 2)},
		{"1j2", complex(1, 2)},
		{"3.14J¯2.5", complex(3.14, -2.5)},
		{"¯1J¯1", complex(-1, -1)},
		{"0J1", complex(0, 1)},
	}
	for _, tt := range tests {
		got, err := Complex(tt.input)
		if err != nil {
			t.Errorf("Complex(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Complex(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestComplexErrors(t *testing.T) {
	for _, input := range []string{"42", "abc", ""} {
		_, err := Complex(input)
		if err == nil {
			t.Errorf("Complex(%q) should have returned error", input)
		}
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'hello'", "hello"},
		{"'it''s'", "it's"},
		{"''", ""},
		{"'hello world'", "hello world"},
		{"unquoted", "unquoted"},
	}
	for _, tt := range tests {
		got, err := String(tt.input)
		if err != nil {
			t.Errorf("String(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInts(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1 2 3", []int{1, 2, 3}},
		{"¯1 2 ¯3", []int{-1, 2, -3}},
		{"42", []int{42}},
		{"0 1 0 1", []int{0, 1, 0, 1}},
	}
	for _, tt := range tests {
		got, err := Ints(tt.input)
		if err != nil {
			t.Errorf("Ints(%q) error: %v", tt.input, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("Ints(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("Ints(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIntsEmpty(t *testing.T) {
	got, err := Ints("")
	if err != nil {
		t.Errorf("Ints(\"\") error: %v", err)
	}
	if got != nil {
		t.Errorf("Ints(\"\") = %v, want nil", got)
	}
}

func TestFloats(t *testing.T) {
	got, err := Floats("1.5 2.5 ¯3.5")
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{1.5, 2.5, -3.5}
	if len(got) != len(want) {
		t.Fatalf("Floats len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-15 {
			t.Errorf("Floats[%d] = %g, want %g", i, got[i], want[i])
		}
	}
}

func TestIntMatrix(t *testing.T) {
	got, err := IntMatrix("1 2 3\n4 5 6")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{1, 2, 3}, {4, 5, 6}}
	if len(got) != len(want) {
		t.Fatalf("IntMatrix rows = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("IntMatrix row %d len = %d, want %d", i, len(got[i]), len(want[i]))
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("IntMatrix[%d][%d] = %d, want %d", i, j, got[i][j], want[i][j])
			}
		}
	}
}

func TestFloatMatrix(t *testing.T) {
	got, err := FloatMatrix("1.5 2.5\n¯3.5 4.5")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[0]) != 2 {
		t.Fatalf("FloatMatrix shape wrong: %v", got)
	}
	if got[1][0] != -3.5 {
		t.Errorf("FloatMatrix[1][0] = %g, want -3.5", got[1][0])
	}
}

func TestIntMatrixTrailingNewline(t *testing.T) {
	got, err := IntMatrix("1 2\n3 4\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("IntMatrix with trailing newline: rows = %d, want 2", len(got))
	}
}

func TestAutoNil(t *testing.T) {
	for _, input := range []string{"", "  ", "\n", "\n\n"} {
		got := Auto(input)
		if got != nil {
			t.Errorf("Auto(%q) = %v, want nil", input, got)
		}
	}
}

func TestAutoEmpty(t *testing.T) {
	got := Auto("⍬")
	arr, ok := got.([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("Auto(\"⍬\") = %v (%T), want empty []any", got, got)
	}
}

func TestAutoScalarInt(t *testing.T) {
	got := Auto("42")
	if got != 42 {
		t.Errorf("Auto(\"42\") = %v (%T), want 42", got, got)
	}
}

func TestAutoScalarNegative(t *testing.T) {
	got := Auto("¯7")
	if got != -7 {
		t.Errorf("Auto(\"¯7\") = %v (%T), want -7", got, got)
	}
}

func TestAutoScalarFloat(t *testing.T) {
	got := Auto("3.14")
	f, ok := got.(float64)
	if !ok || math.Abs(f-3.14) > 1e-15 {
		t.Errorf("Auto(\"3.14\") = %v (%T), want 3.14", got, got)
	}
}

func TestAutoScalarComplex(t *testing.T) {
	got := Auto("1J2")
	c, ok := got.(complex128)
	if !ok || c != complex(1, 2) {
		t.Errorf("Auto(\"1J2\") = %v (%T), want (1+2i)", got, got)
	}
}

func TestAutoQuotedString(t *testing.T) {
	got := Auto("'hello'")
	if got != "hello" {
		t.Errorf("Auto(\"'hello'\") = %v, want \"hello\"", got)
	}
}

func TestAutoVector(t *testing.T) {
	got := Auto("1 2 3")
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Errorf("Auto(\"1 2 3\") = %v (%T), want []any{1,2,3}", got, got)
		return
	}
	if arr[0] != 1 || arr[1] != 2 || arr[2] != 3 {
		t.Errorf("Auto(\"1 2 3\") = %v, want [1 2 3]", arr)
	}
}

func TestAutoVectorMixedSign(t *testing.T) {
	got := Auto("¯1 2 ¯3")
	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("Auto(\"¯1 2 ¯3\") = %v (%T)", got, got)
	}
	if arr[0] != -1 || arr[1] != 2 || arr[2] != -3 {
		t.Errorf("Auto(\"¯1 2 ¯3\") = %v", arr)
	}
}

func TestAutoMatrix(t *testing.T) {
	got := Auto("1 2 3\n4 5 6")
	mat, ok := got.([][]any)
	if !ok || len(mat) != 2 {
		t.Fatalf("Auto(\"1 2 3\\n4 5 6\") = %v (%T)", got, got)
	}
	if len(mat[0]) != 3 || mat[0][0] != 1 || mat[1][2] != 6 {
		t.Errorf("Auto matrix = %v", mat)
	}
}

func TestAutoMatrixTrailingNewline(t *testing.T) {
	got := Auto("1 2\n3 4\n")
	mat, ok := got.([][]any)
	if !ok || len(mat) != 2 {
		t.Errorf("Auto with trailing newline: %v (%T), want 2-row matrix", got, got)
	}
}

func TestAutoScalarWithTrailingNewline(t *testing.T) {
	got := Auto("42\n")
	if got != 42 {
		t.Errorf("Auto(\"42\\n\") = %v (%T), want 42", got, got)
	}
}

func TestAutoScientificNotation(t *testing.T) {
	got := Auto("1.5E10")
	f, ok := got.(float64)
	if !ok || f != 1.5e10 {
		t.Errorf("Auto(\"1.5E10\") = %v (%T), want 1.5e10", got, got)
	}
}

func TestAutoUnparseable(t *testing.T) {
	got := Auto("hello")
	if got != "hello" {
		t.Errorf("Auto(\"hello\") = %v, want \"hello\"", got)
	}
}

func TestScalar(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{"42", 42},
		{"¯42", -42},
		{"3.14", 3.14},
		{"1J2", complex(1, 2)},
		{"'hello'", "hello"},
		{"⍬", []any{}},
		{"", nil},
		{"abc", "abc"},
	}
	for _, tt := range tests {
		got := Scalar(tt.input)
		switch w := tt.want.(type) {
		case nil:
			if got != nil {
				t.Errorf("Scalar(%q) = %v, want nil", tt.input, got)
			}
		case []any:
			arr, ok := got.([]any)
			if !ok || len(arr) != len(w) {
				t.Errorf("Scalar(%q) = %v (%T), want %v", tt.input, got, got, tt.want)
			}
		default:
			if got != tt.want {
				t.Errorf("Scalar(%q) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
			}
		}
	}
}

func TestRaggedMatrix(t *testing.T) {
	got := Auto("1 2 3\n4 5")
	mat, ok := got.([][]any)
	if !ok || len(mat) != 2 {
		t.Fatalf("ragged matrix: %v (%T)", got, got)
	}
	if len(mat[0]) != 3 || len(mat[1]) != 2 {
		t.Errorf("ragged matrix shapes: %d, %d", len(mat[0]), len(mat[1]))
	}
}

func TestSingleRowNotMatrix(t *testing.T) {
	// Single line should be a vector, not a 1-row matrix
	got := Auto("1 2 3")
	_, isMatrix := got.([][]any)
	if isMatrix {
		t.Errorf("Auto(\"1 2 3\") returned matrix, want flat vector")
	}
}
