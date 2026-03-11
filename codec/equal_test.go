package codec

import (
	"math"
	"testing"
)

func TestEqualScalars(t *testing.T) {
	tests := []struct {
		a, b any
		want bool
	}{
		{42, 42, true},
		{42, 43, false},
		{3.14, 3.14, true},
		{"hello", "hello", true},
		{"hello", "world", false},
		{42, "42", false},
		{complex(1, 2), complex(1, 2), true},
		{complex(1, 2), complex(1, 3), false},
	}
	for _, tt := range tests {
		if got := Equal(tt.a, tt.b); got != tt.want {
			t.Errorf("Equal(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestEqualNaN(t *testing.T) {
	if !Equal(math.NaN(), math.NaN()) {
		t.Error("Equal(NaN, NaN) should be true")
	}
}

func TestEqualZilde(t *testing.T) {
	if !Equal(Zilde, []any{}) {
		t.Error("Equal(Zilde, []any{}) should be true")
	}
	if !Equal([]any{}, Zilde) {
		t.Error("Equal([]any{}, Zilde) should be true")
	}
}

func TestEqualSlices(t *testing.T) {
	if !Equal([]any{1, 2, 3}, []any{1, 2, 3}) {
		t.Error("equal slices should be true")
	}
	if Equal([]any{1, 2}, []any{1, 2, 3}) {
		t.Error("different length slices should be false")
	}
}

func TestEqualArrays(t *testing.T) {
	a := &Array{Data: []any{[]any{1, 2}, []any{3, 4}}, Shape: []int{2, 2}}
	b := &Array{Data: []any{[]any{1, 2}, []any{3, 4}}, Shape: []int{2, 2}}
	if !Equal(a, b) {
		t.Error("equal arrays should be true")
	}

	c := &Array{Data: []any{[]any{1, 2}, []any{3, 5}}, Shape: []int{2, 2}}
	if Equal(a, c) {
		t.Error("different data should be false")
	}
}

func TestEqualNamespaces(t *testing.T) {
	a := &Namespace{Keys: []string{"x", "y"}, Values: map[string]any{"x": 1, "y": 2}}
	b := &Namespace{Keys: []string{"x", "y"}, Values: map[string]any{"x": 1, "y": 2}}
	if !Equal(a, b) {
		t.Error("equal namespaces should be true")
	}

	c := &Namespace{Keys: []string{"x", "y"}, Values: map[string]any{"x": 1, "y": 3}}
	if Equal(a, c) {
		t.Error("different values should be false")
	}
}
