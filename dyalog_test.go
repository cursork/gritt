package main

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/cursork/gritt/session"
)

func TestFindDyalog(t *testing.T) {
	exe, err := session.FindDyalog("")
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if err != nil {
			t.Skip("No Dyalog installation found (not installed?)")
		}
		t.Logf("Found: %s", exe)
	}
}

func TestFindDyalogVersion(t *testing.T) {
	_, err := session.FindDyalog("99.99")
	if err == nil {
		t.Error("Expected error for nonexistent version")
	}
}

func TestResolveDyalog(t *testing.T) {
	if _, err := exec.LookPath("dyalog"); err == nil {
		exe := resolveDyalog("")
		if exe == "" {
			t.Error("Expected to find dyalog via PATH")
		}
		t.Logf("PATH resolved: %s", exe)
	}
}

func TestDyalogEnv(t *testing.T) {
	env := session.DyalogEnv("/opt/mdyalog/20.0/64/unicode/dyalog")
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
