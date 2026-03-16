package aplcart

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := filepath.Join(tmpDir, "aplcart.db")

	entries := []Entry{
		{Syntax: "⍳N", Description: "First N integers", Keywords: "iota index"},
		{Syntax: "⍴A", Description: "Shape of A", Keywords: "rho shape"},
		{Syntax: "+/A", Description: "Sum of A", Keywords: "plus reduce sum"},
	}

	if err := writeCacheTo(tmpDB, entries); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadFrom(tmpDB)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != len(entries) {
		t.Fatalf("got %d entries, want %d", len(loaded), len(entries))
	}
	for i, e := range entries {
		if loaded[i].Syntax != e.Syntax {
			t.Errorf("[%d] Syntax = %q, want %q", i, loaded[i].Syntax, e.Syntax)
		}
		if loaded[i].Description != e.Description {
			t.Errorf("[%d] Description = %q, want %q", i, loaded[i].Description, e.Description)
		}
		if loaded[i].Keywords != e.Keywords {
			t.Errorf("[%d] Keywords = %q, want %q", i, loaded[i].Keywords, e.Keywords)
		}
	}
}

func TestParseTSV(t *testing.T) {
	tsv := "syntax\tdesc\tf2\tf3\tf4\tf5\tkeywords\n" +
		"⍳N\tFirst N integers\t\t\t\t\tiota index\n" +
		"⍴A\tShape of A\t\t\t\t\trho shape\n" +
		"\n" // trailing blank line

	entries := parseTSV(tsv)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Syntax != "⍳N" {
		t.Errorf("[0] Syntax = %q, want %q", entries[0].Syntax, "⍳N")
	}
	if entries[0].Keywords != "iota index" {
		t.Errorf("[0] Keywords = %q, want %q", entries[0].Keywords, "iota index")
	}
	if entries[1].Description != "Shape of A" {
		t.Errorf("[1] Description = %q, want %q", entries[1].Description, "Shape of A")
	}
}

func TestParseTSVSkipsShortLines(t *testing.T) {
	tsv := "header\n" +
		"only\ttwo\tfields\n" +
		"⍳N\tFirst N integers\t\t\t\t\tiota\n"

	entries := parseTSV(tsv)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (short line should be skipped)", len(entries))
	}
}

func TestLoadCacheEmpty(t *testing.T) {
	tmpDB := filepath.Join(t.TempDir(), "nonexistent.db")
	os.Remove(tmpDB)

	_, err := loadFrom(tmpDB)
	if err == nil {
		// It's OK if this fails — we just need it not to panic
	}
	_ = err
}

func TestSearch(t *testing.T) {
	entries := []Entry{
		{Syntax: "⍳N", Description: "First N integers", Keywords: "iota index"},
		{Syntax: "⍴A", Description: "Shape of A", Keywords: "rho shape"},
		{Syntax: "+/A", Description: "Sum of A", Keywords: "plus reduce sum"},
	}

	// Empty query returns all
	results := Search(entries, "")
	if len(results) != 3 {
		t.Fatalf("empty query: got %d, want 3", len(results))
	}

	// Description match
	results = Search(entries, "shape")
	if len(results) != 1 || results[0].Syntax != "⍴A" {
		t.Errorf("'shape' search: got %v", results)
	}

	// Syntax match sorts first
	results = Search(entries, "A")
	if len(results) < 2 {
		t.Fatalf("'A' search: got %d results, want at least 2", len(results))
	}
	// ⍴A and +/A have syntax matches, ⍳N doesn't
	for _, r := range results[:2] {
		if r.Syntax != "⍴A" && r.Syntax != "+/A" {
			t.Errorf("expected syntax-matching entries first, got %q", r.Syntax)
		}
	}
}
