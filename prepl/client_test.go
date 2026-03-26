package prepl

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- UUIDv7 ---

func TestUUIDv7Format(t *testing.T) {
	id := UUIDv7()
	// 8-4-4-4-12 hex format
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(id) {
		t.Errorf("UUIDv7() = %q, does not match UUIDv7 pattern", id)
	}
}

func TestUUIDv7Version(t *testing.T) {
	id := UUIDv7()
	// Version nibble is at position 14 (the char after the second hyphen)
	if id[14] != '7' {
		t.Errorf("UUIDv7() version nibble = %c, want '7'", id[14])
	}
}

func TestUUIDv7Variant(t *testing.T) {
	id := UUIDv7()
	// Variant nibble is at position 19 (the char after the third hyphen)
	c := id[19]
	if c != '8' && c != '9' && c != 'a' && c != 'b' {
		t.Errorf("UUIDv7() variant nibble = %c, want one of 8/9/a/b", c)
	}
}

func TestUUIDv7Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := UUIDv7()
		if seen[id] {
			t.Fatalf("UUIDv7() produced duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestUUIDv7TimeOrdering(t *testing.T) {
	id1 := UUIDv7()
	time.Sleep(2 * time.Millisecond)
	id2 := UUIDv7()
	// The first 8 hex chars (most significant timestamp bits) should be ordered
	// Compare the full timestamp portion (first 12 hex chars, excluding hyphens)
	ts1 := strings.ReplaceAll(id1[:18], "-", "") // first 13 hex chars = 48-bit timestamp
	ts2 := strings.ReplaceAll(id2[:18], "-", "")
	if ts1 >= ts2 {
		t.Errorf("UUIDv7() not time-ordered: %s >= %s (from %s, %s)", ts1, ts2, id1, id2)
	}
}

func TestUUIDv7Length(t *testing.T) {
	id := UUIDv7()
	if len(id) != 36 {
		t.Errorf("UUIDv7() length = %d, want 36", len(id))
	}
}

// --- toStringSlice ---

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want []string
	}{
		{"single string", "hello", []string{"hello"}},
		{"string slice", []any{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"mixed slice skips non-strings", []any{"a", 42, "b"}, []string{"a", "b"}},
		{"empty slice", []any{}, []string{}},
		{"nil", nil, nil},
		{"int", 42, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStringSlice(tt.in)
			if tt.want == nil {
				if got != nil {
					t.Errorf("toStringSlice(%v) = %v, want nil", tt.in, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("toStringSlice(%v) = %v, want %v", tt.in, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("toStringSlice(%v)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- Error.Error() ---

func TestErrorInterface(t *testing.T) {
	e := &Error{Message: "DOMAIN ERROR", EN: 11, DM: []string{"DOMAIN ERROR", "fn[1] 1÷0"}}
	if e.Error() != "DOMAIN ERROR" {
		t.Errorf("Error.Error() = %q, want %q", e.Error(), "DOMAIN ERROR")
	}
	// Verify it satisfies the error interface
	var _ error = e
}

// --- Mock server for Eval/EvalRaw tests ---

// mockServer creates a TCP listener that responds to prepl requests.
// The handler receives each line and returns a response line.
func mockServer(t *testing.T, handler func(line string) string) (addr string, close func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			resp := handler(line)
			fmt.Fprintf(conn, "%s\n", resp)
		}
	}()
	return ln.Addr().String(), func() {
		ln.Close()
		wg.Wait()
	}
}

// --- Eval with mock ---

func TestEvalReturnValue(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'ret' ⋄ val: 1 2 3)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	resp, err := c.Eval("⍳3")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if resp.Tag != "ret" {
		t.Errorf("Tag = %q, want %q", resp.Tag, "ret")
	}
	if resp.Val == nil {
		t.Fatal("Val is nil, want []any{1, 2, 3}")
	}
	if resp.Err != nil {
		t.Errorf("Err = %v, want nil", resp.Err)
	}
}

func TestEvalReturnVoid(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'ret')"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	resp, err := c.Eval("x←5")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if resp.Tag != "ret" {
		t.Errorf("Tag = %q, want %q", resp.Tag, "ret")
	}
	if resp.Val != nil {
		t.Errorf("Val = %v, want nil", resp.Val)
	}
	if resp.Raw != "" {
		t.Errorf("Raw = %q, want empty", resp.Raw)
	}
}

func TestEvalError(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'err' ⋄ en: 11 ⋄ message: 'DOMAIN ERROR' ⋄ dm: ('DOMAIN ERROR' '1÷0'))"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	resp, err := c.Eval("1÷0")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if resp.Tag != "err" {
		t.Errorf("Tag = %q, want %q", resp.Tag, "err")
	}
	if resp.Err == nil {
		t.Fatal("Err is nil")
	}
	if resp.Err.Message != "DOMAIN ERROR" {
		t.Errorf("Err.Message = %q, want %q", resp.Err.Message, "DOMAIN ERROR")
	}
	if resp.Err.EN != 11 {
		t.Errorf("Err.EN = %d, want 11", resp.Err.EN)
	}
	if len(resp.Err.DM) != 2 {
		t.Errorf("Err.DM = %v, want 2 elements", resp.Err.DM)
	}
	// Verify Error implements error interface
	if resp.Err.Error() != "DOMAIN ERROR" {
		t.Errorf("Err.Error() = %q, want %q", resp.Err.Error(), "DOMAIN ERROR")
	}
}

func TestEvalWithID(t *testing.T) {
	var received string
	addr, cleanup := mockServer(t, func(line string) string {
		received = line
		return "(tag: 'ret' ⋄ id: 'abc-123' ⋄ val: 42)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	resp, err := c.Eval("1+1", "abc-123")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	// Check that the ID was appended to the expression
	if !strings.Contains(received, "⍝ID:abc-123") {
		t.Errorf("sent line = %q, want to contain ⍝ID:abc-123", received)
	}
	if resp.ID != "abc-123" {
		t.Errorf("resp.ID = %q, want %q", resp.ID, "abc-123")
	}
}

func TestEvalWithoutID(t *testing.T) {
	var received string
	addr, cleanup := mockServer(t, func(line string) string {
		received = line
		return "(tag: 'ret' ⋄ val: 42)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err = c.Eval("1+1")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if strings.Contains(received, "⍝ID:") {
		t.Errorf("sent line = %q, should not contain ⍝ID:", received)
	}
	if received != "1+1" {
		t.Errorf("sent line = %q, want %q", received, "1+1")
	}
}

func TestEvalEmptyID(t *testing.T) {
	var received string
	addr, cleanup := mockServer(t, func(line string) string {
		received = line
		return "(tag: 'ret' ⋄ val: 42)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err = c.Eval("1+1", "")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	// Empty ID should not append ⍝ID:
	if strings.Contains(received, "⍝ID:") {
		t.Errorf("sent line = %q, should not contain ⍝ID: for empty id", received)
	}
}

func TestEvalUnknownTag(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'bogus')"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err = c.Eval("1+1")
	if err == nil {
		t.Fatal("expected error for unknown tag, got nil")
	}
	if !strings.Contains(err.Error(), "unknown tag") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "unknown tag")
	}
}

func TestEvalMissingTag(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(val: 42)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err = c.Eval("1+1")
	if err == nil {
		t.Fatal("expected error for missing tag, got nil")
	}
	if !strings.Contains(err.Error(), "missing tag") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "missing tag")
	}
}

func TestEvalNotNamespace(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "42"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err = c.Eval("1+1")
	if err == nil {
		t.Fatal("expected error for non-namespace response, got nil")
	}
	if !strings.Contains(err.Error(), "expected namespace") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "expected namespace")
	}
}

