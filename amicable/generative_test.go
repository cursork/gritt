package amicable

import (
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/cursork/gritt/codec"
)

// Generative round-trip tests (FACIENDA "generative round-trip tests").
//
// Rather than hand-writing every namespace/array shape, these build random
// structures, serialize them in Dyalog with 1(220⌶), and check the Go side.
// The point is to fuzz the fragile surfaces without enumerating cases:
//
//   - Namespaces: unmarshal correctness. Members mix class 2 (variables),
//     class 3 (functions) and class 9 (nested namespaces) in random orders,
//     counts and depths — exactly the interleaving that stresses the
//     sequential-vs-relocated-tail layout logic in unmarshalNamespace. Full
//     re-marshal isn't possible (writeArray has no *codec.Namespace case), so
//     the check is structural: the parsed tree must match the generated spec.
//
//   - Arrays: full round-trip. 1(220⌶) → Unmarshal → Marshal → 0(220⌶), with
//     Dyalog's ≡ confirming value identity across random shapes and fan-out.
//
// Seeds are fixed so any failure reproduces exactly (and prints the offending
// APL expression). Bump the seed to explore more shapes.

// genKind enumerates the member classes a generated namespace can hold.
type genKind int

const (
	gInt genKind = iota // class 2 variable, integer scalar
	gFloat              // class 2 variable, float scalar
	gStr                // class 2 variable, character vector
	gFn                 // class 3 function (dfn) → opaque Raw on unmarshal
	gNs                 // class 9 nested namespace → recurse
)

type genMember struct {
	name string
	kind genKind
	ival int
	fval float64
	sval string
	sub  *genNs
}

type genNs struct {
	members []genMember
}

func genNamespace(rng *rand.Rand, depth, maxDepth int) *genNs {
	n := 1 + rng.Intn(5) // 1..5 members
	ns := &genNs{}
	for i := 0; i < n; i++ {
		m := genMember{name: fmt.Sprintf("m%d", i)}
		// Leaf kinds are always available; nesting only within the budget.
		choices := []genKind{gInt, gInt, gFloat, gStr, gStr, gFn}
		if depth < maxDepth {
			choices = append(choices, gNs, gNs)
		}
		m.kind = choices[rng.Intn(len(choices))]
		switch m.kind {
		case gInt:
			m.ival = rng.Intn(4001) - 2000 // spans Int8 and Int16 widths
		case gFloat:
			m.fval = genFloat(rng)
		case gStr:
			m.sval = genChars(rng, 1+rng.Intn(6))
		case gFn:
			// concrete dfn rendered below; unmarshals to opaque Raw
		case gNs:
			m.sub = genNamespace(rng, depth+1, maxDepth)
		}
		ns.members = append(ns.members, m)
	}
	return ns
}

// genFloat returns a non-integer value of the form k/8, which is exactly
// representable as a float64 and whose shortest decimal parses back exactly in
// both Go and Dyalog. Avoiding whole numbers keeps Dyalog from storing the
// value as an integer (which would come back as Go int, not float64).
func genFloat(rng *rand.Rand) float64 {
	k := rng.Intn(8001) - 4000
	if k%8 == 0 {
		k++
	}
	return float64(k) / 8.0
}

func genChars(rng *rand.Rand, n int) string {
	const alpha = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = alpha[rng.Intn(len(alpha))]
	}
	return string(b)
}

