package main

import (
	"os/exec"
	"path/filepath"
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
	dyalogPath := filepath.Join("opt", "mdyalog", "20.0", "64", "unicode", "dyalog")
	want := "DYALOG=" + filepath.Dir(dyalogPath)
	env := session.DyalogEnv(dyalogPath)
	found := false
	for _, e := range env {
		if e == want {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected %q in %v", want, env)
	}
}
