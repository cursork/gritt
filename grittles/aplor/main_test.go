package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// --- parseSignedInts tests ---

func TestParseSignedInts_Valid(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []byte
	}{
		{"positive", "0 1 42 127", []byte{0, 1, 42, 127}},
		{"negative", "-1 -5 -128", []byte{0xFF, 0xFB, 0x80}},
		{"mixed", "0 127 -128 -1 42", []byte{0, 127, 0x80, 0xFF, 42}},
		{"single", "42", []byte{42}},
		{"high_neg", "-128", []byte{0x80}},
		{"high_pos", "127", []byte{127}},
		{"extra_spaces", "  1   2   3  ", []byte{1, 2, 3}},
		{"tabs_and_newlines", "1\t2\n3", []byte{1, 2, 3}},
		{"multiline", "1 2\n3 4\n5 6", []byte{1, 2, 3, 4, 5, 6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSignedInts(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("length: got %d, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseSignedInts_HighBar(t *testing.T) {
	// APL uses ¯ (U+00AF) for negative sign
	cases := []struct {
		name  string
		input string
		want  []byte
	}{
		{"single_neg", "¯1", []byte{0xFF}},
		{"mixed", "¯33 ¯92 4 0 0 0", []byte{0xDF, 0xA4, 4, 0, 0, 0}},
		{"all_neg", "¯128 ¯1 ¯5", []byte{0x80, 0xFF, 0xFB}},
		// Real amicable magic bytes: ¯33 ¯92 is 0xDF 0xA4 (64-bit magic)
		{"magic_header", "¯33 ¯92 4 0 0 0 0 0 0 0 15 34 0 0 0 0 0 0 42 0 0 0 0 0 0 0",
			[]byte{0xDF, 0xA4, 4, 0, 0, 0, 0, 0, 0, 0, 15, 34, 0, 0, 0, 0, 0, 0, 42, 0, 0, 0, 0, 0, 0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSignedInts(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("length: got %d, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseSignedInts_Empty(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace_only", "   \t\n  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSignedInts(tc.input)
			if err == nil {
				t.Fatal("expected error for empty input")
			}
			if !strings.Contains(err.Error(), "empty") {
				t.Errorf("error should mention 'empty', got: %v", err)
			}
		})
	}
}

func TestParseSignedInts_OutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"too_high", "128"},
		{"too_low", "-129"},
		{"way_too_high", "1000"},
		{"way_too_low", "-1000"},
		{"mixed_with_bad", "0 1 200 3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSignedInts(tc.input)
			if err == nil {
				t.Fatal("expected error for out-of-range value")
			}
			if !strings.Contains(err.Error(), "out of range") {
				t.Errorf("error should mention 'out of range', got: %v", err)
			}
		})
	}
}

func TestParseSignedInts_InvalidTokens(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"letters", "abc"},
		{"mixed_bad", "1 2 abc 4"},
		{"float", "1.5"},
		{"hex", "0xFF"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSignedInts(tc.input)
			if err == nil {
				t.Fatal("expected error for invalid token")
			}
		})
	}
}

func TestParseSignedInts_BoundaryValues(t *testing.T) {
	// Verify exact boundary: -128 and 127 are OK, -129 and 128 are not.
	got, err := parseSignedInts("-128 127")
	if err != nil {
		t.Fatalf("boundary values should succeed: %v", err)
	}
	if got[0] != 0x80 || got[1] != 0x7F {
		t.Errorf("got [0x%02X, 0x%02X], want [0x80, 0x7F]", got[0], got[1])
	}

	if _, err := parseSignedInts("-129"); err == nil {
		t.Error("-129 should fail")
	}
	if _, err := parseSignedInts("128"); err == nil {
		t.Error("128 should fail")
	}
}

