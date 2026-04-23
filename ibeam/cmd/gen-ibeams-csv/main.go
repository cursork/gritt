// gen-ibeams-csv builds ~/.config/gritt/ibeams.csv from two sources:
//
//  1. A webarchive or HTML file of the Dyalog internal wiki I-beams page
//  2. A text file of all known I-beam numbers (one per line or space-separated)
//
// Numbers from the text file that aren't in the wiki get an "UNKNOWN" entry.
//
// Usage:
//
//	gen-ibeams-csv -wiki internal.webarchive -numbers ibeams.txt
//	gen-ibeams-csv -wiki internal.html -numbers ibeams.txt
//	gen-ibeams-csv -numbers ibeams.txt   # numbers only, no wiki descriptions
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func main() {
	wikiPath := flag.String("wiki", "", "Path to Dyalog internal wiki page (.webarchive or .html)")
	numbersPath := flag.String("numbers", "", "Path to text file with all I-beam numbers")
	output := flag.String("o", "", "Output path (default: ~/.config/gritt/ibeams.csv)")
	flag.Parse()

	if *numbersPath == "" {
		fmt.Fprintln(os.Stderr, "usage: gen-ibeams-csv -numbers ibeams.txt [-wiki page.webarchive] [-o output.csv]")
		os.Exit(1)
	}

	// Determine output path
	outPath := *output
	if outPath == "" {
		home := os.Getenv("HOME")
		if home == "" {
			fmt.Fprintln(os.Stderr, "HOME not set")
			os.Exit(1)
		}
		outPath = filepath.Join(home, ".config", "gritt", "ibeams.csv")
	}

	// Load all known numbers
	allNumbers := loadNumbers(*numbersPath)
	fmt.Fprintf(os.Stderr, "Loaded %d unique I-beam numbers\n", len(allNumbers))

	// Parse wiki if provided
	wikiEntries := make(map[int]wikiEntry)
	if *wikiPath != "" {
		wikiEntries = parseWiki(*wikiPath)
		fmt.Fprintf(os.Stderr, "Parsed %d entries from wiki\n", len(wikiEntries))
	}

	// Build CSV: wiki entries, UNKNOWN for missing, "Allocated to RH" for 9000 range
	var rows []csvRow
	for _, n := range allNumbers {
		if e, ok := wikiEntries[n]; ok {
			rows = append(rows, csvRow{n, e.name, e.sig, e.desc})
		} else if n >= 9000 && n <= 9996 {
			rows = append(rows, csvRow{n, "Allocated to RH", fmt.Sprintf("%d⌶", n), "Reserved range for RH"})
		} else {
			rows = append(rows, csvRow{n, "UNKNOWN", fmt.Sprintf("%d⌶", n), ""})
		}
	}

	// Write CSV
	os.MkdirAll(filepath.Dir(outPath), 0755)
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	for _, r := range rows {
		w.Write([]string{strconv.Itoa(r.number), r.name, r.sig, r.desc})
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "Wrote %d entries to %s\n", len(rows), outPath)

	// Summary
	known, unknown := 0, 0
	for _, r := range rows {
		if r.name == "UNKNOWN" {
			unknown++
		} else {
			known++
		}
	}
	fmt.Fprintf(os.Stderr, "  %d with descriptions, %d UNKNOWN\n", known, unknown)
}

type csvRow struct {
	number int
	name   string
	sig    string
	desc   string
}

type wikiEntry struct {
	name string
	sig  string
	desc string
}

func loadNumbers(path string) []int {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}
	fields := strings.Fields(string(raw))
	seen := make(map[int]bool)
	var nums []int
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			continue
		}
		if !seen[n] {
			nums = append(nums, n)
			seen[n] = true
		}
	}
	sort.Ints(nums)
	return nums
}

func parseWiki(path string) map[int]wikiEntry {
	// Convert webarchive to HTML if needed
	htmlPath := path
	if strings.HasSuffix(strings.ToLower(path), ".webarchive") {
		tmp, err := os.CreateTemp("", "ibeams-*.html")
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file: %v\n", err)
			return nil
		}
		tmp.Close()
		htmlPath = tmp.Name()
		defer os.Remove(htmlPath)

		cmd := exec.Command("textutil", "-convert", "html", "-output", htmlPath, path)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "textutil: %v\n", err)
			return nil
		}
	}

	f, err := os.Open(htmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", htmlPath, err)
		return nil
	}
	defer f.Close()

	doc, err := html.Parse(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse html: %v\n", err)
		return nil
	}
	text := textContent(doc)

	// Skip past preamble to the actual descriptions
	idx := strings.Index(text, "ask Andy.")
	if idx < 0 {
		// Try alternative markers
		idx = strings.Index(text, "10⌶ - Set APL_CODE_E_MAGNITUDE")
		if idx < 0 {
			fmt.Fprintln(os.Stderr, "couldn't find description section in wiki page")
			return nil
		}
	} else {
		text = text[idx:]
	}

	headerRe := regexp.MustCompile(`(\d+)⌶\s*[-–]\s*([^\n]+)`)
	locs := headerRe.FindAllStringSubmatchIndex(text, -1)

	result := make(map[int]wikiEntry)
	for i, loc := range locs {
		numStr := text[loc[2]:loc[3]]
		num, _ := strconv.Atoi(numStr)
		name := clean(text[loc[4]:loc[5]])

		start := loc[1]
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		desc := clean(text[start:end])
		if len(desc) > 800 {
			desc = desc[:800]
		}

		if _, exists := result[num]; exists {
			continue
		}

		sig := fmt.Sprintf("%d⌶", num)
		sigRe := regexp.MustCompile(`(R?\s*←[^⌶\n]{0,15}` + numStr + `⌶[^\n]{0,15})`)
		if m := sigRe.FindStringSubmatch(desc); len(m) > 1 {
			s := clean(m[1])
			if len(s) < 50 {
				sig = s
			}
		}

		result[num] = wikiEntry{name: name, sig: sig, desc: desc}
	}
	return result
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}
