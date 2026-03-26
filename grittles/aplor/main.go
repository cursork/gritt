// aplor decompiles Dyalog ⎕OR binary blobs back to APL source.
//
// It reads 220⌶-serialized bytes (as signed integers from stdin or a file,
// or raw binary with -raw) and reconstructs the original dfn or tradfn source.
//
// Usage:
//
//	# From Dyalog: serialize a function and pipe to aplor
//	gritt -l -e "1(220⌶)⎕OR'myfn'" | aplor
//
//	# From a binary file
//	aplor -raw saved.220
//
//	# From a file of signed integers (space-separated, ¯ for negative)
//	aplor dump.txt
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cursork/gritt/amicable"
)

const usage = `Usage: aplor [-raw] [FILE]

Decompile Dyalog ⎕OR blobs to APL source.

Input is 220⌶ output: signed integers (-128..127) separated by spaces,
as produced by "1(220⌶)⎕OR'name'" in Dyalog. Reads from FILE or stdin.

Flags:
  -raw    Input is raw binary bytes (not text integers)`

func main() {
	raw := false
	var filename string

	for _, arg := range os.Args[1:] {
		switch arg {
		case "-h", "-help", "--help":
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(0)
		case "-raw":
			raw = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
				os.Exit(1)
			}
			filename = arg
		}
	}

	var input []byte
	var err error

	if filename != "" {
		input, err = os.ReadFile(filename)
	} else {
		input, err = io.ReadAll(os.Stdin)
	}
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

	val, err := amicable.Unmarshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal: %v\n", err)
		os.Exit(1)
	}

	r, ok := val.(amicable.Raw)
	if !ok {
		fmt.Fprintf(os.Stderr, "not an ⎕OR blob (got %T — this is a plain array, not a function)\n", val)
		os.Exit(1)
	}

	src, err := r.Decompile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "decompile: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(src)
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
