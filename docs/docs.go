// Package docs provides Dyalog documentation search and caching.
// Shares the SQLite cache at ~/.cache/gritt/dyalog-docs.db with gritt's TUI.
package docs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/cursork/gritt/cache"
	_ "modernc.org/sqlite"
)

const releaseAPI = "https://api.github.com/repos/xpqz/bundle-docs/releases/latest"

// Result represents a single search result.
type Result struct {
	Rowid int64
	Title string
	Path  string
}

// Search performs a three-tier search: keyword match, FTS title, FTS content.
func Search(db *sql.DB, query string, limit int) []Result {
	seen := make(map[int64]bool)
	var results []Result

	// 1. Exact case-insensitive match on keywords
	rows, err := db.Query(`
		SELECT rowid, title, path FROM docs
		WHERE keywords LIKE ? COLLATE NOCASE AND exclude = 0
	`, "%"+query+"%")
	if err == nil {
		results = collectResults(rows, seen, limit, results)
	}
	if len(results) >= limit {
		return results
	}

	// 2. FTS search on title
	rows, err = db.Query(`
		SELECT f.rowid, f.title, f.path FROM docs_fts f
		JOIN docs d ON f.rowid = d.rowid
		WHERE f.title MATCH ? AND d.exclude = 0
	`, escapeQuery(query))
	if err == nil {
		results = collectResults(rows, seen, limit, results)
	}
	if len(results) >= limit {
		return results
	}

	// 3. FTS search on content
	rows, err = db.Query(`
		SELECT f.rowid, f.title, f.path FROM docs_fts f
		JOIN docs d ON f.rowid = d.rowid
		WHERE f.content MATCH ? AND d.exclude = 0
	`, escapeQuery(query))
	if err == nil {
		results = collectResults(rows, seen, limit, results)
	}

	return results
}

// OpenCache opens the docs database in read-only mode.
func OpenCache() (*sql.DB, error) {
	dbPath := cache.Path("dyalog-docs.db")
	if dbPath == "" {
		return nil, fmt.Errorf("cache dir unavailable")
	}
	return sql.Open("sqlite", dbPath+"?mode=ro")
}

// RefreshCache downloads the latest docs database from GitHub releases.
func RefreshCache() error {
	dbPath := cache.Path("dyalog-docs.db")
	if dbPath == "" {
		return fmt.Errorf("cache dir unavailable")
	}

	resp, err := http.Get(releaseAPI)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".db") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no .db asset in latest release")
	}

	resp2, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	tmp := dbPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp2.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()

	return os.Rename(tmp, dbPath)
}

// CacheIsStale returns true if the cache needs refreshing.
func CacheIsStale() bool {
	dbPath := cache.Path("dyalog-docs.db")
	return dbPath == "" || cache.IsStale(dbPath)
}

// Content retrieves the markdown content for a search result.
func Content(db *sql.DB, path string) (string, error) {
	var content string
	err := db.QueryRow("SELECT content FROM docs WHERE path = ?", path).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("doc not found: %s", path)
	}
	return content, nil
}

// --- Internal ---

func collectResults(rows *sql.Rows, seen map[int64]bool, limit int, results []Result) []Result {
	defer rows.Close()
	for rows.Next() {
		if len(results) >= limit {
			break
		}
		var rowid int64
		var title, path string
		if err := rows.Scan(&rowid, &title, &path); err != nil {
			continue
		}
		if seen[rowid] {
			continue
		}
		seen[rowid] = true
		results = append(results, Result{Rowid: rowid, Title: title, Path: path})
	}
	return results
}

func escapeQuery(q string) string {
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}
