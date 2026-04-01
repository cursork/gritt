package prepl

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cursork/gritt/amicable"
	"github.com/cursork/gritt/codec"
)

// startAplsock builds and launches aplsock in the given mode, returns a connected client.
func startAplsock(t *testing.T, mode string) (*Client, func()) {
	t.Helper()

	if err := exec.Command("go", "build", "-o", "/tmp/test-aplsock-modes", "../grittles/aplsock/").Run(); err != nil {
		t.Fatalf("build aplsock: %v", err)
	}

	port := 14300 + os.Getpid()%1000
	cmd := exec.Command("/tmp/test-aplsock-modes", "-l", "-sock", fmt.Sprintf(":%d", port), "-mode", mode)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start aplsock: %v", err)
	}

	cleanup := func() {
		cmd.Process.Kill()
		cmd.Wait()
	}

	var client *Client
	var err error
	for i := 0; i < 50; i++ {
		client, err = Connect(fmt.Sprintf("localhost:%d", port))
		if err == nil {
			return client, cleanup
		}
		time.Sleep(200 * time.Millisecond)
	}
	cleanup()
	t.Fatalf("connect to aplsock (mode=%s): %v", mode, err)
	return nil, nil
}

// TestModeAplan tests the default APLAN mode.
func TestModeAplan(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	client, cleanup := startAplsock(t, "aplan")
	defer cleanup()
	defer client.Close()

	t.Run("scalar", func(t *testing.T) {
		resp, err := client.Eval("42")
		if err != nil {
			t.Fatal(err)
		}
		if resp.Tag != "ret" {
			t.Fatalf("tag=%s, want ret", resp.Tag)
		}
		if resp.Val != 42 {
			t.Fatalf("val=%v, want 42", resp.Val)
		}
	})

	t.Run("vector", func(t *testing.T) {
		resp, err := client.Eval("⍳3")
		if err != nil {
			t.Fatal(err)
		}
		vec, ok := resp.Val.([]any)
		if !ok {
			t.Fatalf("val is %T, want []any", resp.Val)
		}
		if len(vec) != 3 {
			t.Fatalf("len=%d, want 3", len(vec))
		}
	})

	t.Run("string", func(t *testing.T) {
		resp, err := client.Eval("'hello'")
		if err != nil {
			t.Fatal(err)
		}
		if resp.Val != "hello" {
			t.Fatalf("val=%v, want 'hello'", resp.Val)
		}
	})

	t.Run("error", func(t *testing.T) {
		resp, err := client.Eval("1÷0")
		if err != nil {
			t.Fatal(err)
		}
		if resp.Tag != "err" {
			t.Fatalf("tag=%s, want err", resp.Tag)
		}
		if resp.Err.EN != 11 {
			t.Fatalf("EN=%d, want 11", resp.Err.EN)
		}
	})

	t.Run("namespace", func(t *testing.T) {
		client.Eval("ns←⎕NS ''")
		client.Eval("ns.x←42")
		resp, err := client.Eval("ns")
		if err != nil {
			t.Fatal(err)
		}
		ns, ok := resp.Val.(*codec.Namespace)
		if !ok {
			t.Fatalf("val is %T, want *codec.Namespace", resp.Val)
		}
		if ns.Values["x"] != 42 {
			t.Fatalf("ns.x=%v, want 42", ns.Values["x"])
		}
	})
}

