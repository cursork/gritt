// aplfmt formats APL source files using a Dyalog interpreter.
//
// Usage:
//
//	aplfmt file.aplf                    # format in place (auto-launch Dyalog)
//	aplfmt -addr localhost:4502 *.aplf  # use running Dyalog
//	aplfmt -version 20.0 src/          # specific version
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cursork/gritt/session"
)

func main() {
	addr := flag.String("addr", "", "Connect to running Dyalog at host:port (skips launch)")
	version := flag.String("version", "", "Dyalog version (e.g. 20.0) or path to binary")
	flag.Parse()

	files := collectFiles(flag.Args())
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: aplfmt [-addr host:port] [-version X.Y] file.aplf ...")
		os.Exit(1)
	}

	ctx := context.Background()
	var sess *session.Session
	var err error

	if *addr != "" {
		sess, err = session.Connect(ctx, session.ConnectOptions{Addr: *addr})
	} else {
		sess, err = session.Launch(ctx, session.LaunchOptions{Version: *version})
	}
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Close()

	if err := sess.Format(ctx, files...); err != nil {
		log.Fatal(err)
	}
}

// collectFiles expands directories to APL source files.
func collectFiles(args []string) []string {
	aplExts := map[string]bool{".aplf": true, ".aplo": true, ".apln": true, ".aplc": true, ".apli": true}
	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			log.Printf("warning: %s: %v", arg, err)
			continue
		}
		if info.IsDir() {
			filepath.Walk(arg, func(path string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return err
				}
				if aplExts[strings.ToLower(filepath.Ext(path))] {
					files = append(files, path)
				}
				return nil
			})
		} else {
			files = append(files, arg)
		}
	}
	return files
}
