// apldocs searches Dyalog documentation from the terminal.
//
// Usage:
//
//	apldocs "each operator"
//	apldocs -refresh
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cursork/gritt/docs"
)

func main() {
	refresh := false
	var query string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-refresh":
			refresh = true
		case "-h", "-help", "--help":
			fmt.Fprintln(os.Stderr, "Usage: apldocs [-refresh] [query]")
			os.Exit(0)
		default:
			if query != "" {
				query += " "
			}
			query += arg
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
			fmt.Fprintln(os.Stderr, "Cache refreshed.")
			return
		}
	}

	db, err := docs.OpenCache()
	if err != nil {
		log.Fatalf("Open cache: %v\nTry: apldocs -refresh", err)
	}
	defer db.Close()

	if query == "" {
		fmt.Fprintln(os.Stderr, "Provide a query to search.")
		return
	}

	results := docs.Search(db, query, 20)
	for _, r := range results {
		fmt.Printf("%-50s %s\n", r.Title, r.Path)
	}
	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No matches.")
	}
}
