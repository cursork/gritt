// Package aplcart provides APLcart data loading, caching, and search.
// Shares the SQLite cache at ~/.cache/gritt/aplcart.db with gritt's TUI.
package aplcart

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/cursork/gritt/cache"
	_ "modernc.org/sqlite"
)

const sourceURL = "https://raw.githubusercontent.com/abrudz/aplcart/master/table.tsv"

// Entry represents one APLcart entry.
type Entry struct {
	Syntax      string
	Description string
	Keywords    string
}

// LoadCache loads entries from the SQLite cache.
func LoadCache() ([]Entry, error) {
	dbPath := cache.Path("aplcart.db")
	if dbPath == "" {
		return nil, fmt.Errorf("cache dir unavailable")
	}
	return loadFrom(dbPath)
}

// RefreshCache fetches APLcart from GitHub and updates the cache.
func RefreshCache() ([]Entry, error) {
	resp, err := http.Get(sourceURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	entries := parseTSV(string(body))
	if err := writeCache(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// Search filters entries by query, matching against syntax, description, and keywords.
// Results with syntax matches sort before description/keyword-only matches.
func Search(entries []Entry, query string) []Entry {
	if query == "" {
		return entries
	}

	q := strings.ToLower(query)
	var results []Entry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Description), q) ||
			strings.Contains(strings.ToLower(e.Keywords), q) ||
			strings.Contains(strings.ToLower(e.Syntax), q) {
			results = append(results, e)
		}
	}

	// Syntax matches sort first
	sort.SliceStable(results, func(i, j int) bool {
		iSyntax := strings.Contains(strings.ToLower(results[i].Syntax), q)
		jSyntax := strings.Contains(strings.ToLower(results[j].Syntax), q)
		return iSyntax && !jSyntax
	})

	return results
}

// CacheIsStale returns true if the cache needs refreshing.
func CacheIsStale() bool {
	dbPath := cache.Path("aplcart.db")
	return dbPath == "" || cache.IsStale(dbPath)
}

// --- Internal ---

func loadFrom(dbPath string) ([]Entry, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT syntax, description, keywords FROM entries")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Syntax, &e.Description, &e.Keywords); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func parseTSV(data string) []Entry {
	lines := strings.Split(data, "\n")
	entries := make([]Entry, 0, len(lines))
	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		entries = append(entries, Entry{
			Syntax:      fields[0],
			Description: fields[1],
			Keywords:    fields[6],
		})
	}
	return entries
}

func writeCache(entries []Entry) error {
	dbPath := cache.Path("aplcart.db")
	if dbPath == "" {
		return fmt.Errorf("cache dir unavailable")
	}
	return writeCacheTo(dbPath, entries)
}

func writeCacheTo(dbPath string, entries []Entry) error {
	tmp := dbPath + ".tmp"
	os.Remove(tmp)
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE entries (syntax TEXT, description TEXT, keywords TEXT)`); err != nil {
		db.Close()
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO entries (syntax, description, keywords) VALUES (?, ?, ?)")
	if err != nil {
		db.Close()
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		stmt.Exec(e.Syntax, e.Description, e.Keywords)
	}
	if err := tx.Commit(); err != nil {
		db.Close()
		return err
	}
	db.Close()

	return os.Rename(tmp, dbPath)
}
