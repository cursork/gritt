// aplor decodes Dyalog 220⌶ binary blobs.
//
// Function ⎕OR blobs decompile back to APL source; plain arrays and
// namespaces round-trip to APLAN.
//
// Usage:
//
//	# From Dyalog: serialize a function and pipe to aplor
//	gritt -l -e "1(220⌶)⎕OR'myfn'" | aplor
//
//	# Recover values from aplsock running in aplor mode
//	printf '%s\n' '⍳5' "'hello world'" '2 3⍴⍳6' | nc localhost 4212 | aplor -stream
//
//	# From a binary file
//	aplor -raw saved.220
//
//	# From a file of signed integers (space-separated, ¯ for negative)
//	aplor dump.txt
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cursork/gritt/amicable"
	"github.com/cursork/gritt/codec"
)

const usage = `Usage: aplor [-raw] [-stream] [FILE]

Decode Dyalog 220⌶ binary blobs. Function ⎕OR blobs are decompiled to
APL source; plain arrays and namespaces are recovered as APLAN.

Input is 220⌶ output: signed integers (-128..127) separated by spaces,
as produced by "1(220⌶)⎕OR'name'" in Dyalog or by aplsock's aplor mode.
Reads from FILE or stdin.

Flags:
  -raw     Input is raw binary bytes (not text integers)
  -stream  Input contains multiple blobs, one per line`

func main() {
	raw := false
	stream := false
	var filename string

	for _, arg := range os.Args[1:] {
		switch arg {
		case "-h", "-help", "--help":
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(0)
		case "-raw":
			raw = true
		case "-stream":
			stream = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
				os.Exit(1)
			}
			filename = arg
		}
	}

	if raw && stream {
		fmt.Fprintln(os.Stderr, "-raw and -stream are mutually exclusive")
		os.Exit(1)
	}

	var reader io.Reader = os.Stdin
	if filename != "" {
		f, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		reader = f
	}

	if stream {
		scanner := bufio.NewScanner(reader)
		// Responses from aplsock aplor mode can be quite large (namespaces
		// carry a full atoms table); allow lines up to 16 MB.
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			data, err := parseSignedInts(line)
			if err != nil {
				fmt.Fprintf(os.Stderr, "parse: %v\n", err)
				os.Exit(1)
			}
			if err := decodeAndPrint(data); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "read: %v\n", err)
			os.Exit(1)
		}
		return
	}

	input, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}

	var data []byte
	if raw {
		data = input
	} else {
		data, err = parseSignedInts(string(input))
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse: %v\n", err)
			os.Exit(1)
		}
	}

	if err := decodeAndPrint(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// decodeAndPrint unmarshals a 220⌶ blob and prints the result.
//
// Top-level function blobs come back as amicable.Raw and go through
// Decompile() to APL source. Everything else — plain arrays, namespaces,
// namespaces containing functions — goes through codec.Serialize. amicable
// decompiles embedded function members inline during unmarshal, so the
// serialize path renders them as raw APL source inside the namespace.
func decodeAndPrint(data []byte) error {
	val, err := amicable.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	if r, ok := val.(amicable.Raw); ok {
		src, err := r.Decompile()
		if err != nil {
			return fmt.Errorf("decompile: %w", err)
		}
		fmt.Println(src)
		return nil
	}
	fmt.Println(codec.Serialize(val, codec.SerializeOptions{UseDiamond: true}))
	return nil
}

func parseSignedInts(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "¯", "-")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	data := make([]byte, len(fields))
	for i, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("byte %d: %w", i, err)
		}
		if v < -128 || v > 127 {
			return nil, fmt.Errorf("byte %d: value %d out of range", i, v)
		}
		data[i] = byte(int8(v))
	}
	return data, nil
}
