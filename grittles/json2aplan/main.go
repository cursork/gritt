// json2aplan reads JSON from stdin and writes APLAN to stdout.
//
// Usage:
//
//	echo '[1,2,3]' | json2aplan
//	echo '{"a":1}' | json2aplan
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cursork/gritt/codec"
)

func main() {
	lossy := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-lossy":
			lossy = true
		case "-h", "-help", "--help":
			fmt.Fprintln(os.Stderr, "Usage: json2aplan [-lossy] < input.json")
			fmt.Fprintln(os.Stderr, "  -lossy  Don't reconstruct shaped arrays from type metadata")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			os.Exit(1)
		}
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}

	var v any
	if err := json.Unmarshal(input, &v); err != nil {
		fmt.Fprintf(os.Stderr, "parse JSON: %v\n", err)
		os.Exit(1)
	}

	aplanVal := codec.FromJSON(v, lossy)
	fmt.Println(codec.Serialize(aplanVal))
}
