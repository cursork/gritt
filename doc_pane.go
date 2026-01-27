package main

import (
	"database/sql"
	"fmt"
	"path"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss/v2"
)

// DocPane displays rendered markdown documentation in a floating pane.
type DocPane struct {
	navPath  string
	file     string // file column from docs table (for resolving relative links)
	rawLines []string // glamour-rendered lines with «markers» intact
	lines    []string // display lines with styled links
	scroll   int
	links    []docLink
	linkIdx  int // -1 = no selection
	linkPos  []int // line index where each link marker appears
	db       *sql.DB
	width    int
	history  []docState
}

type docLink struct {
	display string
	file    string // resolved file path relative to repo root
}

type docState struct {
	navPath string
	file    string
	scroll  int
}

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// processLinks extracts internal markdown links, replacing them with «Text»
// markers and returning the resolved link targets.
func processLinks(markdown, currentFile string) (string, []docLink) {
	dir := path.Dir(currentFile)
	var links []docLink

	processed := mdLinkRe.ReplaceAllStringFunc(markdown, func(match string) string {
		m := mdLinkRe.FindStringSubmatch(match)
		text, target := m[1], m[2]

		// Skip external links
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
			return match
		}

		// Strip anchor
		if i := strings.Index(target, "#"); i >= 0 {
			target = target[:i]
		}
		if target == "" {
			return text // anchor-only link, just show text
		}

		// Resolve relative path
		resolved := path.Clean(path.Join(dir, target))
		links = append(links, docLink{display: text, file: resolved})
		return fmt.Sprintf("«%s»", text)
	})

	return processed, links
}

// RenderMarkdown pre-renders markdown for terminal display at the given width.
func RenderMarkdown(markdown string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return markdown
	}
	out, err := r.Render(markdown)
	if err != nil {
		return markdown
	}
	return out
}

func NewDocPane(navPath, file, rendered string, links []docLink, db *sql.DB, width int) *DocPane {
	rawLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")

	// Find which line each link marker appears on
	linkPos := findLinkPositions(rawLines, links)

	dp := &DocPane{
		navPath:  navPath,
		file:     file,
		links:    links,
		linkIdx:  -1,
		linkPos:  linkPos,
		db:       db,
		width:    width,
		rawLines: rawLines,
	}
	dp.styleLinks()
	return dp
}

