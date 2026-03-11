package codec_test

import (
	"fmt"

	"github.com/cursork/gritt/codec"
)

func ExampleSerialize() {
	examples := []struct {
		label string
		aplan string
	}{
		// Scalars
		{"Integer", "42"},
		{"Negative integer", "¯5"},
		{"Float", "3.14"},
		{"Negative float", "¯2.5"},
		{"Exponential", "1E5"},
		{"Negative exponent", "2.5E¯3"},
		{"Complex", "3J4"},
		{"Complex negative", "¯2J¯3"},

		// Strings
		{"String", "'hello world'"},
		{"Empty string", "''"},
		{"Escaped quotes", "'it''s'"},

		// Zilde
		{"Zilde (empty vector)", "⍬"},

		// Strands (space-separated)
		{"Number strand", "1 2 3"},
		{"Negative strand", "¯1 0 1"},
		{"Mixed strand", "1 2.5 3"},

		// Vectors (parenthesised with separators)
		{"Vector (diamond)", "(1 ⋄ 2 ⋄ 3)"},
		{"Vector (newline)", "(1\n2\n3)"},
		{"Mixed vector", "(1 ⋄ 'two' ⋄ 3)"},
		{"Nested vector", "((1 ⋄ 2) ⋄ (3 ⋄ 4))"},
		{"Char vector collapse", "('a' ⋄ 'b' ⋄ 'c')"},
		{"Multi-char strings", "('ab' ⋄ 'cd')"},
		{"1-element vector (leading sep)", "(⋄ 42)"},
		{"1-element vector (trailing sep)", "(42 ⋄)"},
		{"Grouping (no sep)", "(42)"},
		{"Vector of strands", "(1 2 ⋄ 3 4)"},

		// Matrices
		{"2×2 matrix", "[1 2 ⋄ 3 4]"},
		{"Column vector (3×1)", "[1 ⋄ 2 ⋄ 3]"},
		{"Padded matrix (2×3)", "[1 2 ⋄ 3 4 5]"},
		{"Bracket stranding", "[1 2]"},
		{"String matrix", "['a' 'b' ⋄ 'c' 'd']"},
		{"3D array", "[[1 2 ⋄ 3 4] ⋄ [5 6 ⋄ 7 8]]"},

		// Namespaces
		{"Empty namespace", "()"},
		{"Simple namespace", "(x: 1 ⋄ y: 2)"},
		{"Namespace with string", "(name: 'John')"},
		{"Namespace with vector", "(data: (1 ⋄ 2 ⋄ 3))"},
		{"Nested namespace", "(outer: (inner: 42))"},
		{"Namespace with matrix", "(name: 'data' ⋄ matrix: [1 2 ⋄ 3 4])"},

		// Complex nesting
		{"Vector of matrices", "([1 2 ⋄ 3 4] ⋄ [5 6 ⋄ 7 8])"},
		{"Vector of namespaces", "((x: 1) ⋄ (y: 2))"},
		{"Deeply nested", "(((1 ⋄ 2) ⋄ (3 ⋄ 4)) ⋄ ((5 ⋄ 6) ⋄ (7 ⋄ 8)))"},
	}

	for _, ex := range examples {
		parsed, err := codec.APLAN(ex.aplan)
		if err != nil {
			fmt.Printf("%-35s ERROR: %v\n", ex.label, err)
			continue
		}
		serialized := codec.Serialize(parsed, codec.SerializeOptions{UseDiamond: true})
		fmt.Printf("%-35s %s\n", ex.label, serialized)
	}

	// Output:
	// Integer                             42
	// Negative integer                    ¯5
	// Float                               3.14
	// Negative float                      ¯2.5
	// Exponential                         100000
	// Negative exponent                   0.0025
	// Complex                             3J4
	// Complex negative                    ¯2J¯3
	// String                              'hello world'
	// Empty string                        ''
	// Escaped quotes                      'it''s'
	// Zilde (empty vector)                ⍬
	// Number strand                       1 2 3
	// Negative strand                     ¯1 0 1
	// Mixed strand                        1 2.5 3
	// Vector (diamond)                    1 2 3
	// Vector (newline)                    1 2 3
	// Mixed vector                        (1 ⋄ 'two' ⋄ 3)
	// Nested vector                       (1 2 ⋄ 3 4)
	// Char vector collapse                'abc'
	// Multi-char strings                  ('ab' ⋄ 'cd')
	// 1-element vector (leading sep)      (⋄ 42)
	// 1-element vector (trailing sep)     (⋄ 42)
	// Grouping (no sep)                   42
	// Vector of strands                   (1 2 ⋄ 3 4)
	// 2×2 matrix                          [1 2 ⋄ 3 4]
	// Column vector (3×1)                 [1 ⋄ 2 ⋄ 3]
	// Padded matrix (2×3)                 [1 2 0 ⋄ 3 4 5]
	// Bracket stranding                   [1 ⋄ 2]
	// String matrix                       ['a' 'b' ⋄ 'c' 'd']
	// 3D array                            [[1 2 ⋄ 3 4] ⋄ [5 6 ⋄ 7 8]]
	// Empty namespace                     ()
	// Simple namespace                    (x: 1 ⋄ y: 2)
	// Namespace with string               (name: 'John')
	// Namespace with vector               (data: 1 2 3)
	// Nested namespace                    (outer: (inner: 42))
	// Namespace with matrix               (name: 'data' ⋄ matrix: [1 2 ⋄ 3 4])
	// Vector of matrices                  ([1 2 ⋄ 3 4] ⋄ [5 6 ⋄ 7 8])
	// Vector of namespaces                ((x: 1) ⋄ (y: 2))
	// Deeply nested                       ((1 2 ⋄ 3 4) ⋄ (5 6 ⋄ 7 8))
}
