package amicable

import (
	"math"
	"testing"

	"github.com/cursork/gritt/codec"
)

// signedToUnsigned converts APL's signed byte representation to Go bytes.
func signedToUnsigned(vals ...int) []byte {
	out := make([]byte, len(vals))
	for i, v := range vals {
		out[i] = byte(int8(v))
	}
	return out
}

// All test vectors captured from Dyalog v20 (64-bit, macOS ARM64).

func TestUnmarshalScalarInt8(t *testing.T) {
	// 42, DR=83
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 34, 0, 0, 0, 0, 0, 0,
		42, 0, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("got %v (%T), want 42", got, got)
	}
}

func TestUnmarshalScalarIntZero(t *testing.T) {
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 34, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestUnmarshalScalarIntNeg(t *testing.T) {
	// -5, DR=83
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 34, 0, 0, 0, 0, 0, 0,
		-5, 0, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != -5 {
		t.Fatalf("got %v, want -5", got)
	}
}

func TestUnmarshalScalarInt16(t *testing.T) {
	// 1000, DR=163
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 35, 0, 0, 0, 0, 0, 0,
		-24, 3, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1000 {
		t.Fatalf("got %v, want 1000", got)
	}
}

func TestUnmarshalScalarInt32(t *testing.T) {
	// 1000000, DR=323
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 36, 0, 0, 0, 0, 0, 0,
		64, 66, 15, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1000000 {
		t.Fatalf("got %v, want 1000000", got)
	}
}

func TestUnmarshalScalarBigInt(t *testing.T) {
	// 2147483647, DR=323
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 36, 0, 0, 0, 0, 0, 0,
		-1, -1, -1, 127, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2147483647 {
		t.Fatalf("got %v, want 2147483647", got)
	}
}

func TestUnmarshalScalarFloat(t *testing.T) {
	// 3.14, DR=645
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 37, 0, 0, 0, 0, 0, 0,
		31, -123, -21, 81, -72, 30, 9, 64,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3.14 {
		t.Fatalf("got %v, want 3.14", got)
	}
}

func TestUnmarshalScalarFloat64Large(t *testing.T) {
	// 2*40 = 1099511627776, stored as float64 by Dyalog
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 37, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 112, 66,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	want := 1099511627776.0
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnmarshalScalarChar(t *testing.T) {
	// 'X', DR=80
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 39, 0, 0, 0, 0, 0, 0,
		88, 0, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "X" {
		t.Fatalf("got %v, want \"X\"", got)
	}
}

func TestUnmarshalScalarComplex(t *testing.T) {
	// 1J2, DR=1289
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		15, 42, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, -16, 63, // 1.0
		0, 0, 0, 0, 0, 0, 0, 64, // 2.0
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	want := complex(1.0, 2.0)
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnmarshalScalarDecimal128(t *testing.T) {
	// 1÷3 with ⎕FR←1287
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		15, 46, 0, 0, 0, 0, 0, 0,
		85, 85, 85, 85, 33, -38, -39, 103,
		-107, -126, -28, -108, 88, -92, -4, 47,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := got.(Decimal128)
	if !ok {
		t.Fatalf("got %T, want Decimal128", got)
	}
	// Verify the raw bytes survived
	if d[0] != 85 || d[15] != 47 {
		t.Fatalf("decimal128 bytes corrupted")
	}
}

func TestUnmarshalVecInt(t *testing.T) {
	// 1 2 3, DR=83
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		31, 34, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	assertIntSlice(t, vec, []int{1, 2, 3})
}

func TestUnmarshalVecChar(t *testing.T) {
	// 'hello', DR=80
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		31, 39, 0, 0, 0, 0, 0, 0,
		5, 0, 0, 0, 0, 0, 0, 0,
		104, 101, 108, 108, 111, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("got %v, want \"hello\"", got)
	}
}

func TestUnmarshalVecBool(t *testing.T) {
	// 1 0 1 1 0, DR=11
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		31, 33, 0, 0, 0, 0, 0, 0,
		5, 0, 0, 0, 0, 0, 0, 0,
		-80, 0, 0, 0, 0, 0, 0, 0, // 0xB0 = 10110000
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	assertIntSlice(t, vec, []int{1, 0, 1, 1, 0})
}

func TestUnmarshalVecFloat(t *testing.T) {
	// 1.1 2.2 3.3, DR=645
	data := signedToUnsigned(
		-33, -92, 7, 0, 0, 0, 0, 0, 0, 0,
		31, 37, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		-102, -103, -103, -103, -103, -103, -15, 63, // 1.1
		-102, -103, -103, -103, -103, -103, 1, 64, // 2.2
		102, 102, 102, 102, 102, 102, 10, 64, // 3.3
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(vec) != 3 {
		t.Fatalf("len=%d, want 3", len(vec))
	}
	assertFloat(t, vec[0], 1.1)
	assertFloat(t, vec[1], 2.2)
	assertFloat(t, vec[2], 3.3)
}

func TestUnmarshalVecInt16(t *testing.T) {
	// 200 300 400, DR=163
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		31, 35, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		-56, 0, 44, 1, -112, 1, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	assertIntSlice(t, vec, []int{200, 300, 400})
}

func TestUnmarshalVecInt32(t *testing.T) {
	// 100000 200000 300000, DR=323
	data := signedToUnsigned(
		-33, -92, 6, 0, 0, 0, 0, 0, 0, 0,
		31, 36, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		-96, -122, 1, 0, 64, 13, 3, 0,
		-32, -109, 4, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	assertIntSlice(t, vec, []int{100000, 200000, 300000})
}

func TestUnmarshalVecComplex(t *testing.T) {
	// 1J2 3J4, DR=1289
	data := signedToUnsigned(
		-33, -92, 8, 0, 0, 0, 0, 0, 0, 0,
		31, 42, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, -16, 63, // 1.0
		0, 0, 0, 0, 0, 0, 0, 64, // 2.0
		0, 0, 0, 0, 0, 0, 8, 64, // 3.0
		0, 0, 0, 0, 0, 0, 16, 64, // 4.0
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(vec) != 2 {
		t.Fatalf("len=%d, want 2", len(vec))
	}
	if vec[0] != complex(1.0, 2.0) {
		t.Fatalf("vec[0]=%v, want 1+2i", vec[0])
	}
	if vec[1] != complex(3.0, 4.0) {
		t.Fatalf("vec[1]=%v, want 3+4i", vec[1])
	}
}

func TestUnmarshalVecEmptyChar(t *testing.T) {
	// '', DR=80
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		31, 39, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %v, want empty string", got)
	}
}

func TestUnmarshalVecEmptyNum(t *testing.T) {
	// ⍬, DR=83
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		31, 34, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(vec) != 0 {
		t.Fatalf("len=%d, want 0", len(vec))
	}
}

func TestUnmarshalVecUnicode(t *testing.T) {
	// '⍳⍴⍬', DR=160
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		31, 40, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		115, 35, 116, 35, 108, 35, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "⍳⍴⍬" {
		t.Fatalf("got %q, want \"⍳⍴⍬\"", got)
	}
}

func TestUnmarshalVecChar32(t *testing.T) {
	// ⎕UCS 100000 100001 100002, DR=320
	data := signedToUnsigned(
		-33, -92, 6, 0, 0, 0, 0, 0, 0, 0,
		31, 41, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		-96, -122, 1, 0, -95, -122, 1, 0,
		-94, -122, 1, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("got %T, want string", got)
	}
	runes := []rune(s)
	if len(runes) != 3 || runes[0] != 100000 || runes[1] != 100001 || runes[2] != 100002 {
		t.Fatalf("got runes %v, want [100000 100001 100002]", runes)
	}
}

func TestUnmarshalMatInt(t *testing.T) {
	// 2 3⍴⍳6, DR=83
	data := signedToUnsigned(
		-33, -92, 6, 0, 0, 0, 0, 0, 0, 0,
		47, 34, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 4, 5, 6, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*codec.Array)
	if !ok {
		t.Fatalf("got %T, want *codec.Array", got)
	}
	assertShape(t, arr, []int{2, 3})
	assertIntSlice(t, arr.Data, []int{1, 2, 3, 4, 5, 6})
}

func TestUnmarshalMatChar(t *testing.T) {
	// 2 3⍴'abcdef', DR=80
	data := signedToUnsigned(
		-33, -92, 6, 0, 0, 0, 0, 0, 0, 0,
		47, 39, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		97, 98, 99, 100, 101, 102, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*codec.Array)
	if !ok {
		t.Fatalf("got %T, want *codec.Array", got)
	}
	assertShape(t, arr, []int{2, 3})
	// Char matrix elements are single-char strings
	want := []string{"a", "b", "c", "d", "e", "f"}
	for i, w := range want {
		if arr.Data[i] != w {
			t.Fatalf("Data[%d]=%v, want %q", i, arr.Data[i], w)
		}
	}
}

func TestUnmarshalMatBool(t *testing.T) {
	// 2 3⍴1 0 1 0 1 0, DR=11
	data := signedToUnsigned(
		-33, -92, 6, 0, 0, 0, 0, 0, 0, 0,
		47, 33, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		-88, 0, 0, 0, 0, 0, 0, 0, // 0xA8 = 10101000
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*codec.Array)
	if !ok {
		t.Fatalf("got %T, want *codec.Array", got)
	}
	assertShape(t, arr, []int{2, 3})
	assertIntSlice(t, arr.Data, []int{1, 0, 1, 0, 1, 0})
}

func TestUnmarshalMatFloat(t *testing.T) {
	// 2 2⍴1.1 2.2 3.3 4.4, DR=645
	data := signedToUnsigned(
		-33, -92, 9, 0, 0, 0, 0, 0, 0, 0,
		47, 37, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		-102, -103, -103, -103, -103, -103, -15, 63,
		-102, -103, -103, -103, -103, -103, 1, 64,
		102, 102, 102, 102, 102, 102, 10, 64,
		-102, -103, -103, -103, -103, -103, 17, 64,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*codec.Array)
	if !ok {
		t.Fatalf("got %T, want *codec.Array", got)
	}
	assertShape(t, arr, []int{2, 2})
	assertFloat(t, arr.Data[0], 1.1)
	assertFloat(t, arr.Data[1], 2.2)
	assertFloat(t, arr.Data[2], 3.3)
	assertFloat(t, arr.Data[3], 4.4)
}

func TestUnmarshalRank3(t *testing.T) {
	// 2 3 4⍴⍳24, DR=83
	data := signedToUnsigned(
		-33, -92, 9, 0, 0, 0, 0, 0, 0, 0,
		63, 34, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		4, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*codec.Array)
	if !ok {
		t.Fatalf("got %T, want *codec.Array", got)
	}
	assertShape(t, arr, []int{2, 3, 4})
	if len(arr.Data) != 24 {
		t.Fatalf("len=%d, want 24", len(arr.Data))
	}
	for i := range 24 {
		if arr.Data[i] != i+1 {
			t.Fatalf("Data[%d]=%v, want %d", i, arr.Data[i], i+1)
		}
	}
}

func TestUnmarshalNestedSimple(t *testing.T) {
	// (1 2)(3 4), DR=326
	data := signedToUnsigned(
		-33, -92, 6, 0, 0, 0, 0, 0, 0, 0,
		23, 6, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		// child 0: 1 2
		5, 0, 0, 0, 0, 0, 0, 0,
		31, 34, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 0, 0, 0, 0, 0, 0,
		// child 1: 3 4
		5, 0, 0, 0, 0, 0, 0, 0,
		31, 34, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 4, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(vec) != 2 {
		t.Fatalf("len=%d, want 2", len(vec))
	}
	// Each element is a []any
	v0, ok := vec[0].([]any)
	if !ok {
		t.Fatalf("vec[0] is %T, want []any", vec[0])
	}
	assertIntSlice(t, v0, []int{1, 2})
	v1, ok := vec[1].([]any)
	if !ok {
		t.Fatalf("vec[1] is %T, want []any", vec[1])
	}
	assertIntSlice(t, v1, []int{3, 4})
}

func TestUnmarshalNestedMixed(t *testing.T) {
	// 1 'hello' (2 3⍴⍳6), DR=326
	data := signedToUnsigned(
		-33, -92, 7, 0, 0, 0, 0, 0, 0, 0,
		23, 6, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		// child 0: scalar 1
		4, 0, 0, 0, 0, 0, 0, 0,
		15, 34, 0, 0, 0, 0, 0, 0,
		1, 0, 0, 0, 0, 0, 0, 0,
		// child 1: 'hello'
		5, 0, 0, 0, 0, 0, 0, 0,
		31, 39, 0, 0, 0, 0, 0, 0,
		5, 0, 0, 0, 0, 0, 0, 0,
		104, 101, 108, 108, 111, 0, 0, 0,
		// child 2: 2 3⍴⍳6
		6, 0, 0, 0, 0, 0, 0, 0,
		47, 34, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 4, 5, 6, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(vec) != 3 {
		t.Fatalf("len=%d, want 3", len(vec))
	}
	if vec[0] != 1 {
		t.Fatalf("vec[0]=%v, want 1", vec[0])
	}
	if vec[1] != "hello" {
		t.Fatalf("vec[1]=%v, want \"hello\"", vec[1])
	}
	arr, ok := vec[2].(*codec.Array)
	if !ok {
		t.Fatalf("vec[2] is %T, want *codec.Array", vec[2])
	}
	assertShape(t, arr, []int{2, 3})
	assertIntSlice(t, arr.Data, []int{1, 2, 3, 4, 5, 6})
}

func TestUnmarshalNestedDeep(t *testing.T) {
	// ⊂⊂1 2 3 — doubly enclosed vector
	data := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		7, 6, 0, 0, 0, 0, 0, 0,
		// child: ⊂1 2 3 (another enclosed)
		4, 0, 0, 0, 0, 0, 0, 0,
		7, 6, 0, 0, 0, 0, 0, 0,
		// grandchild: 1 2 3
		5, 0, 0, 0, 0, 0, 0, 0,
		31, 34, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	// ⊂⊂1 2 3 → unwrap both enclosures → 1 2 3
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any (1 2 3)", got)
	}
	assertIntSlice(t, vec, []int{1, 2, 3})
}

func TestUnmarshalNestedEmpty(t *testing.T) {
	// 0⍴⊂'', DR=326, shape=0
	data := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		23, 6, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		// prototype: '' (empty char vector)
		4, 0, 0, 0, 0, 0, 0, 0,
		31, 39, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	)
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.(*codec.Array)
	if !ok {
		t.Fatalf("got %T, want *codec.Array", got)
	}
	if len(arr.Data) != 0 {
		t.Fatalf("len=%d, want 0", len(arr.Data))
	}
	assertShape(t, arr, []int{0})
}

// --- Round-trip tests ---

func TestRoundtripScalarInt(t *testing.T) {
	for _, v := range []int{0, 1, -1, 42, -5, 127, -128, 1000, -1000, 1000000, 2147483647} {
		data, err := Marshal(v)
		if err != nil {
			t.Fatalf("Marshal(%d): %v", v, err)
		}
		got, err := Unmarshal(data)
		if err != nil {
			t.Fatalf("Unmarshal(%d): %v", v, err)
		}
		gotInt, ok := got.(int)
		if !ok {
			// Large ints may come back as float64
			if f, ok := got.(float64); ok {
				if f != float64(v) {
					t.Fatalf("roundtrip(%d): got %v", v, f)
				}
				continue
			}
			t.Fatalf("roundtrip(%d): got %T", v, got)
		}
		if gotInt != v {
			t.Fatalf("roundtrip(%d): got %d", v, gotInt)
		}
	}
}

func TestRoundtripScalarFloat(t *testing.T) {
	for _, v := range []float64{0.0, 3.14, -2.718, math.Pi, math.MaxFloat64} {
		data, err := Marshal(v)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", v, err)
		}
		got, err := Unmarshal(data)
		if err != nil {
			t.Fatalf("Unmarshal(%v): %v", v, err)
		}
		if got != v {
			t.Fatalf("roundtrip(%v): got %v", v, got)
		}
	}
}

func TestRoundtripScalarComplex(t *testing.T) {
	v := complex(1.0, 2.0)
	data, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Fatalf("got %v, want %v", got, v)
	}
}

func TestRoundtripString(t *testing.T) {
	for _, s := range []string{"", "X", "hello", "⍳⍴⍬", string([]rune{100000, 100001})} {
		data, err := Marshal(s)
		if err != nil {
			t.Fatalf("Marshal(%q): %v", s, err)
		}
		got, err := Unmarshal(data)
		if err != nil {
			t.Fatalf("Unmarshal(%q): %v", s, err)
		}
		if got != s {
			t.Fatalf("roundtrip(%q): got %q", s, got)
		}
	}
}

func TestRoundtripIntVector(t *testing.T) {
	vals := []any{1, 2, 3}
	data, err := Marshal(vals)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	assertIntSlice(t, vec, []int{1, 2, 3})
}

func TestRoundtripFloatVector(t *testing.T) {
	vals := []any{1.1, 2.2, 3.3}
	data, err := Marshal(vals)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(vec) != 3 {
		t.Fatalf("len=%d, want 3", len(vec))
	}
	assertFloat(t, vec[0], 1.1)
	assertFloat(t, vec[1], 2.2)
	assertFloat(t, vec[2], 3.3)
}

func TestRoundtripNestedVector(t *testing.T) {
	vals := []any{1, "hello", []any{3, 4}}
	data, err := Marshal(vals)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	vec, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(vec) != 3 {
		t.Fatalf("len=%d, want 3", len(vec))
	}
	if vec[0] != 1 {
		t.Fatalf("vec[0]=%v, want 1", vec[0])
	}
	if vec[1] != "hello" {
		t.Fatalf("vec[1]=%v, want \"hello\"", vec[1])
	}
	inner, ok := vec[2].([]any)
	if !ok {
		t.Fatalf("vec[2] is %T, want []any", vec[2])
	}
	assertIntSlice(t, inner, []int{3, 4})
}

func TestRoundtripMatrix(t *testing.T) {
	arr := &codec.Array{
		Data:  []any{1, 2, 3, 4, 5, 6},
		Shape: []int{2, 3},
	}
	data, err := Marshal(arr)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	gotArr, ok := got.(*codec.Array)
	if !ok {
		t.Fatalf("got %T, want *codec.Array", got)
	}
	assertShape(t, gotArr, []int{2, 3})
	assertIntSlice(t, gotArr.Data, []int{1, 2, 3, 4, 5, 6})
}

func TestRoundtripDecimal128(t *testing.T) {
	d := Decimal128{85, 85, 85, 85, 33, 0xDA, 0xE7, 103, 0x95, 0x82, 0xE4, 0x94, 88, 0xA4, 0xFC, 47}
	data, err := Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	gotD, ok := got.(Decimal128)
	if !ok {
		t.Fatalf("got %T, want Decimal128", got)
	}
	if gotD != d {
		t.Fatalf("got %v, want %v", gotD, d)
	}
}

// --- Dyalog interop test (exact bytes from probing) ---

func TestExactBytesFromDyalog(t *testing.T) {
	// Verify our Marshal produces bytes that Dyalog generated.
	// 'ab' from the official docs (but 64-bit version)
	// Dyalog gave us for 'hello': specific bytes. Let's verify 'hello' round-trips.
	dyalogHello := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		31, 39, 0, 0, 0, 0, 0, 0,
		5, 0, 0, 0, 0, 0, 0, 0,
		104, 101, 108, 108, 111, 0, 0, 0,
	)
	marshalled, err := Marshal("hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(marshalled) != len(dyalogHello) {
		t.Fatalf("length mismatch: got %d, want %d", len(marshalled), len(dyalogHello))
	}
	for i := range marshalled {
		if marshalled[i] != dyalogHello[i] {
			t.Fatalf("byte %d: got 0x%02X, want 0x%02X", i, marshalled[i], dyalogHello[i])
		}
	}
}

func TestExactBytesScalarInt42(t *testing.T) {
	dyalog := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 34, 0, 0, 0, 0, 0, 0,
		42, 0, 0, 0, 0, 0, 0, 0,
	)
	marshalled, err := Marshal(42)
	if err != nil {
		t.Fatal(err)
	}
	assertBytesEqual(t, marshalled, dyalog)
}