func findLinkPositions(lines []string, links []docLink) []int {
	linkPos := make([]int, len(links))
	linkFound := 0
	for i, line := range lines {
		plain := stripANSI(line)
		for linkFound < len(links) && strings.Contains(plain, "«"+links[linkFound].display+"»") {
			linkPos[linkFound] = i
			linkFound++
		}
	}
	return linkPos
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

var (
	docLinkStyle    = lipgloss.NewStyle().Underline(true)
	docSelectedStyle = lipgloss.NewStyle().Underline(true).Bold(true).Reverse(true)
)

// styleLinks rebuilds d.lines from d.rawLines, replacing «Text» markers
// with styled link text.
func (d *DocPane) styleLinks() {
	linkStyle := docLinkStyle.Foreground(AccentColor)
	selectedStyle := docSelectedStyle

	// Copy raw lines
	d.lines = make([]string, len(d.rawLines))
	copy(d.lines, d.rawLines)

	for i, link := range d.links {
		marker := fmt.Sprintf("«%s»", link.display)
		var styled string
		if i == d.linkIdx {
			styled = selectedStyle.Render(link.display)
		} else {
			styled = linkStyle.Render(link.display)
		}
		for j, line := range d.lines {
			if strings.Contains(line, marker) {
				d.lines[j] = strings.Replace(line, marker, styled, 1)
				break
			}
		}
	}
}

func (d *DocPane) Title() string {
	if len(d.history) > 0 {
		return fmt.Sprintf("← %s", d.navPath)
	}
	return d.navPath
}

func (d *DocPane) Render(w, h int) string {
	var sb strings.Builder
	end := d.scroll + h
	if end > len(d.lines) {
		end = len(d.lines)
	}

	for i := d.scroll; i < end; i++ {
		sb.WriteString(d.lines[i])
		if i < end-1 {
			sb.WriteRune('\n')
		}
	}

	// Scroll position indicator on last line
	if len(d.lines) > h {
		sb.WriteRune('\n')
		pos := fmt.Sprintf(" %d/%d ", d.scroll+1, len(d.lines))
		pad := w - len(pos)
		if pad > 0 {
			sb.WriteString(strings.Repeat("─", pad))
		}
		sb.WriteString(pos)
	}

	return sb.String()
}

func (d *DocPane) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		d.scrollUp(1)
	case tea.KeyDown:
		d.scrollDown(1)
	case tea.KeyPgUp:
		d.scrollUp(20)
	case tea.KeyPgDown:
		d.scrollDown(20)
	case tea.KeyTab:
		d.nextLink()
	case tea.KeyShiftTab:
		d.prevLink()
	case tea.KeyEnter:
		d.followLink()
	case tea.KeyBackspace:
		d.goBack()
	default:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'j':
				d.scrollDown(1)
			case 'k':
				d.scrollUp(1)
			case 'b':
				d.goBack()
			default:
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func (d *DocPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	switch msg.Type {
	case tea.MouseWheelUp:
		d.scrollUp(3)
		return true
	case tea.MouseWheelDown:
		d.scrollDown(3)
		return true
	}
	return false
}

func (d *DocPane) scrollUp(n int) {
	d.scroll -= n
	if d.scroll < 0 {
		d.scroll = 0
	}
}

func (d *DocPane) scrollDown(n int) {
	d.scroll += n
	max := len(d.lines) - 10
	if max < 0 {
		max = 0
	}
	if d.scroll > max {
		d.scroll = max
	}
}

func (d *DocPane) nextLink() {
	if len(d.links) == 0 {
		return
	}
	d.linkIdx++
	if d.linkIdx >= len(d.links) {
		d.linkIdx = 0
	}
	d.scrollToLink()
	d.styleLinks()
}

func (d *DocPane) prevLink() {
	if len(d.links) == 0 {
		return
	}
	d.linkIdx--
	if d.linkIdx < 0 {
		d.linkIdx = len(d.links) - 1
	}
	d.scrollToLink()
	d.styleLinks()
}

func (d *DocPane) scrollToLink() {
	if d.linkIdx < 0 || d.linkIdx >= len(d.linkPos) {
		return
	}
	line := d.linkPos[d.linkIdx]
	if line < d.scroll+2 {
		d.scroll = line - 2
		if d.scroll < 0 {
			d.scroll = 0
		}
	} else if line > d.scroll+20 {
		d.scroll = line - 5
	}
}

func (d *DocPane) followLink() {
	if d.linkIdx < 0 || d.linkIdx >= len(d.links) || d.db == nil {
		return
	}
	link := d.links[d.linkIdx]

	var navPath, content string
	err := d.db.QueryRow("SELECT path, content FROM docs WHERE file = ?", link.file).Scan(&navPath, &content)
	if err != nil {
		return
	}

	// Push current state
	d.history = append(d.history, docState{
		navPath: d.navPath,
		file:    d.file,
		scroll:  d.scroll,
	})

	d.loadContent(navPath, link.file, content)
}

func (d *DocPane) goBack() {
	if len(d.history) == 0 {
		return
	}
	prev := d.history[len(d.history)-1]
	d.history = d.history[:len(d.history)-1]

	content := ""
	if d.db != nil {
		d.db.QueryRow("SELECT content FROM docs WHERE path = ?", prev.navPath).Scan(&content)
	}

	d.loadContent(prev.navPath, prev.file, content)
	d.scroll = prev.scroll
}

func (d *DocPane) loadContent(navPath, file, content string) {
	processed, links := processLinks(content, file)
	rendered := RenderMarkdown(processed, d.width)
	rawLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")

	d.navPath = navPath
	d.file = file
	d.rawLines = rawLines
	d.links = links
	d.linkIdx = -1
	d.linkPos = findLinkPositions(rawLines, links)
	d.scroll = 0
	d.styleLinks()
}
