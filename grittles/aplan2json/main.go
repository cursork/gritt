// aplan2json reads APLAN from stdin and writes JSON to stdout.
//
// Usage:
//
//	echo '(1 2 3)' | aplan2json
//	echo '(1 2 3)' | aplan2json -lossy
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
			fmt.Fprintln(os.Stderr, "Usage: aplan2json [-lossy] < input.aplan")
			fmt.Fprintln(os.Stderr, "  -lossy  Shaped arrays become nested JSON arrays (not round-trippable)")
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

	parsed, err := codec.APLAN(string(input))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse APLAN: %v\n", err)
		os.Exit(1)
	}

	var out any
	if lossy {
		out = toLossy(parsed)
	} else {
		out = codec.ToJSON(parsed)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode JSON: %v\n", err)
		os.Exit(1)
	}
}

// toLossy converts APLAN values to plain JSON without shape metadata.
// Matrices become nested arrays. Not round-trippable.
func toLossy(v any) any {
	switch val := v.(type) {
	case *codec.Array:
		result := make([]any, len(val.Data))
		for i, el := range val.Data {
			result[i] = toLossy(el)
		}
		return result
	case *codec.Namespace:
		m := make(map[string]any, len(val.Keys))
		for _, k := range val.Keys {
			m[k] = toLossy(val.Values[k])
		}
		return m
	case []any:
		result := make([]any, len(val))
		for i, el := range val {
			result[i] = toLossy(el)
		}
		return result
	case complex128:
		return map[string]any{"re": real(val), "im": imag(val)}
	default:
		return v
	}
}
