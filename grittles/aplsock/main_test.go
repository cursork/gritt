package main

import (
	"os/exec"
	"testing"
)

func TestBuild(t *testing.T) {
	// Verify the binary builds without errors
	out, err := exec.Command("go", "build", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
}

func TestHelpFlag(t *testing.T) {
	out, err := exec.Command("go", "run", ".", "-help").CombinedOutput()
	// -help exits 2 (flag usage) but should print usage text
	_ = err
	s := string(out)
	if len(s) == 0 {
		t.Fatal("no output from -help")
	}
}
