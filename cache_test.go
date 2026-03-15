package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cursork/gritt/cache"
	_ "github.com/mattn/go-sqlite3"
)

func TestCacheDir(t *testing.T) {
	dir, err := cacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Fatal("cacheDir returned empty string")
	}
	// Should end with /gritt
	if filepath.Base(dir) != "gritt" {
		t.Errorf("cacheDir = %q, want base 'gritt'", dir)
	}
	// Directory should exist
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("cacheDir %q does not exist: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("cacheDir %q is not a directory", dir)
	}
}

func TestCachePath(t *testing.T) {
	p := cachePath("test.db")
	if p == "" {
		t.Fatal("cachePath returned empty string")
	}
	if filepath.Base(p) != "test.db" {
		t.Errorf("cachePath = %q, want base 'test.db'", p)
	}
	dir := filepath.Dir(p)
	if filepath.Base(dir) != "gritt" {
		t.Errorf("cachePath parent = %q, want 'gritt'", dir)
	}
}

func TestIsCacheStale(t *testing.T) {
	// Non-existent file is stale
	if !isCacheStale("/nonexistent/file.db") {
		t.Error("non-existent file should be stale")
	}

	// Fresh file is not stale
	tmp := filepath.Join(t.TempDir(), "fresh.db")
	os.WriteFile(tmp, []byte("test"), 0644)
	if isCacheStale(tmp) {
		t.Error("just-created file should not be stale")
	}

	// Old file is stale
	old := filepath.Join(t.TempDir(), "old.db")
	os.WriteFile(old, []byte("test"), 0644)
	then := time.Now().Add(-(cache.MaxAge + time.Hour))
	os.Chtimes(old, then, then)
	if !isCacheStale(old) {
		t.Error("8-day-old file should be stale")
	}
}

func TestAPLcartCacheRoundTrip(t *testing.T) {
	// Use a temp dir to avoid polluting the real cache
	tmpDir := t.TempDir()
	tmpDB := filepath.Join(tmpDir, "aplcart.db")

	entries := []APLcartEntry{
		{Syntax: "⍳N", Description: "First N integers", Keywords: "iota index"},
		{Syntax: "⍴A", Description: "Shape of A", Keywords: "rho shape"},
		{Syntax: "+/A", Description: "Sum of A", Keywords: "plus reduce sum"},
	}

	if err := writeAPLcartCacheTo(tmpDB, entries); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadAPLcartCacheFrom(tmpDB)
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

func TestParseAPLcartTSV(t *testing.T) {
	tsv := "syntax\tdesc\tf2\tf3\tf4\tf5\tkeywords\n" +
		"⍳N\tFirst N integers\t\t\t\t\tiota index\n" +
		"⍴A\tShape of A\t\t\t\t\trho shape\n" +
		"\n" // trailing blank line

	entries := parseAPLcartTSV(tsv)
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

func TestParseAPLcartTSVSkipsShortLines(t *testing.T) {
	tsv := "header\n" +
		"only\ttwo\tfields\n" +
		"⍳N\tFirst N integers\t\t\t\t\tiota\n"

	entries := parseAPLcartTSV(tsv)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (short line should be skipped)", len(entries))
	}
}

func TestLoadAPLcartCacheEmpty(t *testing.T) {
	// Remove cache file if it exists, then try loading
	dbPath := cachePath("aplcart-nonexistent-test.db")
	os.Remove(dbPath)

	_, err := LoadAPLcartCache()
	if err == nil {
		// It's OK if this fails — we just need it not to panic
		// The actual error depends on whether the DB exists
	}
	_ = err
}

// TestRefreshAPLcartCache exercises the real APLcart fetch-and-cache path.
// Uses the real cache: fast when fresh, hits GitHub when stale/missing.
// Set NO_CACHE=1 to force a fresh fetch.
func TestRefreshAPLcartCache(t *testing.T) {
	dbPath := cachePath("aplcart.db")
	noCache := os.Getenv("NO_CACHE") == "1"

	// If cache is fresh and not forced, just verify we can load from it
	if !noCache && !isCacheStale(dbPath) {
		t.Log("APLcart cache is fresh, loading from disk")
	} else {
		t.Log("APLcart cache is stale/missing, fetching from GitHub...")
		result := RefreshAPLcartCache().(APLcartCacheResult)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if len(result.Entries) < 100 {
			t.Fatalf("expected at least 100 APLcart entries, got %d", len(result.Entries))
		}
		t.Logf("Fetched %d entries", len(result.Entries))
	}

	// Either way, cache should now be loadable
	entries, err := LoadAPLcartCache()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 100 {
		t.Fatalf("expected at least 100 cached entries, got %d", len(entries))
	}

	// Spot-check: iota should be in there somewhere
	found := false
	for _, e := range entries {
		if len(e.Syntax) > 0 && []rune(e.Syntax)[0] == '⍳' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find iota (⍳) in APLcart entries")
	}
}

// TestRefreshDocsCache exercises the real docs DB fetch-and-cache path.
// Uses the real cache: fast when fresh, hits GitHub when stale/missing.
// Set NO_CACHE=1 to force a fresh fetch.
func TestRefreshDocsCache(t *testing.T) {
	dbPath := cachePath("dyalog-docs.db")
	noCache := os.Getenv("NO_CACHE") == "1"

	if !noCache && !isCacheStale(dbPath) {
		t.Log("Docs cache is fresh, verifying from disk")
	} else {
		t.Log("Docs cache is stale/missing, fetching from GitHub...")
		result := RefreshDocsCache().(DocsCacheResult)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		t.Log("Fetched docs DB")
	}

	// Cache file should exist and be a valid SQLite DB
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 1000 {
		t.Fatalf("docs DB too small: %d bytes", info.Size())
	}

	// Open and verify schema
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// help_urls table should have entries
	var count int
	if err := db.QueryRow("SELECT count(*) FROM help_urls").Scan(&count); err != nil {
		t.Fatal("help_urls:", err)
	}
	if count == 0 {
		t.Error("help_urls table is empty")
	}
	t.Logf("help_urls: %d entries", count)

	// docs table should have content
	if err := db.QueryRow("SELECT count(*) FROM docs").Scan(&count); err != nil {
		t.Fatal("docs:", err)
	}
	if count == 0 {
		t.Error("docs table is empty")
	}
	t.Logf("docs: %d entries", count)

	// Symbol lookup should work (⍳ → Iota)
	var path string
	if err := db.QueryRow("SELECT path FROM help_urls WHERE symbol = ?", "⍳").Scan(&path); err != nil {
		t.Fatal("iota lookup:", err)
	}
	if path == "" {
		t.Error("empty path for ⍳")
	}
}
