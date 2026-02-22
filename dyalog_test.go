package main

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestFindDyalog(t *testing.T) {
	exe := findDyalog("")
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		// Should find something on dev machines where Dyalog is installed
		if exe == "" {
			t.Skip("No Dyalog installation found (not installed?)")
		}
		t.Logf("Found: %s", exe)
	}
}

func TestFindDyalogVersion(t *testing.T) {
	// Ask for a version that doesn't exist
	exe := findDyalog("99.99")
	if exe != "" {
		t.Errorf("Expected empty for nonexistent version, got %s", exe)
	}
}

func TestResolveDyalog(t *testing.T) {
	// If dyalog is in PATH, resolveDyalog should find it
	if _, err := exec.LookPath("dyalog"); err == nil {
		exe := resolveDyalog("")
		if exe == "" {
			t.Error("Expected to find dyalog via PATH")
		}
		t.Logf("PATH resolved: %s", exe)
	}
}

func TestDyalogEnv(t *testing.T) {
	env := dyalogEnv("/opt/mdyalog/20.0/64/unicode/dyalog")
	found := false
	for _, e := range env {
		if e == "DYALOG=/opt/mdyalog/20.0/64/unicode" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected DYALOG env var, got %v", env)
	}
}