func TestParseSignedInts_SignedToUnsignedConversion(t *testing.T) {
	// Verify the signed-to-unsigned conversion matches amicable's signedToUnsigned.
	// These are real 220⌶ headers from amicable_test.go test vectors.
	cases := []struct {
		name   string
		signed []int // signed APL values
		want   []byte
	}{
		// Scalar int 42: magic + type + value
		{"scalar_42", []int{-33, -92, 4, 0, 0, 0, 0, 0, 0, 0, 15, 34, 0, 0, 0, 0, 0, 0, 42, 0, 0, 0, 0, 0, 0, 0},
			[]byte{0xDF, 0xA4, 0x04, 0, 0, 0, 0, 0, 0, 0, 0x0F, 0x22, 0, 0, 0, 0, 0, 0, 42, 0, 0, 0, 0, 0, 0, 0}},
		// 'hello': char vector
		{"hello", []int{-33, -92, 5, 0, 0, 0, 0, 0, 0, 0, 31, 39, 0, 0, 0, 0, 0, 0, 5, 0, 0, 0, 0, 0, 0, 0, 104, 101, 108, 108, 111, 0, 0, 0},
			[]byte{0xDF, 0xA4, 0x05, 0, 0, 0, 0, 0, 0, 0, 0x1F, 0x27, 0, 0, 0, 0, 0, 0, 5, 0, 0, 0, 0, 0, 0, 0, 'h', 'e', 'l', 'l', 'o', 0, 0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Build the input string with spaces
			parts := make([]string, len(tc.signed))
			for i, v := range tc.signed {
				parts[i] = fmt.Sprintf("%d", v)
			}
			input := strings.Join(parts, " ")

			got, err := parseSignedInts(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("length: got %d, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// --- CLI tests ---

// buildAplor builds the aplor binary into a temp directory and returns the path.
func buildAplor(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "aplor")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(projectRoot(), "grittles", "aplor")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build aplor: %v\n%s", err, out)
	}
	return bin
}

// runnerPrefix returns the tokens from GRITT_TEST_RUNNER (split on spaces)
// to prepend to a test subprocess command line. Empty/unset → no prefix,
// launch directly.
func runnerPrefix() []string {
	v := strings.TrimSpace(os.Getenv("GRITT_TEST_RUNNER"))
	if v == "" {
		return nil
	}
	return strings.Fields(v)
}

func projectRoot() string {
	// Walk up from this test file to find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func TestCLI_Help(t *testing.T) {
	bin := buildAplor(t)

	cmd := exec.Command(bin, "-help")
	out, err := cmd.CombinedOutput()
	// -help exits 0
	if err != nil {
		t.Fatalf("aplor -help failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Dyalog 220⌶") {
		t.Errorf("help output missing expected text:\n%s", out)
	}
}

func TestCLI_UnknownFlag(t *testing.T) {
	bin := buildAplor(t)

	cmd := exec.Command(bin, "-bogus")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected nonzero exit for unknown flag")
	}
	if !strings.Contains(string(out), "unknown flag") {
		t.Errorf("expected 'unknown flag' error, got:\n%s", out)
	}
}

func TestCLI_EmptyStdin(t *testing.T) {
	bin := buildAplor(t)

	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected nonzero exit for empty stdin")
	}
	if !strings.Contains(string(out), "empty") {
		t.Errorf("expected 'empty' error, got:\n%s", out)
	}
}

func TestCLI_InvalidInput(t *testing.T) {
	bin := buildAplor(t)

	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("not numbers at all")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected nonzero exit for invalid input")
	}
	if !strings.Contains(string(out), "parse:") {
		t.Errorf("expected 'parse:' error, got:\n%s", out)
	}
}

func TestCLI_PlainArrayScalar(t *testing.T) {
	bin := buildAplor(t)

	// Feed it a valid 220⌶ serialized scalar (42). It's not a ⎕OR function
	// blob, so aplor falls through to codec.Serialize and recovers "42".
	// From amicable_test.go TestUnmarshalScalarInt8.
	input := "-33 -92 4 0 0 0 0 0 0 0 15 34 0 0 0 0 0 0 42 0 0 0 0 0 0 0"
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Errorf("got %q, want '42'", out)
	}
}

func TestCLI_PlainArrayHighBar(t *testing.T) {
	bin := buildAplor(t)

	// Same scalar 42 but using APL high-bar notation (¯ for negative).
	input := "¯33 ¯92 4 0 0 0 0 0 0 0 15 34 0 0 0 0 0 0 42 0 0 0 0 0 0 0"
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Errorf("got %q, want '42'", out)
	}
}

func TestCLI_OutOfRangeInput(t *testing.T) {
	bin := buildAplor(t)

	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("200 0 0")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected nonzero exit for out-of-range value")
	}
	if !strings.Contains(string(out), "out of range") {
		t.Errorf("expected 'out of range' error, got:\n%s", out)
	}
}

func TestCLI_FileInput(t *testing.T) {
	bin := buildAplor(t)

	// Write a signed-int file with scalar 42 and read it back out via
	// the FILE argument path. Plain arrays are recovered via APLAN.
	tmp := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmp, []byte("-33 -92 4 0 0 0 0 0 0 0 15 34 0 0 0 0 0 0 42 0 0 0 0 0 0 0"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Errorf("got %q, want '42'", out)
	}
}

