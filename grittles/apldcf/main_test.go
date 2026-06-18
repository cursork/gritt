package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureDir = "../../dcf/testdata"

func runApldcf(t *testing.T, args ...string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "apldcf")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stderr = &bytes.Buffer{}
	if err := build.Run(); err != nil {
		t.Fatalf("build: %v\n%s", err, build.Stderr.(*bytes.Buffer).String())
	}
	cmd := exec.Command(bin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	return out.String()
}

func TestListThreeComponents(t *testing.T) {
	out := runApldcf(t, filepath.Join(fixtureDir, "three_components.dcf"))
	for _, want := range []string{"components: 3", "80", "83"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestDumpCharComponent(t *testing.T) {
	out := runApldcf(t, "-n", "1", filepath.Join(fixtureDir, "one_char.dcf"))
	if !strings.Contains(out, "'hello'") {
		t.Errorf("want 'hello' in output, got:\n%s", out)
	}
}

func TestDumpAll(t *testing.T) {
	out := runApldcf(t, "-all", filepath.Join(fixtureDir, "three_components.dcf"))
	for _, want := range []string{"--- component 1 ---", "'first'", "--- component 3 ---"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}