// TestModePlain tests plain text mode.
func TestModePlain(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	client, cleanup := startAplsock(t, "plain")
	defer cleanup()
	defer client.Close()

	// Plain mode returns display text, not APLAN. Use EvalRaw.
	t.Run("scalar", func(t *testing.T) {
		raw, err := client.EvalRaw("42")
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(raw) != "42" {
			t.Fatalf("got %q, want '42'", raw)
		}
	})

	t.Run("vector", func(t *testing.T) {
		raw, err := client.EvalRaw("⍳5")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(raw, "1 2 3 4 5") {
			t.Fatalf("got %q, want '1 2 3 4 5'", raw)
		}
	})

	t.Run("string", func(t *testing.T) {
		raw, err := client.EvalRaw("'hello'")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(raw, "hello") {
			t.Fatalf("got %q, want 'hello'", raw)
		}
	})

	t.Run("error", func(t *testing.T) {
		raw, err := client.EvalRaw("1÷0")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(raw, "ivide") && !strings.Contains(raw, "DOMAIN") {
			t.Fatalf("got %q, want error message", raw)
		}
	})

	t.Run("not_aplan", func(t *testing.T) {
		raw, err := client.EvalRaw("42")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(raw, "tag:") {
			t.Fatalf("plain mode should not return APLAN, got %q", raw)
		}
	})
}

// TestModeAplor tests 220⌶ binary mode.
func TestModeAplor(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	client, cleanup := startAplsock(t, "aplor")
	defer cleanup()
	defer client.Close()

	// In aplor mode, the response is a 220⌶ serialized namespace.
	// amicable.Unmarshal returns *codec.Namespace with typed members.

	t.Run("scalar", func(t *testing.T) {
		ns := aplorEval(t, client, "42")
		if ns.Values["tag"] != "ret" {
			t.Fatalf("tag=%v, want 'ret'", ns.Values["tag"])
		}
		if ns.Values["val"] != 42 {
			t.Fatalf("val=%v, want 42", ns.Values["val"])
		}
	})

	t.Run("string", func(t *testing.T) {
		ns := aplorEval(t, client, "'hello'")
		if ns.Values["val"] != "hello" {
			t.Fatalf("val=%v, want 'hello'", ns.Values["val"])
		}
	})

	t.Run("error", func(t *testing.T) {
		ns := aplorEval(t, client, "1÷0")
		if ns.Values["tag"] != "err" {
			t.Fatalf("tag=%v, want 'err'", ns.Values["tag"])
		}
		if ns.Values["en"] != 11 {
			t.Fatalf("en=%v, want 11", ns.Values["en"])
		}
	})

	t.Run("function_roundtrip", func(t *testing.T) {
		t.Skip("Embedded function blobs use different encoding than standalone ⎕OR (tradfn-style literal indices, 152-byte header). See deliberanda/namespace-unmarshal.md.")
		ns := aplorEval(t, client, "{⍵+1}{f←⍺⍺⋄⎕OR'f'}⍬")
		if ns.Values["tag"] != "ret" {
			t.Fatalf("tag=%v, want 'ret'", ns.Values["tag"])
		}
		r, ok := ns.Values["val"].(amicable.Raw)
		if !ok {
			t.Fatalf("val is %T, want amicable.Raw", ns.Values["val"])
		}
		src, err := r.Decompile()
		if err != nil {
			t.Fatalf("decompile: %v", err)
		}
		if src != "{⍵+1}" {
			t.Fatalf("decompiled=%q, want {⍵+1}", src)
		}
	})
}

// aplorEval sends an expression in aplor mode, returns the response namespace.
func aplorEval(t *testing.T, client *Client, expr string) *codec.Namespace {
	t.Helper()
	raw, err := client.EvalRaw(expr)
	if err != nil {
		t.Fatalf("EvalRaw(%q): %v", expr, err)
	}
	data := parseSignedInts(t, raw)
	val, err := amicable.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal(%q): %v", expr, err)
	}
	ns, ok := val.(*codec.Namespace)
	if !ok {
		t.Fatalf("expected *codec.Namespace, got %T", val)
	}
	return ns
}

func parseSignedInts(t *testing.T, s string) []byte {
	t.Helper()
	s = strings.ReplaceAll(s, "¯", "-")
	fields := strings.Fields(s)
	data := make([]byte, len(fields))
	for i, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("parse byte %d %q: %v", i, f, err)
		}
		data[i] = byte(int8(v))
	}
	return data
}
