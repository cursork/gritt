// bundle-docs clones the Dyalog documentation repo, parses its mkdocs
// monorepo structure, and produces a sqlite3 database of all markdown
// content keyed by navigation path.
//
//	go build ./cmd/bundle-docs
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

// mkdocsConfig represents the parts of mkdocs.yml we care about.
type mkdocsConfig struct {
	SiteName string     `yaml:"site_name"`
	DocsDir  string     `yaml:"docs_dir"`
	Nav      []yaml.Node `yaml:"nav"`
}

func main() {
	output := flag.String("o", "dyalog-docs.db", "output database path")
	repo := flag.String("repo", "git@github.com:Dyalog/documentation.git", "documentation repo URL")
	helpURLs := flag.String("help-urls", "help_urls.h", "path to help_urls.h")
	keep := flag.Bool("keep", false, "keep cloned repo (print path)")
	flag.Parse()

	// Clone repo
	tmpDir, err := os.MkdirTemp("", "dyalog-docs-*")
	if err != nil {
		log.Fatal(err)
	}
	if !*keep {
		defer os.RemoveAll(tmpDir)
	}

	fmt.Fprintf(os.Stderr, "Cloning %s...\n", *repo)
	cmd := exec.Command("git", "clone", "--depth=1", "--branch=main", "--single-branch", *repo, tmpDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("git clone failed: %v", err)
	}
	if *keep {
		fmt.Fprintf(os.Stderr, "Repo cloned to: %s\n", tmpDir)
	}

	// Parse top-level mkdocs.yml
	cfg, err := parseMkdocs(filepath.Join(tmpDir, "mkdocs.yml"))
	if err != nil {
		log.Fatalf("parsing mkdocs.yml: %v", err)
	}

	// Collect all docs
	var docs []docEntry
	docsDir := cfg.DocsDir
	if docsDir == "" {
		docsDir = "docs"
	}
	walkNav(cfg.Nav, filepath.Join(tmpDir, docsDir), tmpDir, nil, &docs)

	fmt.Fprintf(os.Stderr, "Found %d documents\n", len(docs))

	// Write database
	os.Remove(*output)
	db, err := sql.Open("sqlite3", *output)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE docs (
			path TEXT PRIMARY KEY,
			file TEXT NOT NULL,
			content TEXT NOT NULL
		);
		CREATE TABLE help_urls (
			symbol TEXT PRIMARY KEY,
			path TEXT NOT NULL
		);
	`); err != nil {
		log.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	ins, err := tx.Prepare("INSERT OR IGNORE INTO docs (path, file, content) VALUES (?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}

	for _, d := range docs {
		if _, err := ins.Exec(d.path, d.file, d.content); err != nil {
			log.Printf("insert %s: %v", d.path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	// Build file-to-path index for help_urls matching
	fileIndex := make(map[string]string) // normalized file path → nav path
	for _, d := range docs {
		// Normalize: strip subsite prefix to get just the doc-relative path
		// e.g. "language-reference-guide/docs/symbols/iota.md" → "language-reference-guide/symbols/iota"
		norm := normalizeFilePath(d.file)
		fileIndex[norm] = d.path
	}

	// Parse help_urls.h and insert mappings
	if *helpURLs != "" {
		entries, err := parseHelpURLs(*helpURLs)
		if err != nil {
			log.Printf("warning: help_urls: %v", err)
		} else {
			// First pass: find unmatched URLs and try to add their files to the docs table.
			// These are disambiguation pages (e.g. symbols/iota) referenced by help_urls.h
			// but not in the mkdocs nav.
			added := 0
			tx2, err := db.Begin()
			if err != nil {
				log.Fatal(err)
			}
			docIns, err := tx2.Prepare("INSERT OR IGNORE INTO docs (path, file, content) VALUES (?, ?, ?)")
			if err != nil {
				log.Fatal(err)
			}
			for _, e := range entries {
				if _, ok := matchHelpURL(e.url, fileIndex); ok {
					continue // already in docs
				}
				// Try to find the markdown file in the repo
				navPath, filePath, content, ok := findHelpFile(e.url, tmpDir)
				if !ok {
					continue
				}
				docIns.Exec(navPath, filePath, content)
				fileIndex[normalizeFilePath(filePath)] = navPath
				added++
			}
			if err := tx2.Commit(); err != nil {
				log.Fatal(err)
			}
			if added > 0 {
				fmt.Fprintf(os.Stderr, "Added %d disambiguation pages from help_urls.h\n", added)
			}

			// Second pass: now match all help URLs to docs
			matched := 0
			tx3, err := db.Begin()
			if err != nil {
				log.Fatal(err)
			}
			hins, err := tx3.Prepare("INSERT OR IGNORE INTO help_urls (symbol, path) VALUES (?, ?)")
			if err != nil {
				log.Fatal(err)
			}
			for _, e := range entries {
				if navPath, ok := matchHelpURL(e.url, fileIndex); ok {
					hins.Exec(e.symbol, navPath)
					matched++
				}
			}
			if err := tx3.Commit(); err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(os.Stderr, "Help URLs: %d parsed, %d matched to docs\n", len(entries), matched)
		}
	}

	fmt.Fprintf(os.Stderr, "Wrote %s\n", *output)
}

type docEntry struct {
	path    string // nav breadcrumb
	file    string // relative path in repo
	content string
}

func parseMkdocs(path string) (*mkdocsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg mkdocsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

// walkNav recursively traverses a mkdocs nav structure.
// docsDir is the absolute path to the docs directory for the current site.
// repoRoot is the absolute path to the cloned repo root.
// breadcrumb is the current nav path prefix.
func walkNav(nodes []yaml.Node, docsDir, repoRoot string, breadcrumb []string, out *[]docEntry) {
	for i := range nodes {
		walkNavNode(&nodes[i], docsDir, repoRoot, breadcrumb, out)
	}
}

func walkNavNode(node *yaml.Node, docsDir, repoRoot string, breadcrumb []string, out *[]docEntry) {
	switch node.Kind {
	case yaml.ScalarNode:
		// Bare string: "index.md" or "some-file.md"
		addDoc(node.Value, docsDir, repoRoot, breadcrumb, out)

	case yaml.MappingNode:
		// Key-value pairs: {"Title": "file.md"} or {"Title": [...]}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			title := key.Value

			switch val.Kind {
			case yaml.ScalarNode:
				value := val.Value
				if strings.HasPrefix(value, "!include ") {
					handleInclude(value, title, repoRoot, breadcrumb, out)
				} else {
					path := append(breadcrumb, title)
					addDoc(value, docsDir, repoRoot, path, out)
				}
			case yaml.SequenceNode:
				// Nested section
				path := append(breadcrumb, title)
				for j := range val.Content {
					walkNavNode(val.Content[j], docsDir, repoRoot, path, out)
				}
			case yaml.MappingNode:
				path := append(breadcrumb, title)
				walkNavNode(val, docsDir, repoRoot, path, out)
			}
		}

	case yaml.SequenceNode:
		for i := range node.Content {
			walkNavNode(node.Content[i], docsDir, repoRoot, breadcrumb, out)
		}
	}
}

func handleInclude(value, parentTitle string, repoRoot string, breadcrumb []string, out *[]docEntry) {
	// value is like "!include ./subsite/mkdocs.yml"
	relPath := strings.TrimPrefix(value, "!include ")
	relPath = strings.TrimSpace(relPath)
	absPath := filepath.Join(repoRoot, relPath)

	cfg, err := parseMkdocs(absPath)
	if err != nil {
		log.Printf("warning: include %s: %v", relPath, err)
		return
	}

	subsiteDir := filepath.Dir(absPath)
	docsDir := cfg.DocsDir
	if docsDir == "" {
		docsDir = "docs"
	}
	absDocsDir := filepath.Join(subsiteDir, docsDir)

	// Build breadcrumb: parent title + site name
	path := append(breadcrumb, parentTitle)
	if cfg.SiteName != "" && cfg.SiteName != parentTitle {
		// site_name is typically the same as parentTitle; avoid duplication
	}

	walkNav(cfg.Nav, absDocsDir, repoRoot, path, out)
}

func addDoc(mdPath, docsDir, repoRoot string, breadcrumb []string, out *[]docEntry) {
	if !strings.HasSuffix(mdPath, ".md") {
		return
	}
	absPath := filepath.Join(docsDir, mdPath)
	raw, err := os.ReadFile(absPath)
	if err != nil {
		log.Printf("warning: %s: %v", mdPath, err)
		return
	}
	content := stripFrontMatter(raw)

	relFile, _ := filepath.Rel(repoRoot, absPath)
	navPath := strings.Join(breadcrumb, " / ")
	if navPath == "" {
		navPath = mdPath
	}

	*out = append(*out, docEntry{
		path:    navPath,
		file:    relFile,
		content: string(content),
	})
}

// normalizeFilePath strips "docs/" directory segments and the .md extension
// to produce a path comparable to help_urls.h URL paths.
// e.g. "language-reference-guide/docs/symbols/iota.md" → "language-reference-guide/symbols/iota"
func normalizeFilePath(file string) string {
	// Remove /docs/ segment (subsites have subsite/docs/path.md)
	file = strings.ReplaceAll(file, "/docs/", "/")
	// Strip leading docs/ for top-level files
	file = strings.TrimPrefix(file, "docs/")
	// Strip .md extension
	file = strings.TrimSuffix(file, ".md")
	// Strip trailing /index (index.md pages)
	file = strings.TrimSuffix(file, "/index")
	return file
}

type helpURLEntry struct {
	symbol string
	url    string // expanded URL path
}

var (
	defineRe  = regexp.MustCompile(`^#define\s+(\w+)\s+"([^"]*)"`)
	helpURLRe = regexp.MustCompile(`^HELP_URL\("([^"]*?)"\s*,\s*(.+)\)`)
)