func TestEvalParseError(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "((("
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err = c.Eval("1+1")
	if err == nil {
		t.Fatal("expected error for malformed APLAN, got nil")
	}
	if !strings.Contains(err.Error(), "parse APLAN") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "parse APLAN")
	}
}

func TestEvalRetString(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'ret' ⋄ val: 'hello world')"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	resp, err := c.Eval("+/'hello world'")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	s, ok := resp.Val.(string)
	if !ok {
		t.Fatalf("Val type = %T, want string", resp.Val)
	}
	if s != "hello world" {
		t.Errorf("Val = %q, want %q", s, "hello world")
	}
	if resp.Raw == "" {
		t.Error("Raw should be non-empty for ret with val")
	}
}

func TestEvalRetScalar(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'ret' ⋄ val: 42)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	resp, err := c.Eval("6×7")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	v, ok := resp.Val.(int)
	if !ok {
		t.Fatalf("Val type = %T, want int", resp.Val)
	}
	if v != 42 {
		t.Errorf("Val = %d, want 42", v)
	}
}

func TestEvalErrorSingleDM(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'err' ⋄ en: 2 ⋄ message: 'SYNTAX ERROR' ⋄ dm: 'SYNTAX ERROR')"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	resp, err := c.Eval(")")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if resp.Err == nil {
		t.Fatal("Err is nil")
	}
	// dm was a single string, toStringSlice wraps it
	if len(resp.Err.DM) != 1 || resp.Err.DM[0] != "SYNTAX ERROR" {
		t.Errorf("Err.DM = %v, want [\"SYNTAX ERROR\"]", resp.Err.DM)
	}
}