// renderNs builds the namespace with flat dotted-path assignments (the same
// construction the hand-written nested tests use), wrapped in a dfn that
// returns the root so 1(220⌶)<expr> serializes it inline.
func renderNs(spec *genNs, root string) string {
	var stmts []string
	stmts = append(stmts, root+"←⎕NS''")
	var walk func(prefix string, ns *genNs)
	walk = func(prefix string, ns *genNs) {
		for _, m := range ns.members {
			path := prefix + "." + m.name
			switch m.kind {
			case gInt:
				stmts = append(stmts, path+"←"+aplInt(m.ival))
			case gFloat:
				stmts = append(stmts, path+"←"+aplFloat(m.fval))
			case gStr:
				stmts = append(stmts, path+"←'"+m.sval+"'")
			case gFn:
				stmts = append(stmts, path+"←{⍵+1}")
			case gNs:
				stmts = append(stmts, path+"←⎕NS''")
				walk(path, m.sub)
			}
		}
	}
	walk(root, spec)
	return "{" + strings.Join(stmts, " ⋄ ") + " ⋄ " + root + "}⍬"
}

func aplInt(n int) string {
	if n < 0 {
		return "¯" + strconv.Itoa(-n)
	}
	return strconv.Itoa(n)
}

func aplFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if strings.HasPrefix(s, "-") {
		return "¯" + s[1:]
	}
	return s
}

func TestGenerativeNamespaceRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	const (
		seed     = 0x9e3779b9
		numCases = 40
		maxDepth = 3
	)
	t.Logf("seed=%#x cases=%d maxDepth=%d (bump seed to explore more shapes)", seed, numCases, maxDepth)
	rng := rand.New(rand.NewSource(seed))

	specs := make([]*genNs, numCases)
	exprs := make([]string, numCases)
	args := []string{"-l"}
	for i := range specs {
		specs[i] = genNamespace(rng, 0, maxDepth)
		exprs[i] = renderNs(specs[i], "n")
		args = append(args, "-e", fmt.Sprintf("'=%d=' ⋄ 1(220⌶)%s", i, exprs[i]))
	}

	blobs := parseDelimitedBlobs(t, args, numCases)

	for i, blob := range blobs {
		i, blob := i, blob
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("case %d: Unmarshal panicked: %v\n  expr: %s", i, r, exprs[i])
				}
			}()
			v, err := Unmarshal(blob)
			if err != nil {
				t.Errorf("case %d: Unmarshal: %v\n  expr: %s", i, err, exprs[i])
				return
			}
			ns, ok := v.(*codec.Namespace)
			if !ok {
				t.Errorf("case %d: Unmarshal = %T, want *codec.Namespace\n  expr: %s", i, v, exprs[i])
				return
			}
			checkNs(t, fmt.Sprintf("case%d", i), exprs[i], specs[i], ns)
		}()
	}
}

// checkNs recursively asserts that a parsed namespace matches its spec: every
// member present, correct value, correct Go type, and no extra members.
func checkNs(t *testing.T, path, expr string, spec *genNs, got *codec.Namespace) {
	t.Helper()
	if len(got.Keys) != len(spec.members) {
		t.Errorf("%s: got %d keys %v, want %d\n  expr: %s", path, len(got.Keys), got.Keys, len(spec.members), expr)
	}
	for _, m := range spec.members {
		v, ok := got.Values[m.name]
		if !ok {
			t.Errorf("%s.%s: missing member\n  expr: %s", path, m.name, expr)
			continue
		}
		switch m.kind {
		case gInt:
			if iv, ok := v.(int); !ok || iv != m.ival {
				t.Errorf("%s.%s = %v (%T), want int %d\n  expr: %s", path, m.name, v, v, m.ival, expr)
			}
		case gFloat:
			if fv, ok := v.(float64); !ok || fv != m.fval {
				t.Errorf("%s.%s = %v (%T), want float64 %v\n  expr: %s", path, m.name, v, v, m.fval, expr)
			}
		case gStr:
			if sv, ok := v.(string); !ok || sv != m.sval {
				t.Errorf("%s.%s = %v (%T), want string %q\n  expr: %s", path, m.name, v, v, m.sval, expr)
			}
		case gFn:
			if _, ok := v.(Raw); !ok {
				t.Errorf("%s.%s = %T, want Raw (function)\n  expr: %s", path, m.name, v, expr)
			}
		case gNs:
			sub, ok := v.(*codec.Namespace)
			if !ok {
				t.Errorf("%s.%s = %T, want *codec.Namespace\n  expr: %s", path, m.name, v, expr)
				continue
			}
			checkNs(t, path+"."+m.name, expr, m.sub, sub)
		}
	}
}

func TestGenerativeArrayRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("gritt"); err != nil {
		t.Skip("gritt not on PATH")
	}

	const (
		seed     = 0x2545f491
		numCases = 40
	)
	t.Logf("seed=%#x cases=%d (bump seed to explore more shapes)", seed, numCases)
	rng := rand.New(rand.NewSource(seed))

	exprs := make([]string, numCases)
	args := []string{"-l"}
	for i := range exprs {
		exprs[i] = genArrayExpr(rng)
		args = append(args, "-e", fmt.Sprintf("'=%d=' ⋄ 1(220⌶)%s", i, exprs[i]))
	}
	blobs := parseDelimitedBlobs(t, args, numCases)

	// Unmarshal + re-marshal each, then let Dyalog confirm value identity.
	verArgs := []string{"-l"}
	verCases := []int{}
	for i, blob := range blobs {
		v, err := Unmarshal(blob)
		if err != nil {
			t.Errorf("case %d: Unmarshal: %v\n  expr: %s", i, err, exprs[i])
			continue
		}
		reser, err := Marshal(v)
		if err != nil {
			t.Errorf("case %d: Marshal: %v\n  expr: %s", i, err, exprs[i])
			continue
		}
		verArgs = append(verArgs, "-e", fmt.Sprintf("'=%d=' , ⍕ (%s) ≡ 0(220⌶) %s", i, exprs[i], formatAsAPLVector(reser)))
		verCases = append(verCases, i)
	}

	verOut, err := runGritt(verArgs...)
	if err != nil {
		t.Fatalf("verification gritt failed: %v", err)
	}
	for _, i := range verCases {
		delim := fmt.Sprintf("=%d=", i)
		idx := strings.Index(verOut, delim)
		if idx < 0 {
			t.Errorf("case %d: result marker %q not found", i, delim)
			continue
		}
		rest := strings.TrimSpace(verOut[idx+len(delim):])
		result := "1"
		if len(rest) > 0 {
			result = string(rest[0]) // ⍕ catenates the boolean flush against the marker
		}
		if result != "1" {
			t.Errorf("case %d: Dyalog says not ≡ (got %q)\n  expr: %s", i, result, exprs[i])
		}
	}
}

// genArrayExpr generates a random simple (non-nested) array: random rank 0..3,
// random element type, random non-empty shape. Nested and empty arrays are
// already covered by the hand-written E2E cases.
func genArrayExpr(rng *rand.Rand) string {
	rank := rng.Intn(4) // 0..3
	kind := rng.Intn(4) // 0 int, 1 float, 2 bool, 3 char

	if rank == 0 {
		return genArrayScalar(rng, kind)
	}

	shape := make([]int, rank)
	count := 1
	for i := range shape {
		shape[i] = 1 + rng.Intn(4) // 1..4, non-empty
		count *= shape[i]
	}
	shapeStr := make([]string, rank)
	for i, d := range shape {
		shapeStr[i] = strconv.Itoa(d)
	}
	prefix := strings.Join(shapeStr, " ") + "⍴"

	if kind == 3 { // char array
		return prefix + "'" + genChars(rng, count) + "'"
	}
	vals := make([]string, count)
	for i := range vals {
		vals[i] = genArrayScalar(rng, kind)
	}
	return prefix + strings.Join(vals, " ")
}

func genArrayScalar(rng *rand.Rand, kind int) string {
	switch kind {
	case 0: // int
		return aplInt(rng.Intn(4001) - 2000)
	case 1: // float
		return aplFloat(genFloat(rng))
	case 2: // bool
		return strconv.Itoa(rng.Intn(2))
	default: // char
		return "'" + genChars(rng, 1) + "'"
	}
}