func parseHelpURLs(path string) ([]helpURLEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	macros := make(map[string]string)
	var entries []helpURLEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if m := defineRe.FindStringSubmatch(line); m != nil {
			macros[m[1]] = m[2]
			continue
		}

		if m := helpURLRe.FindStringSubmatch(line); m != nil {
			symbol := m[1]
			urlExpr := strings.TrimSpace(m[2])
			url := expandMacro(urlExpr, macros)
			entries = append(entries, helpURLEntry{symbol: symbol, url: url})
		}
	}

	return entries, scanner.Err()
}

// expandMacro expands expressions like: SY"/iota" or "full/path" or QUAD"/json"
func expandMacro(expr string, macros map[string]string) string {
	expr = strings.TrimSpace(expr)

	// Pure string literal: "some/path"
	if strings.HasPrefix(expr, "\"") {
		return strings.Trim(expr, "\"")
	}

	// MACRO"/suffix" or MACRO (no suffix)
	for name, value := range macros {
		if strings.HasPrefix(expr, name) {
			rest := strings.TrimPrefix(expr, name)
			if rest == "" {
				return value
			}
			// rest is like "/iota" (with quotes from concatenation)
			rest = strings.Trim(rest, "\"")
			return value + rest
		}
	}

	return strings.Trim(expr, "\"")
}