func TestCLI_RawBinaryInput(t *testing.T) {
	bin := buildAplor(t)

	// Write raw binary bytes for scalar 42 and decode via -raw.
	rawBytes := []byte{0xDF, 0xA4, 0x04, 0, 0, 0, 0, 0, 0, 0, 0x0F, 0x22, 0, 0, 0, 0, 0, 0, 42, 0, 0, 0, 0, 0, 0, 0}
	tmp := filepath.Join(t.TempDir(), "test.220")
	if err := os.WriteFile(tmp, rawBytes, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-raw", tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Errorf("got %q, want '42'", out)
	}
}

func TestCLI_RawStdin(t *testing.T) {
	bin := buildAplor(t)

	// Pipe raw binary via stdin with -raw flag.
	rawBytes := []byte{0xDF, 0xA4, 0x04, 0, 0, 0, 0, 0, 0, 0, 0x0F, 0x22, 0, 0, 0, 0, 0, 0, 42, 0, 0, 0, 0, 0, 0, 0}
	cmd := exec.Command(bin, "-raw")
	cmd.Stdin = strings.NewReader(string(rawBytes))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Errorf("got %q, want '42'", out)
	}
}

func TestCLI_MissingFile(t *testing.T) {
	bin := buildAplor(t)

	cmd := exec.Command(bin, "/nonexistent/path/to/file.txt")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected nonzero exit for missing file")
	}
	if !strings.Contains(string(out), "read:") {
		t.Errorf("expected 'read:' error, got:\n%s", out)
	}
}

func TestCLI_TruncatedInput(t *testing.T) {
	bin := buildAplor(t)

	// Too short to even have a valid magic header.
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("-33")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected nonzero exit for truncated input")
	}
	if !strings.Contains(string(out), "unmarshal:") {
		t.Errorf("expected 'unmarshal:' error, got:\n%s", out)
	}
}

// TestCLI_DecompileDfn tests the full happy path: serialize a dfn in Dyalog,
// pipe the signed ints to aplor, and verify the decompiled source.
func TestCLI_DecompileDfn(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}
	bin := buildAplor(t)

	cases := []struct {
		name string
		expr string
	}{
		{"add1", "{⍵+1}"},
		{"dyadic", "{⍺+⍵}"},
		{"guard", "{0=⍵:0 ⋄ ⍵}"},
		{"multi", "{r←⍵+1 ⋄ r}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Serialize the dfn in Dyalog via gritt.
			gritt := exec.Command("gritt", "-l",
				"-e", "OR←{f←⍺⍺⋄⎕OR'f'}",
				"-e", fmt.Sprintf("1(220⌶)(%s)OR ⍬", tc.expr),
			)
			serOut, err := gritt.CombinedOutput()
			if err != nil {
				t.Fatalf("gritt serialize: %v\n%s", err, serOut)
			}

			// Step 2: Pipe the signed ints to aplor.
			aplor := exec.Command(bin)
			aplor.Stdin = strings.NewReader(string(serOut))
			decompiled, err := aplor.CombinedOutput()
			if err != nil {
				t.Fatalf("aplor failed: %v\n%s", err, decompiled)
			}

			got := strings.TrimSpace(string(decompiled))
			if got != tc.expr {
				t.Errorf("want: %s\n got: %s", tc.expr, got)
			}
		})
	}
}

// TestCLI_DecompileTradfn tests tradfn decompilation through the CLI.
func TestCLI_DecompileTradfn(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}
	bin := buildAplor(t)

	fix := []string{"r←add x", "r←x+1"}
	parts := make([]string, len(fix))
	for i, l := range fix {
		parts[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(l, "'", "''"))
	}
	fixExpr := "sink←⎕FX " + strings.Join(parts, " ")

	gritt := exec.Command("gritt", "-l",
		"-e", fixExpr,
		"-e", "1(220⌶)⎕OR'add'",
	)
	serOut, err := gritt.CombinedOutput()
	if err != nil {
		t.Fatalf("gritt serialize: %v\n%s", err, serOut)
	}

	aplor := exec.Command(bin)
	aplor.Stdin = strings.NewReader(string(serOut))
	decompiled, err := aplor.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor failed: %v\n%s", err, decompiled)
	}

	got := strings.TrimSpace(string(decompiled))
	want := "r←add x\nr←x+1"
	if got != want {
		t.Errorf("want: %q\n got: %q", want, got)
	}
}