func TestExactBytesVecInt(t *testing.T) {
	dyalog := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		31, 34, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 0, 0, 0, 0, 0,
	)
	marshalled, err := Marshal([]any{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	assertBytesEqual(t, marshalled, dyalog)
}

func TestExactBytesMatInt(t *testing.T) {
	dyalog := signedToUnsigned(
		-33, -92, 6, 0, 0, 0, 0, 0, 0, 0,
		47, 34, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 4, 5, 6, 0, 0,
	)
	marshalled, err := Marshal(&codec.Array{
		Data:  []any{1, 2, 3, 4, 5, 6},
		Shape: []int{2, 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertBytesEqual(t, marshalled, dyalog)
}

func TestExactBytesScalarFloat(t *testing.T) {
	dyalog := signedToUnsigned(
		-33, -92, 4, 0, 0, 0, 0, 0, 0, 0,
		15, 37, 0, 0, 0, 0, 0, 0,
		31, -123, -21, 81, -72, 30, 9, 64,
	)
	marshalled, err := Marshal(3.14)
	if err != nil {
		t.Fatal(err)
	}
	assertBytesEqual(t, marshalled, dyalog)
}

func TestExactBytesScalarComplex(t *testing.T) {
	dyalog := signedToUnsigned(
		-33, -92, 5, 0, 0, 0, 0, 0, 0, 0,
		15, 42, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, -16, 63,
		0, 0, 0, 0, 0, 0, 0, 64,
	)
	marshalled, err := Marshal(complex(1.0, 2.0))
	if err != nil {
		t.Fatal(err)
	}
	assertBytesEqual(t, marshalled, dyalog)
}

func TestExactBytesNestedMixed(t *testing.T) {
	dyalog := signedToUnsigned(
		-33, -92, 7, 0, 0, 0, 0, 0, 0, 0,
		23, 6, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		// child 0: scalar 1
		4, 0, 0, 0, 0, 0, 0, 0,
		15, 34, 0, 0, 0, 0, 0, 0,
		1, 0, 0, 0, 0, 0, 0, 0,
		// child 1: 'hello'
		5, 0, 0, 0, 0, 0, 0, 0,
		31, 39, 0, 0, 0, 0, 0, 0,
		5, 0, 0, 0, 0, 0, 0, 0,
		104, 101, 108, 108, 111, 0, 0, 0,
		// child 2: 2 3⍴⍳6
		6, 0, 0, 0, 0, 0, 0, 0,
		47, 34, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0,
		1, 2, 3, 4, 5, 6, 0, 0,
	)
	marshalled, err := Marshal([]any{
		1,
		"hello",
		&codec.Array{Data: []any{1, 2, 3, 4, 5, 6}, Shape: []int{2, 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertBytesEqual(t, marshalled, dyalog)
}

// --- Helpers ---

func assertIntSlice(t *testing.T, got []any, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d; got=%v", len(got), len(want), got)
	}
	for i := range want {
		g, ok := got[i].(int)
		if !ok {
			t.Fatalf("[%d] got %T (%v), want int", i, got[i], got[i])
		}
		if g != want[i] {
			t.Fatalf("[%d] got %d, want %d", i, g, want[i])
		}
	}
}

func assertShape(t *testing.T, arr *codec.Array, want []int) {
	t.Helper()
	if len(arr.Shape) != len(want) {
		t.Fatalf("rank=%d, want %d", len(arr.Shape), len(want))
	}
	for i := range want {
		if arr.Shape[i] != want[i] {
			t.Fatalf("shape[%d]=%d, want %d", i, arr.Shape[i], want[i])
		}
	}
}

func assertBytesEqual(t *testing.T, got, want []byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("byte %d: got 0x%02X, want 0x%02X", i, got[i], want[i])
		}
	}
}

func assertFloat(t *testing.T, got any, want float64) {
	t.Helper()
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("got %T, want float64", got)
	}
	if math.Abs(f-want) > 1e-10 {
		t.Fatalf("got %v, want %v", f, want)
	}
}
