// apldcf inspects Dyalog Component Files (.dcf) without a Dyalog
// dependency.
//
// Usage:
//
//	apldcf path.dcf            # list components
//	apldcf -n 3 path.dcf       # dump component 3 as APLAN
//	apldcf -all path.dcf       # dump every component as APLAN
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cursork/gritt/codec"
	"github.com/cursork/gritt/dcf"
)

func main() {
	var (
		dumpN = flag.Int("n", 0, "dump component N as APLAN")
		all   = flag.Bool("all", false, "dump every component as APLAN")
	)
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: apldcf [-n N | -all] path.dcf")
		os.Exit(2)
	}

	f, err := dcf.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer f.Close()

	h := f.Header()
	cs := f.Components()

	switch {
	case *dumpN > 0:
		dump(f, *dumpN)
	case *all:
		for _, c := range cs {
			fmt.Printf("--- component %d ---\n", c.Number)
			dump(f, c.Number)
		}
	default:
		fmt.Printf("file:      %s\n", flag.Arg(0))
		fmt.Printf("version:   0x%X\n", h.Version)
		fmt.Printf("ptr_size:  %d\n", h.PtrSize)
		fmt.Printf("created:   %s\n", h.CreatedAt.Format("2006-01-02 15:04:05 MST"))
		expected := int(h.NextFree) - int(h.FirstUsed)
		report := f.ScanReport()
		if len(cs) == expected {
			fmt.Printf("components: %d (numbers %d..%d)\n", len(cs), h.FirstUsed, h.NextFree-1)
		} else {
			fmt.Printf("components: %d found, %d expected (slots %d..%d)\n",
				len(cs), expected, h.FirstUsed, h.NextFree-1)
			fmt.Printf("\n  scan diagnostics:\n")
			fmt.Printf("    %-4d per-component magic patterns found in file\n", report.MagicHits)
			fmt.Printf("    %-4d parsed as valid component headers\n", report.MagicHits-report.ParseFailures)
			fmt.Printf("    %-4d collapsed as journal/backup duplicates\n", report.Deduplicated)
			fmt.Printf("    %-4d rejected (header could not be parsed; bad shape or wrong layout)\n", report.ParseFailures)
			fmt.Printf("    magic variant: %02X %02X %02X %02X",
				report.MagicVariant[0], report.MagicVariant[1],
				report.MagicVariant[2], report.MagicVariant[3])
			if report.IsKnownVariant {
				fmt.Printf("  (v20 — supported)\n")
			} else {
				fmt.Printf("  (NOT v20)\n")
				fmt.Printf("\n  WARNING: this file uses an older Dyalog on-disk format. The parser\n")
				fmt.Printf("  was reverse-engineered against v20 only; some components may have\n")
				fmt.Printf("  been skipped because their internal layout differs.\n")
			}
		}
		fmt.Println()
		fmt.Printf("  #   ⎕DR  rank  shape\n")
		for _, c := range cs {
			fmt.Printf("  %-4d %-4d %-5d %v\n", c.Number, c.DR, c.Rank, c.Shape)
		}
	}
}

func dump(f *dcf.File, n int) {
	v, err := f.Read(n)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read component %d: %v\n", n, err)
		os.Exit(1)
	}
	fmt.Println(codec.Serialize(v))
}
