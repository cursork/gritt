package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

const docsReleaseAPI = "https://api.github.com/repos/xpqz/bundle-docs/releases/latest"

// DocsCacheResult is sent when a docs cache fetch/refresh completes.
type DocsCacheResult struct {
	Err error
}

// RefreshDocsCache downloads the latest docs database from GitHub releases.
func RefreshDocsCache() tea.Msg {
	dbPath := cachePath("dyalog-docs.db")
	if dbPath == "" {
		return DocsCacheResult{Err: fmt.Errorf("cache dir unavailable")}
	}

	// Get latest release info
	resp, err := http.Get(docsReleaseAPI)
	if err != nil {
		return DocsCacheResult{Err: err}
	}
	defer resp.Body.Close()

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return DocsCacheResult{Err: err}
	}

	// Find the .db asset
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".db") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return DocsCacheResult{Err: fmt.Errorf("no .db asset in latest release")}
	}

	// Download to temp file, then rename
	resp2, err := http.Get(downloadURL)
	if err != nil {
		return DocsCacheResult{Err: err}
	}
	defer resp2.Body.Close()

	tmp := dbPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return DocsCacheResult{Err: err}
	}
	if _, err := io.Copy(f, resp2.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return DocsCacheResult{Err: err}
	}
	f.Close()

	if err := os.Rename(tmp, dbPath); err != nil {
		return DocsCacheResult{Err: err}
	}

	return DocsCacheResult{}
}

// DocSearchResult represents a single search result
type DocSearchResult struct {
	Rowid int64
	Title string
	Path  string
}

// DocSearch is a searchable documentation browser
type DocSearch struct {
	db             *sql.DB
	query          string
	results        []DocSearchResult
	selected       int
	scrollOffset   int
	SelectedResult *DocSearchResult // Set when Enter pressed
}

// NewDocSearch creates a doc search pane with the given database
func NewDocSearch(db *sql.DB) *DocSearch {
	return &DocSearch{
		db: db,
	}
}

func (d *DocSearch) search() {
	if d.db == nil || d.query == "" {
		d.results = nil
		d.selected = 0
		d.scrollOffset = 0
		return
	}

	d.results = searchDocs(d.db, d.query, 50)

	// Reset selection if out of bounds
	if d.selected >= len(d.results) {
		d.selected = len(d.results) - 1
	}
	if d.selected < 0 {
		d.selected = 0
	}
	d.scrollOffset = 0
}

// searchDocs performs the three-tier search like docsearch
func searchDocs(db *sql.DB, query string, limit int) []DocSearchResult {
	seen := make(map[int64]bool)
	var results []DocSearchResult

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

func collectResults(rows *sql.Rows, seen map[int64]bool, limit int, results []DocSearchResult) []DocSearchResult {
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
		results = append(results, DocSearchResult{Rowid: rowid, Title: title, Path: path})
	}
	return results
}

// escapeQuery wraps the query in quotes to handle special characters
func escapeQuery(q string) string {
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}

func (d *DocSearch) Title() string {
	return "Search Docs"
}

func (d *DocSearch) Render(w, h int) string {
	var sb strings.Builder

	// Query line
	promptStyle := lipgloss.NewStyle().Foreground(AccentColor)
	sb.WriteString(promptStyle.Render("/ "))
	sb.WriteString(d.query)
	sb.WriteString(cursorStyle.Render(" "))
	sb.WriteString("\n")

	// Separator
	sb.WriteString(strings.Repeat("─", w))
	sb.WriteString("\n")

	// Results list
	selectedStyle := lipgloss.NewStyle().Background(AccentColor).Foreground(lipgloss.Color("0"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	listH := h - 2 // Account for query line and separator
	if listH < 1 {
		listH = 1
	}

	// Adjust scroll to keep selection visible
	d.adjustScroll(listH)

	if len(d.results) == 0 && d.query != "" {
		sb.WriteString(pathStyle.Render("No results"))
		return sb.String()
	}

	// Render visible items based on scroll offset
	visibleCount := 0
	for i := d.scrollOffset; i < len(d.results) && visibleCount < listH; i++ {
		result := d.results[i]
		title := result.Title

		// Truncate title if needed
		maxTitle := w - 2
		if maxTitle < 10 {
			maxTitle = 10
		}
		titleRunes := []rune(title)
		if len(titleRunes) > maxTitle {
			title = string(titleRunes[:maxTitle-1]) + "…"
		}

		var line string
		if i == d.selected {
			line = selectedStyle.Render(title)
		} else {
			line = title
		}

		sb.WriteString(line)
		visibleCount++
		if visibleCount < listH {
			sb.WriteString("\n")
		}
	}

	// Pad remaining lines if needed
	for visibleCount < listH {
		sb.WriteString(strings.Repeat(" ", w))
		visibleCount++
		if visibleCount < listH {
			sb.WriteString("\n")
		}
	}

	// Show count if there are results
	if len(d.results) > 0 {
		count := fmt.Sprintf(" %d results ", len(d.results))
		sb.WriteString("\n")
		sb.WriteString(pathStyle.Render(count))
	}

	return sb.String()
}

func (d *DocSearch) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if d.selected > 0 {
			d.selected--
		}
		return true

	case tea.KeyDown:
		if d.selected < len(d.results)-1 {
			d.selected++
		}
		return true

	case tea.KeyEnter:
		if d.selected >= 0 && d.selected < len(d.results) {
			d.SelectedResult = &d.results[d.selected]
		}
		return true

	case tea.KeyBackspace:
		if len(d.query) > 0 {
			d.query = d.query[:len(d.query)-1]
			d.search()
		}
		return true

	case tea.KeyEscape:
		// Let parent handle escape
		return false

	default:
		if len(msg.Runes) > 0 {
			d.query += string(msg.Runes)
			d.search()
			return true
		}
	}

	return false
}

func (d *DocSearch) adjustScroll(listH int) {
	if listH < 1 {
		listH = 1
	}
	// Scroll down if selected is below visible area
	if d.selected >= d.scrollOffset+listH {
		d.scrollOffset = d.selected - listH + 1
	}
	// Scroll up if selected is above visible area
	if d.selected < d.scrollOffset {
		d.scrollOffset = d.selected
	}
}

func (d *DocSearch) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonLeft && y >= 2 {
		idx := d.scrollOffset + y - 2
		if idx >= 0 && idx < len(d.results) {
			d.selected = idx
			d.SelectedResult = &d.results[d.selected]
			return true
		}
	}
	return false
}
