// aplcart searches APLcart entries from the terminal.
//
// Usage:
//
//	aplcart "matrix inverse"
//	aplcart -refresh
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cursork/gritt/aplcart"
)

func main() {
	refresh := false
	var query string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-refresh":
			refresh = true
		case "-h", "-help", "--help":
			fmt.Fprintln(os.Stderr, "Usage: aplcart [-refresh] [query]")
			os.Exit(0)
		default:
			if query != "" {
				query += " "
			}
			query += arg
		}
	}

	if refresh || aplcart.CacheIsStale() {
		if refresh {
			fmt.Fprintln(os.Stderr, "Refreshing APLcart cache...")
		}
		if _, err := aplcart.RefreshCache(); err != nil {
			log.Fatal(err)
		}
		if query == "" {
			return
		}
	}

	entries, err := aplcart.LoadCache()
	if err != nil {
		log.Fatalf("Load cache: %v\nTry: aplcart -refresh", err)
	}

	if query == "" {
		fmt.Fprintf(os.Stderr, "%d entries loaded. Provide a query to search.\n", len(entries))
		return
	}

	results := aplcart.Search(entries, query)
	for _, e := range results {
		fmt.Printf("%-30s %s\n", e.Syntax, e.Description)
	}
	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No matches.")
	}
}