// matchHelpURL tries to match a help URL path to a doc entry's file path.
func matchHelpURL(url string, fileIndex map[string]string) (string, bool) {
	// Direct match
	if navPath, ok := fileIndex[url]; ok {
		return navPath, true
	}

	// Try with /index suffix (section pages)
	if navPath, ok := fileIndex[url+"/index"]; ok {
		return navPath, true
	}

	// Partial suffix match: find the entry whose normalized file path ends with the URL
	for filePath, navPath := range fileIndex {
		if strings.HasSuffix(filePath, "/"+url) || filePath == url {
			return navPath, true
		}
	}

	return "", false
}

// findHelpFile locates a markdown file in the cloned repo for a help URL path
// that isn't in the mkdocs nav. These are disambiguation pages.
// Returns (navPath, relFilePath, content, ok).
func findHelpFile(url, repoRoot string) (string, string, string, bool) {
	// The URL is like "language-reference-guide/symbols/iota"
	// The file would be at "language-reference-guide/docs/symbols/iota.md"
	// or "language-reference-guide/docs/symbols/iota/index.md"
	parts := strings.SplitN(url, "/", 2)
	if len(parts) < 2 {
		return "", "", "", false
	}
	subsite := parts[0]
	rest := parts[1]

	candidates := []string{
		filepath.Join(repoRoot, subsite, "docs", rest+".md"),
		filepath.Join(repoRoot, subsite, "docs", rest, "index.md"),
	}

	for _, candidate := range candidates {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		relFile, _ := filepath.Rel(repoRoot, candidate)
		// Build a synthetic nav path from the URL segments
		navPath := buildNavPath(url)
		return navPath, relFile, stripFrontMatter(content), true
	}

	return "", "", "", false
}

// stripFrontMatter removes YAML front-matter (--- delimited) from markdown content.
func stripFrontMatter(raw []byte) string {
	s := string(raw)
	if !strings.HasPrefix(s, "---") {
		return s
	}
	// Find closing ---
	end := strings.Index(s[3:], "\n---")
	if end < 0 {
		return s
	}
	return strings.TrimLeft(s[3+end+4:], "\n")
}

// buildNavPath creates a readable nav path from a URL like
// "language-reference-guide/symbols/iota" → "Language Reference Guide / Symbols / Iota"
func buildNavPath(url string) string {
	parts := strings.Split(url, "/")
	for i, p := range parts {
		// Title-case each segment, replacing hyphens with spaces
		words := strings.Split(p, "-")
		for j, w := range words {
			if len(w) > 0 {
				words[j] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		parts[i] = strings.Join(words, " ")
	}
	return strings.Join(parts, " / ")
}
