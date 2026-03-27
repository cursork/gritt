package prepl

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/cursork/gritt/codec"
)

// TestIntegration runs the prepl client against a live aplsock.
// Skips if aplsock/gritt aren't available.
func TestIntegration(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	// Build aplsock
	if err := exec.Command("go", "build", "-o", "/tmp/test-aplsock", "../grittles/aplsock/").Run(); err != nil {
		t.Fatalf("build aplsock: %v", err)
	}

	// Launch aplsock with auto-launch Dyalog on a random port
	port := 14200 + os.Getpid()%1000
	cmd := exec.Command("/tmp/test-aplsock", "-l", "-sock", fmt.Sprintf(":%d", port))
	cmd.Env = append(os.Environ(), "RIDE_SPAWNED=1", "DYALOG_LINEEDITOR_MODE=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start aplsock: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Wait for aplsock to be ready
	var client *Client
	var err error
	for i := 0; i < 50; i++ {
		client, err = Connect(fmt.Sprintf("localhost:%d", port))
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("connect to aplsock: %v", err)
	}
	defer client.Close()

	// Helper
	eval := func(t *testing.T, expr string) *Response {
		t.Helper()
		resp, err := client.Eval(expr)
		if err != nil {
			t.Fatalf("Eval(%q): %v", expr, err)
		}
		if resp.Tag == "err" {
			t.Fatalf("Eval(%q): APL error: %s", expr, resp.Err.Message)
		}
		return resp
	}

	// === Scalar types ===

	t.Run("scalar_int", func(t *testing.T) {
		r := eval(t, "42")
		assertVal(t, r.Val, 42)
	})

	t.Run("scalar_neg", func(t *testing.T) {
		r := eval(t, "¯7")
		assertVal(t, r.Val, -7)
	})

	t.Run("scalar_float", func(t *testing.T) {
		r := eval(t, "○1")
		f, ok := r.Val.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", r.Val)
		}
		if math.Abs(f-math.Pi) > 1e-10 {
			t.Fatalf("got %v, want π", f)
		}
	})

	t.Run("scalar_complex", func(t *testing.T) {
		r := eval(t, "1J2")
		c, ok := r.Val.(complex128)
		if !ok {
			t.Fatalf("expected complex128, got %T: %v", r.Val, r.Val)
		}
		if c != complex(1, 2) {
			t.Fatalf("got %v, want 1J2", c)
		}
	})

	t.Run("scalar_bool", func(t *testing.T) {
		r := eval(t, "1=1")
		assertVal(t, r.Val, 1)
	})

	t.Run("scalar_char", func(t *testing.T) {
		r := eval(t, "'X'")
		if r.Val != "X" {
			t.Fatalf("got %v, want 'X'", r.Val)
		}
	})

	// === Vector types ===

	t.Run("vec_int", func(t *testing.T) {
		r := eval(t, "⍳5")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		if len(vec) != 5 {
			t.Fatalf("len=%d, want 5", len(vec))
		}
		for i, v := range vec {
			assertVal(t, v, i+1)
		}
	})

	t.Run("vec_float", func(t *testing.T) {
		r := eval(t, "1.1 2.2 3.3")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		if len(vec) != 3 {
			t.Fatalf("len=%d, want 3", len(vec))
		}
		assertFloat(t, vec[0], 1.1)
		assertFloat(t, vec[1], 2.2)
		assertFloat(t, vec[2], 3.3)
	})

	t.Run("vec_string", func(t *testing.T) {
		r := eval(t, "'hello world'")
		if r.Val != "hello world" {
			t.Fatalf("got %q, want 'hello world'", r.Val)
		}
	})

	t.Run("vec_unicode", func(t *testing.T) {
		r := eval(t, "'⍳⍴⍬'")
		if r.Val != "⍳⍴⍬" {
			t.Fatalf("got %q, want '⍳⍴⍬'", r.Val)
		}
	})

	t.Run("vec_bool", func(t *testing.T) {
		r := eval(t, "1 0 1 1 0")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		want := []int{1, 0, 1, 1, 0}
		if len(vec) != len(want) {
			t.Fatalf("len=%d, want %d", len(vec), len(want))
		}
		for i, w := range want {
			assertVal(t, vec[i], w)
		}
	})

	t.Run("vec_complex", func(t *testing.T) {
		r := eval(t, "1J2 3J4")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		if len(vec) != 2 {
			t.Fatalf("len=%d, want 2", len(vec))
		}
		assertComplex(t, vec[0], 1, 2)
		assertComplex(t, vec[1], 3, 4)
	})

	t.Run("vec_empty_char", func(t *testing.T) {
		r := eval(t, "''")
		if r.Val != "" {
			t.Fatalf("got %v, want empty string", r.Val)
		}
	})

	t.Run("vec_empty_num", func(t *testing.T) {
		r := eval(t, "⍬")
		// Zilde should come back as empty or codec.Zilde
		switch v := r.Val.(type) {
		case []any:
			if len(v) != 0 {
				t.Fatalf("got %v, want empty", v)
			}
		default:
			if v != codec.Zilde {
				t.Fatalf("got %T %v, want zilde", v, v)
			}
		}
	})

	// === Matrix / shaped ===

	t.Run("matrix", func(t *testing.T) {
		r := eval(t, "2 3⍴⍳6")
		arr, ok := r.Val.(*codec.Array)
		if !ok {
			t.Fatalf("expected *codec.Array, got %T", r.Val)
		}
		if len(arr.Shape) != 2 || arr.Shape[0] != 2 || arr.Shape[1] != 3 {
			t.Fatalf("shape=%v, want [2 3]", arr.Shape)
		}
		// APLAN codec stores matrices as nested rows
		if len(arr.Data) != 2 {
			t.Fatalf("data len=%d, want 2 (rows)", len(arr.Data))
		}
		row0, ok := arr.Data[0].([]any)
		if !ok {
			t.Fatalf("row 0 is %T, want []any", arr.Data[0])
		}
		assertVal(t, row0[0], 1)
		assertVal(t, row0[2], 3)
		row1, ok := arr.Data[1].([]any)
		if !ok {
			t.Fatalf("row 1 is %T, want []any", arr.Data[1])
		}
		assertVal(t, row1[0], 4)
		assertVal(t, row1[2], 6)
	})

	t.Run("matrix_char", func(t *testing.T) {
		r := eval(t, "2 3⍴'abcdef'")
		arr, ok := r.Val.(*codec.Array)
		if !ok {
			t.Fatalf("expected *codec.Array, got %T", r.Val)
		}
		if len(arr.Shape) != 2 || arr.Shape[0] != 2 || arr.Shape[1] != 3 {
			t.Fatalf("shape=%v, want [2 3]", arr.Shape)
		}
		// Rows are flattened to individual chars
		row0, ok := arr.Data[0].([]any)
		if !ok {
			t.Fatalf("row 0 is %T, want []any", arr.Data[0])
		}
		if row0[0] != "a" || row0[1] != "b" || row0[2] != "c" {
			t.Fatalf("row 0=%v, want [a b c]", row0)
		}
		row1, ok := arr.Data[1].([]any)
		if !ok {
			t.Fatalf("row 1 is %T, want []any", arr.Data[1])
		}
		if row1[0] != "d" || row1[1] != "e" || row1[2] != "f" {
			t.Fatalf("row 1=%v, want [d e f]", row1)
		}
	})

	t.Run("rank3", func(t *testing.T) {
		r := eval(t, "2 3 4⍴⍳24")
		arr, ok := r.Val.(*codec.Array)
		if !ok {
			t.Fatalf("expected *codec.Array, got %T", r.Val)
		}
		if len(arr.Shape) != 3 || arr.Shape[0] != 2 || arr.Shape[1] != 3 || arr.Shape[2] != 4 {
			t.Fatalf("shape=%v, want [2 3 4]", arr.Shape)
		}
		// Rank-3: Data has 2 elements (planes), each a sub-array
		if len(arr.Data) != 2 {
			t.Fatalf("data len=%d, want 2 (planes)", len(arr.Data))
		}
	})

	// === Nested structures ===

	t.Run("nested_simple", func(t *testing.T) {
		r := eval(t, "(1 2)(3 4)")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		if len(vec) != 2 {
			t.Fatalf("len=%d, want 2", len(vec))
		}
		inner0, ok := vec[0].([]any)
		if !ok {
			t.Fatalf("vec[0] is %T, want []any", vec[0])
		}
		assertVal(t, inner0[0], 1)
		assertVal(t, inner0[1], 2)
		inner1, ok := vec[1].([]any)
		if !ok {
			t.Fatalf("vec[1] is %T, want []any", vec[1])
		}
		assertVal(t, inner1[0], 3)
		assertVal(t, inner1[1], 4)
	})

	t.Run("nested_mixed", func(t *testing.T) {
		r := eval(t, "1 'hello' (2 3⍴⍳6)")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		if len(vec) != 3 {
			t.Fatalf("len=%d, want 3", len(vec))
		}
		assertVal(t, vec[0], 1)
		if vec[1] != "hello" {
			t.Fatalf("vec[1]=%v, want 'hello'", vec[1])
		}
		arr, ok := vec[2].(*codec.Array)
		if !ok {
			t.Fatalf("vec[2] is %T, want *codec.Array", vec[2])
		}
		if arr.Shape[0] != 2 || arr.Shape[1] != 3 {
			t.Fatalf("vec[2] shape=%v, want [2 3]", arr.Shape)
		}
		row0, ok := arr.Data[0].([]any)
		if !ok {
			t.Fatalf("vec[2] row 0 is %T, want []any", arr.Data[0])
		}
		assertVal(t, row0[0], 1)
	})

	t.Run("nested_deep", func(t *testing.T) {
		r := eval(t, "(1 (2 (3 4)))")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		if len(vec) != 2 {
			t.Fatalf("len=%d, want 2", len(vec))
		}
		assertVal(t, vec[0], 1)
		inner, ok := vec[1].([]any)
		if !ok {
			t.Fatalf("vec[1] is %T, want []any", vec[1])
		}
		assertVal(t, inner[0], 2)
		deepest, ok := inner[1].([]any)
		if !ok {
			t.Fatalf("inner[1] is %T, want []any", inner[1])
		}
		assertVal(t, deepest[0], 3)
		assertVal(t, deepest[1], 4)
	})

	t.Run("nested_namespace", func(t *testing.T) {
		client.Eval("ns←⎕NS ''")    // ignore result — empty ns
		client.Eval("ns.x←42")      // ignore shy assignment result
		client.Eval("ns.name←'Neil'")
		r := eval(t, "ns")
		ns, ok := r.Val.(*codec.Namespace)
		if !ok {
			t.Fatalf("expected *codec.Namespace, got %T: %v", r.Val, r.Val)
		}
		if ns.Values["x"] != 42 {
			t.Fatalf("ns.x=%v, want 42", ns.Values["x"])
		}
		if ns.Values["name"] != "Neil" {
			t.Fatalf("ns.name=%v, want 'Neil'", ns.Values["name"])
		}
	})

	// === Error handling ===

	t.Run("error_domain", func(t *testing.T) {
		resp, err := client.Eval("1÷0")
		if err != nil {
			t.Fatal(err)
		}
		if resp.Tag != "err" {
			t.Fatalf("expected err, got %s", resp.Tag)
		}
		if resp.Err.EN != 11 {
			t.Fatalf("EN=%d, want 11", resp.Err.EN)
		}
		if !strings.Contains(resp.Err.Message, "ivide") && !strings.Contains(resp.Err.Message, "DOMAIN") {
			t.Fatalf("message=%q, want divide-related error", resp.Err.Message)
		}
	})

	t.Run("error_recovery", func(t *testing.T) {
		// Error then success
		client.Eval("÷0")
		r := eval(t, "1+1")
		assertVal(t, r.Val, 2)
	})

	t.Run("shy_result", func(t *testing.T) {
		r := eval(t, "x←42")
		// Shy assignment may or may not return a value
		if r.Val != nil {
			assertVal(t, r.Val, 42)
		}
	})

	// === ID correlation ===

	t.Run("id_correlation", func(t *testing.T) {
		id := UUIDv7()
		resp, err := client.Eval("1+1", id)
		if err != nil {
			t.Fatal(err)
		}
		if resp.ID != id {
			t.Fatalf("ID=%q, want %q", resp.ID, id)
		}
		assertVal(t, resp.Val, 2)
	})

	// === Raw mode ===

	t.Run("raw_response", func(t *testing.T) {
		raw, err := client.EvalRaw("⍳3")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(raw, "tag:") || !strings.Contains(raw, "ret") {
			t.Fatalf("raw=%q, expected APLAN namespace", raw)
		}
		if !strings.Contains(raw, "val:") {
			t.Fatalf("raw=%q, expected val field", raw)
		}
	})

	// === Large values ===

	t.Run("large_vector", func(t *testing.T) {
		r := eval(t, "⍳1000")
		vec, ok := r.Val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", r.Val)
		}
		if len(vec) != 1000 {
			t.Fatalf("len=%d, want 1000", len(vec))
		}
		assertVal(t, vec[0], 1)
		assertVal(t, vec[999], 1000)
	})

	t.Run("large_matrix", func(t *testing.T) {
		r := eval(t, "10 10⍴⍳100")
		arr, ok := r.Val.(*codec.Array)
		if !ok {
			t.Fatalf("expected *codec.Array, got %T", r.Val)
		}
		if arr.Shape[0] != 10 || arr.Shape[1] != 10 {
			t.Fatalf("shape=%v, want [10 10]", arr.Shape)
		}
		// Rows are nested
		if len(arr.Data) != 10 {
			t.Fatalf("data len=%d, want 10 (rows)", len(arr.Data))
		}
		row0, ok := arr.Data[0].([]any)
		if !ok {
			t.Fatalf("row 0 is %T, want []any", arr.Data[0])
		}
		assertVal(t, row0[0], 1)
		assertVal(t, row0[9], 10)
		row9, ok := arr.Data[9].([]any)
		if !ok {
			t.Fatalf("row 9 is %T, want []any", arr.Data[9])
		}
		assertVal(t, row9[0], 91)
		assertVal(t, row9[9], 100)
	})
}

func assertVal(t *testing.T, got any, want int) {
	t.Helper()
	switch v := got.(type) {
	case int:
		if v != want {
			t.Fatalf("got %d, want %d", v, want)
		}
	case float64:
		if int(v) != want {
			t.Fatalf("got %v, want %d", v, want)
		}
	default:
		t.Fatalf("got %T (%v), want int %d", got, got, want)
	}
}

func assertFloat(t *testing.T, got any, want float64) {
	t.Helper()
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("got %T (%v), want float64 %v", got, got, want)
	}
	if math.Abs(f-want) > 1e-10 {
		t.Fatalf("got %v, want %v", f, want)
	}
}

func assertComplex(t *testing.T, got any, wantRe, wantIm float64) {
	t.Helper()
	c, ok := got.(complex128)
	if !ok {
		t.Fatalf("got %T (%v), want complex %v+%vi", got, got, wantRe, wantIm)
	}
	if real(c) != wantRe || imag(c) != wantIm {
		t.Fatalf("got %v, want %v+%vi", c, wantRe, wantIm)
	}
}
