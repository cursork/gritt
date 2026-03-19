// apldocs searches Dyalog documentation from the terminal.
//
// Usage:
//
//	apldocs "each"              # search and display first match
//	apldocs -list "each"        # list all matches
//	apldocs -n 2 "each"         # display Nth match
//	apldocs -refresh             # download latest docs DB
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/charmbracelet/glamour"
	"github.com/cursork/gritt/docs"
)

func main() {
	refresh := false
	list := false
	pick := 1
	var query string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-refresh":
			refresh = true
		case "-list":
			list = true
		case "-n":
			if i+1 < len(args) {
				i++
				n, err := strconv.Atoi(args[i])
				if err != nil || n < 1 {
					log.Fatal("-n requires a positive integer")
				}
				pick = n
			}
		case "-h", "-help", "--help":
			fmt.Fprintln(os.Stderr, "Usage: apldocs [-list] [-n N] [-refresh] [query]")
			fmt.Fprintln(os.Stderr, "  -list     List matching titles instead of showing content")
			fmt.Fprintln(os.Stderr, "  -n N      Show the Nth result (default: 1)")
			fmt.Fprintln(os.Stderr, "  -refresh  Download latest docs database")
			os.Exit(0)
		default:
			if query != "" {
				query += " "
			}
			query += args[i]
		}
	}

	if refresh || docs.CacheIsStale() {
		if refresh {
			fmt.Fprintln(os.Stderr, "Refreshing docs cache...")
		}
		if err := docs.RefreshCache(); err != nil {
			log.Fatal(err)
		}
		if query == "" {
			if refresh {
				fmt.Fprintln(os.Stderr, "Cache refreshed.")
			}
			return
		}
	}

	db, err := docs.OpenCache()
	if err != nil {
		log.Fatalf("Open cache: %v\nTry: apldocs -refresh", err)
	}
	defer db.Close()

	if query == "" {
		fmt.Fprintln(os.Stderr, "Usage: apldocs [-list] [-n N] [query]")
		os.Exit(1)
	}

	results := docs.Search(db, query, 50)
	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No matches.")
		os.Exit(1)
	}

	if list {
		for i, r := range results {
			fmt.Printf("%3d  %s\n", i+1, r.Title)
		}
		return
	}

	if pick > len(results) {
		fmt.Fprintf(os.Stderr, "Only %d results, can't show #%d\n", len(results), pick)
		os.Exit(1)
	}

	result := results[pick-1]
	content, err := docs.Content(db, result.Path)
	if err != nil {
		log.Fatal(err)
	}

	// Show title header
	fmt.Printf("# %s\n\n", result.Title)

	// Render markdown for terminal
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(80))
	if err != nil {
		// Fall back to raw markdown
		fmt.Println(content)
		return
	}
	rendered, err := r.Render(content)
	if err != nil {
		fmt.Println(content)
		return
	}
	fmt.Print(rendered)
}