// --- EvalRaw with mock ---

func TestEvalRaw(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'ret' ⋄ val: 1 2 3)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	raw, err := c.EvalRaw("⍳3")
	if err != nil {
		t.Fatalf("EvalRaw: %v", err)
	}
	if raw != "(tag: 'ret' ⋄ val: 1 2 3)" {
		t.Errorf("EvalRaw = %q, want %q", raw, "(tag: 'ret' ⋄ val: 1 2 3)")
	}
}

func TestEvalRawPassesExpressionThrough(t *testing.T) {
	var received string
	addr, cleanup := mockServer(t, func(line string) string {
		received = line
		return "(tag: 'ret')"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	_, err = c.EvalRaw("2+2 ⍝ comment")
	if err != nil {
		t.Fatalf("EvalRaw: %v", err)
	}
	if received != "2+2 ⍝ comment" {
		t.Errorf("server received = %q, want %q", received, "2+2 ⍝ comment")
	}
}

// --- Connect ---

func TestConnectRefused(t *testing.T) {
	// Try connecting to a port that nothing is listening on
	_, err := Connect("127.0.0.1:1")
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
}

// --- Close ---

func TestCloseConnection(t *testing.T) {
	addr, cleanup := mockServer(t, func(line string) string {
		return "(tag: 'ret')"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	// Eval after close should fail
	_, err = c.Eval("1+1")
	if err == nil {
		t.Error("expected error after Close, got nil")
	}
}

// --- Concurrency ---

func TestEvalConcurrent(t *testing.T) {
	// Verify the mutex serialises concurrent calls correctly
	var mu sync.Mutex
	counter := 0
	addr, cleanup := mockServer(t, func(line string) string {
		mu.Lock()
		counter++
		mu.Unlock()
		return "(tag: 'ret' ⋄ val: 1)"
	})
	defer cleanup()

	c, err := Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = c.Eval("1+1")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	mu.Lock()
	if counter != n {
		t.Errorf("server handled %d requests, want %d", counter, n)
	}
	mu.Unlock()
}

// --- Embed ---

func TestSourceEmbed(t *testing.T) {
	if Source == "" {
		t.Fatal("Source embed is empty")
	}
	if !strings.Contains(Source, "Prepl") {
		t.Error("Source does not contain 'Prepl'")
	}
}

// --- Response struct ---

func TestResponseFields(t *testing.T) {
	// Verify the Response struct can hold all expected field combinations
	r := &Response{
		ID:  "test-id",
		Tag: "ret",
		Val: []any{1, 2, 3},
		Raw: "1 2 3",
	}
	if r.ID != "test-id" || r.Tag != "ret" || r.Raw != "1 2 3" {
		t.Errorf("unexpected Response fields: %+v", r)
	}

	r2 := &Response{
		Tag: "err",
		Err: &Error{Message: "VALUE ERROR", EN: 6, DM: []string{"VALUE ERROR", "x"}},
	}
	if r2.Tag != "err" || r2.Err.EN != 6 {
		t.Errorf("unexpected error Response fields: %+v", r2)
	}
}
