package dcf

import (
	"path/filepath"
	"testing"

	"github.com/cursork/gritt/codec"
)

func TestOpenEmpty(t *testing.T) {
	f, err := Open(filepath.Join("testdata", "empty.dcf"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got, want := f.Header().Version, uint32(0x14); got != want {
		t.Errorf("Version = 0x%X, want 0x%X", got, want)
	}
	if got, want := f.Header().NextFree, uint32(1); got != want {
		t.Errorf("NextFree = %d, want %d", got, want)
	}
	if got := len(f.Components()); got != 0 {
		t.Errorf("Components = %d, want 0", got)
	}
}

func TestOpenOneChar(t *testing.T) {
	f, err := Open(filepath.Join("testdata", "one_char.dcf"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got, want := f.Header().NextFree, uint32(2); got != want {
		t.Errorf("NextFree = %d, want %d", got, want)
	}
	cs := f.Components()
	if len(cs) != 1 {
		t.Fatalf("Components = %d, want 1", len(cs))
	}
	c := cs[0]
	if c.Number != 1 {
		t.Errorf("Number = %d, want 1", c.Number)
	}
	if c.DR != 80 {
		t.Errorf("DR = %d, want 80 (Char8)", c.DR)
	}
	if c.Rank != 1 || len(c.Shape) != 1 || c.Shape[0] != 5 {
		t.Errorf("Rank/Shape = %d/%v, want 1/[5]", c.Rank, c.Shape)
	}
	val, err := f.Read(1)
	if err != nil {
		t.Fatalf("Read(1): %v", err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("Read(1) = %T %v, want string", val, val)
	}
	if s != "hello" {
		t.Errorf("Read(1) = %q, want %q", s, "hello")
	}
}

func TestOpenOneInt(t *testing.T) {
	f, err := Open(filepath.Join("testdata", "one_int.dcf"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cs := f.Components()
	if len(cs) != 1 {
		t.Fatalf("Components = %d, want 1", len(cs))
	}
	c := cs[0]
	if c.DR != 83 {
		t.Errorf("DR = %d, want 83 (Int8)", c.DR)
	}
	if c.Shape[0] != 5 {
		t.Errorf("Shape[0] = %d, want 5", c.Shape[0])
	}
	val, err := f.Read(1)
	if err != nil {
		t.Fatalf("Read(1): %v", err)
	}
	// amicable returns []any of ints for an integer vector.
	got, ok := val.([]any)
	if !ok {
		t.Fatalf("Read(1) = %T, want []any", val)
	}
	if len(got) != 5 {
		t.Fatalf("len(Read(1)) = %d, want 5", len(got))
	}
	for i, v := range got {
		want := i + 1
		if n, ok := v.(int); !ok || n != want {
			t.Errorf("[%d] = %v (%T), want %d", i, v, v, want)
		}
	}
}

func TestOpenThreeComponents(t *testing.T) {
	f, err := Open(filepath.Join("testdata", "three_components.dcf"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cs := f.Components()
	if len(cs) != 3 {
		t.Fatalf("Components = %d, want 3", len(cs))
	}
	if cs[0].Number != 1 || cs[1].Number != 2 || cs[2].Number != 3 {
		t.Errorf("Numbers = %d,%d,%d; want 1,2,3", cs[0].Number, cs[1].Number, cs[2].Number)
	}
	// Component 1: 'first'
	v1, err := f.Read(1)
	if err != nil {
		t.Fatalf("Read(1): %v", err)
	}
	if s, _ := v1.(string); s != "first" {
		t.Errorf("Read(1) = %q, want %q", v1, "first")
	}
	// Component 2: ⍳10 → 1..10 as int8 vector
	v2, err := f.Read(2)
	if err != nil {
		t.Fatalf("Read(2): %v", err)
	}
	if arr, ok := v2.([]any); !ok || len(arr) != 10 {
		t.Errorf("Read(2) shape: got %T %v, want []any len 10", v2, v2)
	}
	// Component 3: 2 3⍴⍳6 — rank-2 array.
	v3, err := f.Read(3)
	if err != nil {
		t.Fatalf("Read(3): %v", err)
	}
	arr, ok := v3.(*codec.Array)
	if !ok {
		t.Fatalf("Read(3) = %T, want *codec.Array", v3)
	}
	if len(arr.Shape) != 2 || arr.Shape[0] != 2 || arr.Shape[1] != 3 {
		t.Errorf("Read(3) shape = %v, want [2 3]", arr.Shape)
	}
}
