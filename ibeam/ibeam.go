// Package ibeam provides I-beam (⌶) lookup from two sources:
//
//  1. Public Dyalog documentation (from the cached docs DB)
//  2. Private/undocumented I-beams (from ~/.config/gritt/ibeams.csv)
//
// The private CSV is for Dyalog developers — it may contain internal
// I-beams not in the public docs.
package ibeam

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Entry represents a single I-beam.
type Entry struct {
	Number      int    // The I-beam number (e.g. 220)
	Name        string // Short name (e.g. "Serialise/Deserialise Array")
	Signature   string // Full signature (e.g. "R←X(220⌶)Y")
	Source      string // "docs" or "private"
	DocPath     string // Path in docs DB (empty for private entries)
	Description string // Brief description (from CSV, or empty for docs entries)
}

var numberRe = regexp.MustCompile(`(\d+)⌶`)

// Search finds I-beams matching a query. Searches both public docs and
// the private CSV. Results are sorted by I-beam number, deduplicated
// (private entries supplement but don't replace public ones).
func Search(db *sql.DB, query string) []Entry {
	var results []Entry
	seen := make(map[int]bool)

	// Public docs
	if db != nil {
		public := searchDocs(db, query)
		for _, e := range public {
			results = append(results, e)
			seen[e.Number] = true
		}
	}

	// Private CSV
	private := searchCSV(query)
	for _, e := range private {
		if !seen[e.Number] {
			results = append(results, e)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Number < results[j].Number
	})
	return results
}

// All returns every known I-beam, sorted by number.
func All(db *sql.DB) []Entry {
	return Search(db, "")
}

// Lookup finds a specific I-beam by number.
func Lookup(db *sql.DB, number int) *Entry {
	// Check docs first
	if db != nil {
		e := lookupDocs(db, number)
		if e != nil {
			return e
		}
	}
	// Check CSV
	entries := loadCSV()
	for i := range entries {
		if entries[i].Number == number {
			return &entries[i]
		}
	}
	return nil
}

// --- Public docs ---

func searchDocs(db *sql.DB, query string) []Entry {
	var rows *sql.Rows
	var err error

	if query == "" {
		rows, err = db.Query(`SELECT title, path FROM docs WHERE path LIKE '%i-beam%' AND title LIKE '%⌶%' ORDER BY rowid`)
	} else if _, err2 := strconv.Atoi(query); err2 == nil {
		// Numeric: prefix match — "62" finds 620⌶, 625⌶, 62583⌶, etc.
		pattern := "%" + query + "%⌶%"
		rows, err = db.Query(`SELECT title, path FROM docs WHERE path LIKE '%i-beam%' AND title LIKE ? ORDER BY rowid`, pattern)
	} else {
		// Text: search title and content (case-insensitive)
		like := "%" + strings.ToLower(query) + "%"
		rows, err = db.Query(`SELECT title, path FROM docs WHERE path LIKE '%i-beam%' AND (LOWER(title) LIKE ? OR LOWER(content) LIKE ?) ORDER BY rowid`, like, like)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []Entry
	for rows.Next() {
		var title, path string
		if err := rows.Scan(&title, &path); err != nil {
			continue
		}
		e := parseDocEntry(title, path)
		if e.Number > 0 {
			results = append(results, e)
		}
	}
	return results
}

func lookupDocs(db *sql.DB, number int) *Entry {
	pattern := fmt.Sprintf("%%%d⌶%%", number)
	var title, path string
	err := db.QueryRow(`SELECT title, path FROM docs WHERE path LIKE '%i-beam%' AND title LIKE ? LIMIT 1`, pattern).Scan(&title, &path)
	if err != nil {
		return nil
	}
	e := parseDocEntry(title, path)
	if e.Number > 0 {
		return &e
	}
	return nil
}

func parseDocEntry(title, path string) Entry {
	e := Entry{Source: "docs", DocPath: path}

	// Extract number from signature: "Name R←X(220⌶)Y" or "Name R←220⌶Y"
	if m := numberRe.FindStringSubmatch(title); len(m) > 1 {
		e.Number, _ = strconv.Atoi(m[1])
	}

	// Split title into name and signature at the first R← or {R}← or N⌶
	// Title format: "Name Signature"
	parts := strings.SplitN(title, " R←", 2)
	if len(parts) == 2 {
		e.Name = strings.TrimSpace(parts[0])
		e.Signature = "R←" + parts[1]
	} else {
		parts = strings.SplitN(title, " {R}←", 2)
		if len(parts) == 2 {
			e.Name = strings.TrimSpace(parts[0])
			e.Signature = "{R}←" + parts[1]
		} else if m := numberRe.FindStringIndex(title); m != nil {
			// Find where the signature starts
			// Look for the number backwards to find the name
			sigStart := m[0]
			for sigStart > 0 && title[sigStart-1] != ' ' {
				sigStart--
			}
			e.Name = strings.TrimSpace(title[:sigStart])
			e.Signature = strings.TrimSpace(title[sigStart:])
		} else {
			e.Name = title
		}
	}

	return e
}

// --- Private CSV ---

// csvPath returns the path to the private I-beams CSV.
func csvPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "gritt", "ibeams.csv")
}

// loadCSV reads the private I-beams CSV if it exists.
// Format: number,name,signature,description
func loadCSV() []Entry {
	path := csvPath()
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = -1 // variable
	records, err := r.ReadAll()
	if err != nil {
		return nil
	}

	var entries []Entry
	for _, rec := range records {
		if len(rec) < 2 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(rec[0]))
		if err != nil {
			continue
		}
		e := Entry{
			Number: n,
			Name:   strings.TrimSpace(rec[1]),
			Source: "private",
		}
		if len(rec) > 2 {
			e.Signature = strings.TrimSpace(rec[2])
		}
		if len(rec) > 3 {
			e.Description = strings.TrimSpace(rec[3])
		}
		entries = append(entries, e)
	}
	return entries
}

func searchCSV(query string) []Entry {
	entries := loadCSV()
	if query == "" {
		return entries
	}

	q := strings.ToLower(query)
	seen := make(map[int]bool)
	var results []Entry

	// Numeric prefix: match I-beams whose number starts with the query
	if _, err := strconv.Atoi(query); err == nil {
		for _, e := range entries {
			numStr := strconv.Itoa(e.Number)
			if strings.HasPrefix(numStr, query) {
				results = append(results, e)
				seen[e.Number] = true
			}
		}
	}

	// Text: fuzzy match on name, description, and signature
	for _, e := range entries {
		if seen[e.Number] {
			continue
		}
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Description), q) ||
			strings.Contains(strings.ToLower(e.Signature), q) {
			results = append(results, e)
		}
	}
	return results
}