// TestCLI_DecompileRaw tests the -raw flag with actual binary ⎕OR data.
func TestCLI_DecompileRaw(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}
	bin := buildAplor(t)

	// Get the signed ints from Dyalog.
	gritt := exec.Command("gritt", "-l",
		"-e", "OR←{f←⍺⍺⋄⎕OR'f'}",
		"-e", "1(220⌶)({⍵+1})OR ⍬",
	)
	serOut, err := gritt.CombinedOutput()
	if err != nil {
		t.Fatalf("gritt serialize: %v\n%s", err, serOut)
	}

	// Convert signed ints to raw bytes.
	rawBytes, err := parseSignedInts(string(serOut))
	if err != nil {
		t.Fatalf("parseSignedInts: %v", err)
	}

	// Write raw bytes to a temp file.
	tmp := filepath.Join(t.TempDir(), "test.220")
	if err := os.WriteFile(tmp, rawBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// Run aplor with -raw.
	aplor := exec.Command(bin, "-raw", tmp)
	decompiled, err := aplor.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor -raw failed: %v\n%s", err, decompiled)
	}

	got := strings.TrimSpace(string(decompiled))
	if got != "{⍵+1}" {
		t.Errorf("want: {⍵+1}\n got: %s", got)
	}
}

// TestCLI_AplsockAplorPipe drives the full `printf | nc aplsock:aplor | aplor
// -stream` pipeline: each expression goes over the socket as APL, aplsock
// returns 220⌶-encoded namespaces as signed ints, and the aplor binary
// recovers each value back into APLAN.
func TestCLI_AplsockAplorPipe(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}
	aplorBin := buildAplor(t)

	// Build aplsock into a temp dir so we don't step on any installed binary.
	aplsockBin := filepath.Join(t.TempDir(), "aplsock")
	build := exec.Command("go", "build", "-o", aplsockBin, ".")
	build.Dir = filepath.Join(projectRoot(), "grittles", "aplsock")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build aplsock: %v\n%s", err, out)
	}

	port := 14700 + os.Getpid()%300
	sock := fmt.Sprintf(":%d", port)
	// GRITT_TEST_RUNNER, if set, is prepended to the command line. It lets
	// the user wrap aplsock with a supervisor (e.g. a SIGTERM→SIGKILL
	// escalator) so the Dyalog child is reliably reaped on abnormal test
	// exits. Unset = launch aplsock directly.
	aplsockArgs := append(runnerPrefix(), aplsockBin, "-l", "-sock", sock, "-mode", "aplor")
	aplsock := exec.Command(aplsockArgs[0], aplsockArgs[1:]...)
	aplsock.Stderr = os.Stderr
	if err := aplsock.Start(); err != nil {
		t.Fatalf("start aplsock: %v", err)
	}
	defer func() {
		if aplsock.Process != nil {
			_ = aplsock.Process.Signal(syscall.SIGTERM)
		}
		done := make(chan error, 1)
		go func() { done <- aplsock.Wait() }()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			_ = aplsock.Process.Kill()
			<-done
		}
	}()

	// Wait for the socket to accept.
	addr := fmt.Sprintf("localhost:%d", port)
	var conn net.Conn
	var dialErr error
	for i := 0; i < 150; i++ {
		conn, dialErr = net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if dialErr == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if dialErr != nil {
		t.Fatalf("dial aplsock %s: %v", addr, dialErr)
	}
	defer conn.Close()

	// Send the demo batch in one go: printf '%s\n' '⍳5' "'hello world'" '2 3⍴⍳6' '○1' '1÷0'
	exprs := []struct {
		expr string
		want string
	}{
		{"⍳5", "(val: 1 2 3 4 5 ⋄ tag: 'ret')"},
		{"'hello world'", "(val: 'hello world' ⋄ tag: 'ret')"},
		{"2 3⍴⍳6", "(val: [1 2 3 ⋄ 4 5 6] ⋄ tag: 'ret')"},
		{"○1", "(val: 3.141592653589793 ⋄ tag: 'ret')"},
		{"1÷0", "'Divide by zero'"},
	}
	var payload strings.Builder
	for _, e := range exprs {
		payload.WriteString(e.expr)
		payload.WriteByte('\n')
	}
	if _, err := conn.Write([]byte(payload.String())); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read one response line per expression. Namespace blobs carry a full
	// atoms table, so lines are large.
	reader := bufio.NewReaderSize(conn, 1<<20)
	var piped strings.Builder
	for i := range exprs {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response %d: %v", i, err)
		}
		piped.WriteString(line)
	}

	// Pipe all five blobs through `aplor -stream` in one invocation.
	aplor := exec.Command(aplorBin, "-stream")
	aplor.Stdin = strings.NewReader(piped.String())
	out, err := aplor.CombinedOutput()
	if err != nil {
		t.Fatalf("aplor -stream failed: %v\n%s", err, out)
	}

	recovered := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(recovered) != len(exprs) {
		t.Fatalf("got %d recovered lines, want %d\n--- output ---\n%s", len(recovered), len(exprs), out)
	}
	for i, e := range exprs {
		if !strings.Contains(recovered[i], e.want) {
			t.Errorf("expr %q:\n  want contains: %s\n  got:           %s", e.expr, e.want, recovered[i])
		}
	}
}
