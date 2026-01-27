package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRenderMarkdown(t *testing.T) {
	md := "# Hello\n\nThis is a test paragraph.\n"
	out := RenderMarkdown(md, 60)
	if out == "" {
		t.Fatal("RenderMarkdown returned empty string")
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("rendered output missing title: %q", out)
	}
	if !strings.Contains(out, "test paragraph") {
		t.Errorf("rendered output missing body: %q", out)
	}
}

func TestNewDocPane(t *testing.T) {
	rendered := RenderMarkdown("# Test\n\nLine 1\n\nLine 2\n", 40)
	dp := NewDocPane("Test / Page", "test.md", rendered, nil, nil, 40)

	if dp.Title() != "Test / Page" {
		t.Errorf("Title() = %q, want %q", dp.Title(), "Test / Page")
	}
	if len(dp.lines) == 0 {
		t.Fatal("DocPane has no lines")
	}
}

func TestDocPaneScroll(t *testing.T) {
	// Create content with many lines
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("Line\n\n")
	}
	rendered := RenderMarkdown(sb.String(), 40)
	dp := NewDocPane("Scroll Test", "test.md", rendered, nil, nil, 40)

	// Initial scroll is 0
	if dp.scroll != 0 {
		t.Errorf("initial scroll = %d, want 0", dp.scroll)
	}

	dp.scrollDown(5)
	if dp.scroll != 5 {
		t.Errorf("after scrollDown(5): scroll = %d, want 5", dp.scroll)
	}

	dp.scrollUp(3)
	if dp.scroll != 2 {
		t.Errorf("after scrollUp(3): scroll = %d, want 2", dp.scroll)
	}

	// Can't scroll above 0
	dp.scrollUp(100)
	if dp.scroll != 0 {
		t.Errorf("after scrollUp(100): scroll = %d, want 0", dp.scroll)
	}
}

func TestDocPaneRender(t *testing.T) {
	rendered := RenderMarkdown("# Title\n\nContent here.\n", 40)
	dp := NewDocPane("Test", "test.md", rendered, nil, nil, 40)

	out := dp.Render(40, 20)
	if out == "" {
		t.Fatal("Render returned empty string")
	}
	// Output should not contain broken ANSI (no bare ESC without proper sequence)
	// Basic check: output should be renderable without errors
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatal("Render produced no lines")
	}
}

func TestDocsDatabase(t *testing.T) {
	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "gritt", "dyalog-docs.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skip("docs database not found at", dbPath)
	}

	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Test help_urls table exists and has entries
	var count int
	err = db.QueryRow("SELECT count(*) FROM help_urls").Scan(&count)
	if err != nil {
		t.Fatal("help_urls table:", err)
	}
	if count == 0 {
		t.Error("help_urls table is empty")
	}

	// Test symbol lookup for key APL primitives
	symbols := []struct {
		sym  string
		want string // substring of expected nav path
	}{
		{"⍳", "Iota"},
		{"⍴", "Rho"},
		{"+", "Plus"},
		{"←", "Left Arrow"},
		{"⌈", "Upstile"},
	}

	for _, s := range symbols {
		var path string
		err := db.QueryRow("SELECT path FROM help_urls WHERE symbol = ?", s.sym).Scan(&path)
		if err != nil {
			t.Errorf("symbol %q: %v", s.sym, err)
			continue
		}
		if !strings.Contains(path, s.want) {
			t.Errorf("symbol %q: path %q doesn't contain %q", s.sym, path, s.want)
		}

		// Verify the path maps to actual content
		var content string
		err = db.QueryRow("SELECT content FROM docs WHERE path = ?", path).Scan(&content)
		if err != nil {
			t.Errorf("symbol %q: content lookup for %q: %v", s.sym, path, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("symbol %q: empty content for %q", s.sym, path)
		}
	}

}

func TestProcessLinks(t *testing.T) {
	md := "See [Transpose](../primitive-functions/transpose.md) and [external](https://example.com).\n"
	processed, links := processLinks(md, "language-reference-guide/docs/symbols/circle-backslash.md")

	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
	if links[0].display != "Transpose" {
		t.Errorf("link display = %q, want %q", links[0].display, "Transpose")
	}
	if links[0].file != "language-reference-guide/docs/primitive-functions/transpose.md" {
		t.Errorf("link file = %q, want %q", links[0].file, "language-reference-guide/docs/primitive-functions/transpose.md")
	}
	if !strings.Contains(processed, "«Transpose»") {
		t.Errorf("processed missing marker: %q", processed)
	}
	// External link should be preserved
	if !strings.Contains(processed, "https://example.com") {
		t.Errorf("external link removed: %q", processed)
	}
}

func TestSymbolAtCursor(t *testing.T) {
	// Create a minimal model with a line containing APL symbols
	m := Model{
		lines: []Line{{Text: "      x←⍳10"}},
	}

	tests := []struct {
		col  int
		want string
	}{
		{0, ""},       // before content
		{7, ""},       // on 'x' (ASCII letter)
		{8, "←"},      // on ←
		{9, "⍳"},      // on ⍳
		{10, ""},      // on '1' (digit)
	}

	for _, tt := range tests {
		m.cursorCol = tt.col
		got := m.symbolAtCursor()
		if got != tt.want {
			t.Errorf("col=%d: symbolAtCursor()=%q, want %q", tt.col, got, tt.want)
		}
	}
}
