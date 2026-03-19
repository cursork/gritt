// aplanconv converts between APLAN and JSON.
//
// The input format is auto-detected by peeking at the first non-whitespace
// character. Most cases are unambiguous (( ' ⍬ ¯ → APLAN; { " → JSON).
// For ambiguous cases like [, detection makes a guess and tries to parse;
// if that fails, it tries the other format.
//
// Usage:
//
//	echo '(1 2 3)' | aplanconv                     # APLAN → JSON (auto-detected)
//	echo '[1,2,3]' | aplanconv                     # JSON → APLAN (auto-detected)
//	echo '(1 2 3)' | aplanconv -from aplan -to json
//	aplanconv data.aplan                            # read from file
//	aplanconv -lossy < shaped.aplan                 # nested arrays, no shape metadata
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/cursork/gritt/codec"
)

const usage = `Usage: aplanconv [-from FORMAT] [-to FORMAT] [-lossy] [FILE]

Converts between APLAN and JSON.

Without -from/-to, the format is auto-detected from the input and the
output is the other format.

Flags:
  -from FORMAT   Input format: aplan or json
  -to FORMAT     Output format: aplan or json
  -lossy         APLAN→JSON: shaped arrays become nested arrays (not round-trippable)
                 JSON→APLAN: don't reconstruct shaped arrays from metadata
  FILE           Read from file instead of stdin`

func main() {
	var fromFmt, toFmt string
	var lossy bool
	var file string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-from":
			i++
			if i >= len(args) {
				die("-from requires a format (aplan or json)")
			}
			fromFmt = normalizeFormat(args[i])
			if fromFmt == "" {
				die("unknown format: %s (expected aplan or json)", args[i])
			}
		case "-to":
			i++
			if i >= len(args) {
				die("-to requires a format (aplan or json)")
			}
			toFmt = normalizeFormat(args[i])
			if toFmt == "" {
				die("unknown format: %s (expected aplan or json)", args[i])
			}
		case "-lossy":
			lossy = true
		case "-h", "-help", "--help":
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(0)
		default:
			if strings.HasPrefix(args[i], "-") {
				die("unknown flag: %s", args[i])
			}
			if file != "" {
				die("only one file argument allowed")
			}
			file = args[i]
		}
	}

	input, err := readInput(file)
	if err != nil {
		die("%v", err)
	}

	if fromFmt == "" {
		fromFmt = detect(input)
		if fromFmt == "" {
			die("cannot detect input format (empty input?)")
		}
	}

	if toFmt == "" {
		if fromFmt == "aplan" {
			toFmt = "json"
		} else {
			toFmt = "aplan"
		}
	}

	if fromFmt == toFmt {
		die("-from and -to are both %s; nothing to convert", fromFmt)
	}

	convert(input, fromFmt, toFmt, lossy)
}

// convert tries fromFmt→toFmt. If fromFmt was auto-detected and parsing
// fails, it tries the other direction before giving up.
func convert(input, fromFmt, toFmt string, lossy bool) {
	switch fromFmt + "→" + toFmt {
	case "aplan→json":
		if err := tryAPLANtoJSON(input, lossy); err != nil {
			// Auto-detect guessed wrong? Try the other way.
			if err2 := tryJSONtoAPLAN(input, lossy); err2 != nil {
				die("parse APLAN: %v", err) // report the original guess's error
			}
		}
	case "json→aplan":
		if err := tryJSONtoAPLAN(input, lossy); err != nil {
			if err2 := tryAPLANtoJSON(input, lossy); err2 != nil {
				die("parse JSON: %v", err)
			}
		}
	default:
		die("unsupported conversion: %s → %s", fromFmt, toFmt)
	}
}

func readInput(file string) (string, error) {
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", file, err)
		}
		return string(data), nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(data), nil
}

// detect peeks at the first non-whitespace character to make a best guess.
// Unambiguous cases return confidently. Ambiguous cases (like [) guess,
// and convert() will try the other format if parsing fails.
func detect(input string) string {
	s := strings.TrimLeftFunc(input, unicode.IsSpace)
	if s == "" {
		return ""
	}
	r := rune(s[0])
	if r >= 0x80 {
		// Multi-byte: could be ⍬, ¯, etc. — APLAN
		return "aplan"
	}
	switch r {
	case '(', '\'':
		return "aplan"
	case '{', '"':
		return "json"
	case '[':
		return "json" // guess JSON; convert() will try APLAN if it fails
	}
	if strings.HasPrefix(s, "true") || strings.HasPrefix(s, "false") || strings.HasPrefix(s, "null") {
		return "json"
	}
	if (r >= '0' && r <= '9') || r == '-' {
		return "json"
	}
	return ""
}

func normalizeFormat(s string) string {
	switch strings.ToLower(s) {
	case "aplan", "apl":
		return "aplan"
	case "json":
		return "json"
	}
	return ""
}

func tryAPLANtoJSON(input string, lossy bool) error {
	parsed, err := codec.APLAN(input)
	if err != nil {
		return err
	}

	var out any
	if lossy {
		out = toLossy(parsed)
	} else {
		out = codec.ToJSON(parsed)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func tryJSONtoAPLAN(input string, lossy bool) error {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return err
	}

	aplanVal := codec.FromJSON(v, lossy)
	fmt.Println(codec.Serialize(aplanVal))
	return nil
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

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "aplanconv: "+format+"\n", args...)
	os.Exit(1)
}
