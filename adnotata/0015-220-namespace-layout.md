# 0015 — 220⌶ namespace layout (nested-member reverse engineering)

Reverse-engineered while fixing the namespace round-trip bug that the
generative test (`amicable/generative_test.go`) surfaced: 22 of 40 random
namespace shapes failed to unmarshal. Root cause was in
`amicable.unmarshalNamespace` — it handed each nested namespace *all trailing
parent bytes* (`Raw(data[nsStart:])`) and re-ran the standalone parser, and it
used a coarse two-branch (`relocated := reversed[0].class == 9`) heuristic that
collapses once class-2/3/9 members mix.

Tooling: `cmd/nsdump` prints a landmark map of a captured blob (name-table
entries, `07..D5 50` block starts classified by their sub-array typeRank,
bytecode `FF FF`). Capture blobs with a PID-disciplined `gritt -l`:
`1(220⌶){n←⎕NS'' ⋄ … ⋄ n}⍬`, one per `=NAME=` delimiter, then
`go run ./cmd/nsdump <file> <NAME>`.

## Overall shape (64-bit)

Fields are ptrSize(8)-aligned starting at **offset 2** (after the 2-byte
`DF A4` magic). A namespace ⎕OR is opaque: typeRank `00 00`, discriminator
byte `0x22` has high nibble `0xA0` (vs `0x20` for functions — see
`220-SPEC.md` §5.7).

```
NS-HEADER (07..D5 50, sub-array typeRank 05 55)
bytecode (FF FF …)          ⍝ the namespace's own script/constructor
name table                  ⍝ 01 XX 00 88 00 00 00 00 + UTF-16LE name
  XX high nibble = name class:  0x28 → nc 2 (var)   0x98 → nc 9 (namespace)
                                0x38 → nc 3 (fn)    0x08 → the ns's own name
  first entry is a 0xFFFF sentinel, skipped
member values               ⍝ see "member values" below
root-only tail:
  settings / translation blocks (07..D5 50, typeRank 17 06)
  the giant (~1400 byte) translation table   ⍝ constant, root carries it ONCE
  TERMINATOR (07..D5 50, typeRank 00 00)
```

## Member values — the part the old code got wrong

Values are laid out in **reverse name-table order**. Class matters:

- **class-2 var**: a standard sub-array (size + typeRank + shape + data),
  read/extent via the normal `readArray` path.
- **class-3 fn**: an embedded function blob (`07..D5 50`, typeRank `45 51`
  "EQ"), extent via `skipFnBlob`.
- **class-9 nested namespace**: a **compact, self-contained sub-blob** —
  `05 55` header + bytecode + name table + its own member values — and
  **no** translation-table/terminator tail. The tail belongs to the root
  only. (This is why one nested ns adds ~180 bytes, not ~1400: case A, 2
  plain vars = 2082 B; case B, one nested ns = 2266 B.)

### The split-around-the-table rule (the multi-class-9 trap)

With several class-9 members, Dyalog **splits them around the translation
table**:

- the **first** class-9 value is inlined right after the name table;
- **subsequent** class-9 values (and relocated class-2/3 values) are placed
  in a tail region **after** the entire root translation table, before the
  terminators.

Case D (`n.a←⎕NS'' ⋄ n.a.x←1 ⋄ n.b←⎕NS'' ⋄ n.b.y←2`, landmark offsets):

```
194 NAME nc9 "b"     218 NAME nc9 "a"       ⍝ root name table [b,a]
242 NS-HEADER …                              ⍝ a's nested ns (inlined)
434/674/746 meta (17 06) + translation table ⍝ ROOT tail
2218 NS-HEADER … 2354 NAME "y"               ⍝ b's nested ns — AFTER the table
2410 / 2482 TERMINATOR                        ⍝ root terminators
```

The old parser never looked past the translation table for a member, so the
second sibling (and mixed class-2/3 tails) vanished.

## Fix direction

Recursive descent where **every member reader returns its exact byte extent**,
so the parent advances by true sizes and each child gets an exact slice
instead of "all trailing bytes":

- `readArray` → already returns extent.
- `skipFnBlob` → already computes fn-blob extent.
- **new**: parse a nested namespace as a bounded sub-blob (header + bytecode +
  name table + its values, recursively) and return where it ends — stopping
  before the parent's translation tail, not consuming it.
- model the split-around-the-table placement for 2nd+ class-9 and relocated
  class-2/3 members explicitly, rather than the `reversed[0].class == 9`
  binary switch.

Open (needs more captured examples before committing to `220-SPEC.md` §5.7):
exact extent terminator of a compact nested-ns sub-blob; ordering rules when
class-2/3 and multiple class-9 interleave at one level; 9.2/9.4/9.5/9.6
(instances/classes/interfaces/external) which share the code path with
different blob shapes.

## Attempt log

**Attempt 1 (reverted).** A big-bang `parseNamespaceAt` — recursive descent
that walks from the name table collecting `len(members)` value-items (bare
sub-array = var, `05 55` block = nested ns → recurse, `45 51` block = fn,
`17 06`/`00 00` blocks = structural → skip by extent), then assigns
`values[i] → members[len-1-i]`. It **regressed 18→16 / 40** and was reverted
(`git checkout amicable/amicable.go`).

Why it failed: the assignment assumed **file order == reverse name-table
order**. That holds for all-var (case A) and all-nested (case D), but NOT for
mixed classes (case E: `[int m0, fn m1, nested m2]`). Values shifted by one and
types landed on the wrong members (`m2 = int, want *codec.Namespace`; `m3 =
[]interface{}, want Raw`). So the placement of a value depends on its class,
not just position.

**The blocking unknown:** the exact rule mapping (member index, class) → value
byte-location under the split-around-the-table layout. RE it from case E
(`nsdump` the captured `=E=` blob) — map each of m0(int)/m1(fn)/m2(nested)'s
value to its offset — before writing any parser. The recursive-descent
architecture is right; the ordering model is what was wrong. Build it
incrementally (small edits), not as one 156-line swap.
