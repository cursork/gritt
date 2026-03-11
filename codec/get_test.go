package codec

import "testing"

func TestGetSimpleArray(t *testing.T) {
	arr := []any{1, 2, 3}
	for i, want := range []int{1, 2, 3} {
		got, err := Get(arr, i)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Get(arr, %d) = %v, want %d", i, got, want)
		}
	}
}

func TestGetNestedArray(t *testing.T) {
	arr := []any{[]any{1, 2}, []any{3, 4}}
	tests := []struct {
		indices []int
		want    int
	}{
		{[]int{0, 0}, 1},
		{[]int{0, 1}, 2},
		{[]int{1, 0}, 3},
		{[]int{1, 1}, 4},
	}
	for _, tt := range tests {
		got, err := Get(arr, tt.indices...)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("Get(arr, %v) = %v, want %d", tt.indices, got, tt.want)
		}
	}
}

func TestGetMatrix(t *testing.T) {
	mat, err := APLAN("[1 2 ⋄ 3 4]")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		indices []int
		want    int
	}{
		{[]int{0, 0}, 1},
		{[]int{0, 1}, 2},
		{[]int{1, 0}, 3},
		{[]int{1, 1}, 4},
	}
	for _, tt := range tests {
		got, err := Get(mat, tt.indices...)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("Get(mat, %v) = %v, want %d", tt.indices, got, tt.want)
		}
	}
}

func TestGetMatrixRankMismatch(t *testing.T) {
	mat, _ := APLAN("[1 2 ⋄ 3 4]")
	_, err := Get(mat, 0)
	if err == nil {
		t.Error("should error on rank mismatch")
	}
}

func TestGetZilde(t *testing.T) {
	_, err := Get(Zilde, 0)
	if err == nil {
		t.Error("should error on zilde")
	}
}

func TestGetOutOfBounds(t *testing.T) {
	_, err := Get([]any{1, 2, 3}, 5)
	if err == nil {
		t.Error("should error on out of bounds")
	}
}

func TestGetParsedNestedVector(t *testing.T) {
	parsed, err := APLAN("((1 2) ⋄ (3 4))")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Get(parsed, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("Get(parsed, 0, 0) = %v, want 1", got)
	}
	got, err = Get(parsed, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != 4 {
		t.Errorf("Get(parsed, 1, 1) = %v, want 4", got)
	}
}
