# 220⌶ Serialise/Deserialise Array — Binary Format Specification

**Status:** Reverse-engineered from Dyalog APL v20.0 (64-bit, macOS ARM64).
Verified by round-tripping through live Dyalog sessions.

**Accuracy:** Everything stated here has been tested. Gaps are marked explicitly.
Nothing is speculative.

---

## 1. Overview

`1(220⌶)Y` serializes any APL array `Y` into a simple integer vector with
values in the range −128 to 127 (type 83, "sint_vector"). `0(220⌶)Y`
deserializes it back. The format is portable across Dyalog interpreter widths
and editions.

The serialized form is a byte stream. APL represents it as signed integers;
the unsigned byte interpretation is used throughout this document.

## 2. Envelope

Every serialized array begins with a 2-byte magic header, followed by
pointer-sized fields.

```
Offset  Size     Field
──────  ───────  ─────
0       2        Magic
2       ptrSize  Size (unsigned LE integer)
2+P     2        Type/Rank
4+P     P−2      Padding (zeros)
2P+2    rank×P   Shape (one unsigned LE integer per dimension)
2P+2+rank×P  …   Body (padded to P-byte boundary)
```

Where `P` = ptrSize (4 or 8 bytes, determined by the magic header).

### 2.1. Magic Header

| Byte 0 | Byte 1 | Pointer Size | Architecture     |
|--------|--------|-------------|------------------|
| `0xDF` | `0x94` | 4 bytes     | 32-bit Dyalog    |
| `0xDF` | `0xA4` | 8 bytes     | 64-bit Dyalog    |

Byte 0 is always `0xDF`. Byte 1 encodes the pointer size.

### 2.2. Size Field

The size field is a P-byte little-endian unsigned integer. For simple
(non-nested) arrays:

```
total_bytes = 2 + (size − 1) × P
```

This includes the size field itself but not the 2-byte magic header. The
formula does NOT apply to nested or opaque arrays.

For nested arrays (type code `0x06`):

```
size = 3 + rank + num_elements
```

The children are serialized inline after the shape. Each child begins with its
own size field (no magic header). The total byte count must be computed by
walking the children recursively.

For empty nested arrays (`num_elements = 0`), one prototype child is
serialized to preserve the array's fill element.

### 2.3. Type/Rank Field

Two bytes, immediately after the size field:

**Low byte — rank and flags:**

| Bits 7–4 | Bits 3–0 | Meaning          |
|----------|----------|-----------------|
| rank     | `0x0F`   | Simple array     |
| rank     | `0x07`   | Nested array     |
| rank     | `0x00`   | Opaque internal  |

**High byte — type code:**

| Code   | ⎕DR  | Type             | Element Size   |
|--------|------|------------------|---------------|
| `0x21` | 11   | Boolean          | 1 bit          |
| `0x22` | 83   | Int8             | 1 byte         |
| `0x23` | 163  | Int16            | 2 bytes        |
| `0x24` | 323  | Int32            | 4 bytes        |
| `0x25` | 645  | Float64          | 8 bytes        |
| `0x27` | 80   | Char8            | 1 byte         |
| `0x28` | 160  | Char16           | 2 bytes        |
| `0x29` | 320  | Char32           | 4 bytes        |
| `0x2A` | 1289 | Complex128       | 16 bytes       |
| `0x2E` | 1287 | Decimal128       | 16 bytes       |
| `0x06` | 326  | Pointer (nested) | recursive      |
| `0x00` | —    | Opaque           | see §5         |

Gaps exist in the type code space (e.g. `0x26`, `0x2B–0x2D`). Their meaning
is unknown.

The remaining `P−2` bytes after the type/rank field are zero padding.

### 2.4. Shape

`rank` unsigned LE integers, each P bytes. The product of the shape values
gives the total number of elements.

Rank-0 arrays (scalars) have no shape field.

## 3. Body — Simple Arrays

The body contains the array data, padded to the next P-byte boundary.

### 3.1. Boolean (type `0x21`)

Bits are packed MSB-first. The number of data bytes is `⌈elements ÷ 8⌉`,
padded to P.

**Verified example:** `1 0 1 1 0` (5 booleans)
```
Data: B0 00 00 00 00 00 00 00
      ││││└─── bit 4 = 0
      │││└──── bit 3 = 1
      ││└───── bit 2 = 1
      │└────── bit 1 = 0
      └─────── bit 0 = 1
```

### 3.2. Integer (types `0x22`, `0x23`, `0x24`)

Little-endian signed integers: 1, 2, or 4 bytes per element. Padded to P.

**Verified example:** Scalar `42` as Int8
```
DF A4 04 00 00 00 00 00 00 00   magic + size=4
0F 22 00 00 00 00 00 00         type=Int8, rank=0
2A 00 00 00 00 00 00 00         data: 42, padded to 8
```

**Verified example:** Vector `100000 200000 300000` as Int32
```
Data: A0 86 01 00  40 0D 03 00  E0 93 04 00  00 00 00 00
      (100000 LE)  (200000 LE)  (300000 LE)  (pad to 8)
```

### 3.3. Float64 (type `0x25`)

IEEE 754 double-precision, little-endian. 8 bytes per element. No padding
needed when P=8.

**Verified example:** Scalar `3.14`
```
Data: 1F 85 EB 51 B8 1E 09 40
```

### 3.4. Complex128 (type `0x2A`)

Two consecutive Float64 values: real part, then imaginary part. 16 bytes per
element.

**Verified example:** Scalar `1J2`
```
Data: 00 00 00 00 00 00 F0 3F   real = 1.0
      00 00 00 00 00 00 00 40   imag = 2.0
```

### 3.5. Decimal128 (type `0x2E`)

16 bytes per element. The exact encoding is IEEE 754 decimal128. No Go native
equivalent exists; these are preserved as raw byte arrays for round-tripping.

### 3.6. Character (types `0x27`, `0x28`, `0x29`)

1, 2, or 4 bytes per element, little-endian. The type is chosen by Dyalog
based on the maximum code point in the array.

Character scalars (rank 0) store a single character value.

**Verified example:** Vector `'hello'` as Char8
```
Data: 68 65 6C 6C 6F 00 00 00    "hello" + 3 bytes padding
```

**Verified example:** Vector `'⍳⍴⍬'` as Char16
```
Data: 73 23 74 23 6C 23 00 00    U+2373 U+2374 U+236C + pad
```

### 3.7. Empty Arrays

When the total number of elements is zero, no body bytes are present. The
shape field encodes the zero dimension.

**Verified example:** Empty character vector `''`
```
DF A4 04 00 00 00 00 00 00 00   magic + size=4
1F 27 00 00 00 00 00 00         type=Char8, rank=1
00 00 00 00 00 00 00 00         shape[0] = 0
(no body)
```

## 4. Body — Nested Arrays (type `0x06`)

The body consists of `num_elements` child arrays serialized sequentially. Each
child follows the same format as §2 but **without the 2-byte magic header**.

Children may themselves be nested, forming arbitrarily deep structures.

For empty nested arrays, one prototype child is included to preserve the fill
element. The prototype's value is not part of the logical array data.

**Verified example:** `(1 2)(3 4)` — nested vector of two int vectors
```
DF A4 06 00 00 00 00 00 00 00   magic + size=6
17 06 00 00 00 00 00 00         type=Pointer, rank=1, flags=nested
02 00 00 00 00 00 00 00         shape[0] = 2

Child 0: vector 1 2
  05 00 00 00 00 00 00 00       size=5
  1F 22 00 00 00 00 00 00       type=Int8, rank=1
  02 00 00 00 00 00 00 00       shape[0] = 2
  01 02 00 00 00 00 00 00       data: 1, 2

Child 1: vector 3 4
  05 00 00 00 00 00 00 00       size=5
  1F 22 00 00 00 00 00 00       type=Int8, rank=1
  02 00 00 00 00 00 00 00       shape[0] = 2
  03 04 00 00 00 00 00 00       data: 3, 4
```

## 5. Opaque Types (type code `0x00`)

`⎕OR` (Object Representation) and namespace internals serialize with type
code `0x00` and flags `0x00`. The body is an opaque proprietary structure.
The size field does NOT encode the total byte count.

For round-tripping, the entire byte vector must be preserved exactly.
`0(220⌶)` on the same bytes will reconstruct the original object.

Within an opaque blob, recognizable sub-structures exist. These use the same
size/type_rank/shape/data framing as standard arrays.

### 5.1. Distinguishing Dfns, Tradfns, and Namespaces

Byte `0x22` (decimal 34, zero-indexed) of the blob distinguishes object types:

| High nibble of byte 0x22 | Object type |
|--------------------------|-------------|
| `0x20`                   | Function (dfn or tradfn) |
| `0xA0`                   | Namespace |

This has been verified for Dyalog v20, 64-bit.

### 5.2. Dfn Bytecode

Dfn source is stored as tokenized bytecode in a Char8 vector sub-array. The
bytecode vector is identified by having `0xFF 0xFF` as its first two bytes.

**Structure:**
- 20-byte header: `FF FF` + 18 bytes of metadata
- Token stream: expression tokens with `0x00` padding between them
- `0x01` bytes appear as line-end markers

**Expression markers** use the pattern `XX YY 6F`:

| YY     | Meaning              |
|--------|---------------------|
| `0x1B` | Expression start     |
| `0x1C` | Guard (`:`)          |
| `0x1D` | Diamond (`⋄`)        |
| `0x1E` | Expression end       |
| `0x1F` | Expression end (alt) |

The `XX` byte preceding the marker appears to be a byte offset or counter.

Only tokens between the **first** `1B`→`1E` group constitute the function
body. Subsequent groups are operator/closure context from the ⎕OR capture
mechanism.

### 5.3. Dfn Token Table

Each APL primitive maps to a single-byte token. These are Dyalog's internal
IDs, not ⎕AV positions or Unicode code points.

**Primitive functions (complete — all verified via `(2031⌶)6`):**

| Code | Glyph | Code | Glyph | Code | Glyph | Code | Glyph |
|------|-------|------|-------|------|-------|------|-------|
| `02` | `+`   | `03` | `-`   | `04` | `×`   | `05` | `÷`   |
| `06` | `⌈`   | `07` | `⌊`   | `08` | `*`   | `09` | `⍟`   |
| `0A` | `\|`  | `0B` | `!`   | `0C` | `○`   | `0E` | `~`   |
| `0F` | `∨`   | `10` | `∧`   | `11` | `⍱`   | `12` | `⍲`   |
| `13` | `<`   | `14` | `≤`   | `15` | `=`   | `16` | `≥`   |
| `17` | `>`   | `18` | `≠`   | `1C` | `.`   |      |       |
| `1E` | `≡`   | `1F` | `≢`   |      |       |      |       |
| `20` | `⍴`   | `21` | `,`   | `22` | `⍪`   | `23` | `⍳`   |
| `24` | `↑`   | `25` | `↓`   | `26` | `?`   | `27` | `⍒`   |
| `28` | `⍋`   | `29` | `⍉`   | `2A` | `⌽`   | `2B` | `⊖`   |
| `2C` | `∊`   | `2D` | `⊥`   | `2E` | `⊤`   | `2F` | `⍎`   |
| `30` | `⍕`   | `31` | `⌹`   | `32` | `⊂`   | `33` | `⊃`   |
| `34` | `∪`   | `35` | `∩`   | `36` | `⍷`   | `37` | `⌷`   |
| `4F` | `⊆`   | `50` | `⍥`   | `52` | `⊣`   | `53` | `⊢`   |
| `5C` | `⍸`   | `5D` | `@`   |      |       |      |       |

Remaining gaps: `0x0D`, `0x19`–`0x1B`, `0x39`, `0x3C`–`0x3F`, `0x45`–`0x46`,
`0x49`, `0x4B`–`0x4E`, `0x51`, `0x56`–`0x58`, `0x5A`, `0x5E`–`0x5F`.

**Operators (complete — all verified):**

| Code | Glyph | Meaning                    |
|------|-------|---------------------------|
| `40` | `/`   | Reduce / Replicate         |
| `41` | `⌿`   | Reduce-first / Replicate-first |
| `42` | `\`   | Scan / Expand              |
| `43` | `⍀`   | Scan-first / Expand-first  |
| `44` | `.`   | Inner/outer product dot    |
| `47` | `¨`   | Each                       |
| `48` | `⍣`   | Power                      |
| `4A` | `⍨`   | Commute / Selfie           |
| `54` | `⍠`   | Variant                    |
| `55` | `⍤`   | Rank / Atop                |
| `59` | `⌸`   | Key                        |
| `5B` | `⌺`   | Stencil                    |

Operators follow their left operand: `02 40` = `+/`, `02 4A` = `+⍨`.
Outer product: `38 44 02` = `∘.+`.

**Syntax tokens (verified):**

| Code | Glyph | Meaning              |
|------|-------|---------------------|
| `3A` | `←`   | Assignment           |
| `3B` | `⎕`   | Quad                 |
| `38` | `∘`   | Jot / Compose        |
| `60` | `(`   | Open parenthesis     |
| `61` | `)`   | Close parenthesis    |
| `62` | `[`   | Open bracket         |
| `63` | `]`   | Close bracket        |

### 5.4. Dfn References

Two-byte sequences where the second byte identifies the reference type:

| Pattern  | Meaning                        |
|----------|-------------------------------|
| `XX 4C`  | Name/argument reference        |
| `XX 57`  | Literal pool reference         |
| `XX 3E`  | System variable reference      |

**Name references (`XX 4C`):**

| Index | Meaning |
|-------|---------|
| `00`  | `⍺` (left argument)   |
| `01`  | `⍵` (right argument)  |
| `02`  | `∇` (self-reference)  |
| `03+` | Local variables       |

**Literal pool references (`XX 57`):** see §5.5.

**System variable references (`XX 3E`):**

| Index | Verified |
|-------|----------|
| `02`  | `⎕IO`   |

Other system variable indices are not yet mapped.

**Dfn variable names** are encoded as inline ASCII bytes in the token stream.
Variable `r` is byte `0x72` (ASCII `r`). This applies to single-character
names; multi-character dfn locals have not been tested.

### 5.5. Dfn Literal Pool

Literal values (integers, floats, strings, vectors) referenced by `XX 57`
are stored as standard sub-arrays after the bytecode vector within the ⎕OR
blob.

**The pool is in reverse order.** All sub-arrays following the bytecode
(including metadata entries) form the pool. The **last** sub-array maps to
pool index 0, the second-to-last to index 1, and so on.

**Verified example:** `{(⍵+1)×2}`

Sub-arrays after bytecode: `[int8(2), int16(220), int8(1)]`

Reversed pool:
- Index 0 → `int8(1)` = 1
- Index 1 → `int16(220)` (metadata, not referenced)
- Index 2 → `int8(2)` = 2

Bytecode `00 57` → pool[0] = 1. Bytecode `02 57` → pool[2] = 2.

### 5.6. Tradfn Structure

Tradfns use the same token codes as dfns but with different framing.

**Name table:** Entries follow the pattern `01 XX 00 88 00 00 00 00`, where
`XX` is a name class byte, followed by a null-terminated UTF-16LE string.

The high nibble of `XX` encodes the APL name class:

| `XX` high nibble | Name class |
|-------------------|-----------|
| `0x00`            | Argument or local |
| `0x08` (special)  | Namespace/function name |
| `0x20`            | Variable (nc 2) |
| `0x30`            | Function (nc 3) |

Sentinel entries have `0xFFFF` as their text data and are skipped.

**Name indices** start at `0x70` and are assigned in **reverse order** of the
name table entries in the blob. The last entry in the blob gets index `0x70`,
the second-to-last `0x71`, etc.

**Token stream** starts with the first `0x67` byte followed by a byte ≥
`0x70` (a name reference). Lines are separated by `0x67`. The stream ends at
the first run of 3+ consecutive zero bytes.

The first line encodes the function header: `result ← name arg` (or
`name arg` for functions without a result).

**Literal pool offset:** Bytecode literal index = number of names + pool
position. With 4 names, the first literal is at bytecode index 4.

**Control keywords** use the same `XX YY 6F` marker pattern as expression
markers:

| YY     | Keyword (verified) |
|--------|--------------------|
| `0x00` | `:If`              |
| `0x04` | `:Else`            |
| `0x05` | `:EndIf`           |

Unverified but provisionally assigned: `0x01`=`:While`, `0x02`=`:Repeat`,
`0x03`=`:For`, `0x06`=`:EndWhile`, `0x07`=`:EndRepeat`, `0x08`=`:EndFor`,
`0x09`=`:Select`, `0x0A`=`:Case`, `0x0B`=`:EndSelect`.

### 5.7. Namespace Structure

Namespace ⎕ORs use type code `0x00` with byte `0x22` having high nibble
`0xA0` (vs `0x20` for functions).

The name table uses the same `01 XX 00 88` entry format as tradfns, listing
all namespace members. Entries are contiguous; a gap of >40 bytes indicates
the end of the name table.

Member values follow the name table:
- **Functions** are serialized as complete ⎕OR sub-blobs (with their own
  bytecode, literal pool, etc.)
- **Variables** are serialized as standard sub-arrays (matching §2–§4)

Variable values appear in **reverse name-table order** relative to the name
table entries.

The namespace name itself appears as a `01 08 00 88` entry when function
members are present.

---

## 6. Verified Test Cases

All of the following have been verified by serializing in Dyalog v20,
parsing in Go, re-serializing, and confirming byte-identical output and/or
`≡` identity in Dyalog.

### Arrays

| Value                       | Type     | Rank | Verified |
|-----------------------------|----------|------|----------|
| `42`                        | Int8     | 0    | marshal + unmarshal + exact bytes |
| `0`                         | Int8     | 0    | unmarshal |
| `¯5`                        | Int8     | 0    | unmarshal |
| `1000`                      | Int16    | 0    | unmarshal |
| `1000000`                   | Int32    | 0    | unmarshal |
| `2147483647`                | Int32    | 0    | unmarshal |
| `3.14`                      | Float64  | 0    | marshal + unmarshal + exact bytes |
| `1099511627776`             | Float64  | 0    | unmarshal |
| `'X'`                       | Char8    | 0    | unmarshal |
| `1J2`                       | Complex  | 0    | marshal + unmarshal + exact bytes |
| `1÷3` (⎕FR←1287)           | Dec128   | 0    | unmarshal |
| `1 2 3`                     | Int8     | 1    | marshal + unmarshal + exact bytes |
| `'hello'`                   | Char8    | 1    | marshal + unmarshal + exact bytes |
| `1 0 1 1 0`                 | Bool     | 1    | unmarshal |
| `1.1 2.2 3.3`              | Float64  | 1    | unmarshal |
| `200 300 400`               | Int16    | 1    | unmarshal |
| `100000 200000 300000`      | Int32    | 1    | unmarshal |
| `1J2 3J4`                   | Complex  | 1    | unmarshal |
| `'⍳⍴⍬'`                    | Char16   | 1    | unmarshal |
| `⎕UCS 100000 100001 100002` | Char32   | 1    | unmarshal |
| `''`                        | Char8    | 1    | unmarshal |
| `⍬`                         | Int8     | 1    | unmarshal |
| `2 3⍴⍳6`                   | Int8     | 2    | marshal + unmarshal + exact bytes |
| `2 3⍴'abcdef'`             | Char8    | 2    | unmarshal |
| `2 3⍴1 0 1 0 1 0`          | Bool     | 2    | unmarshal |
| `2 2⍴1.1 2.2 3.3 4.4`     | Float64  | 2    | unmarshal |
| `2 3 4⍴⍳24`                | Int8     | 3    | unmarshal |
| `(1 2)(3 4)`               | Nested   | 1    | unmarshal |
| `1 'hello' (2 3⍴⍳6)`      | Nested   | 1    | marshal + unmarshal + exact bytes |
| `⊂⊂1 2 3`                  | Nested   | 0    | unmarshal |
| `0⍴⊂''`                    | Nested   | 1    | unmarshal (with prototype) |

### E2E Round-Trips (Dyalog → Go → Dyalog, `≡` verified)

Scalars: `42`, `¯5`, `0`, `3.14`, `'X'`, `1J2`, `1000`, `1000000`.
Vectors: `1 2 3`, `'hello world'`, `1 0 1 1 0`, `1.1 2.2 3.3`,
`200 300 400`, `100000 200000 300000`, `1J2 3J4`, `'⍳⍴⍬'`, `''`, `⍬`.
Matrices: `2 3⍴⍳6`, `2 3⍴'abcdef'`, `2 3⍴1 0 1 0 1 0`, `2 2⍴1.1 2.2 3.3 4.4`.
Higher rank: `2 3 4⍴⍳24`. Nested: `(1 2)(3 4)`, `1 'hello' (2 3⍴⍳6)`.

### Decompiled ⎕ORs (source exactly matches original)

**Dfns:** `{⍵+1}`, `{⍺+⍵}`, `{⍵-1}`, `{⍵×2}`, `{+/⍵}`, `{+\⍵}`,
`{+⍨⍵}`, `{0=⍵:0 ⋄ ⍵}`, `{(⍵+1)×2}`, `{⍵[1]}`, `{⎕←'hello world'}`,
`{⎕IO}`, `{r←⍵+1 ⋄ r}`, `{0=2|⍵:⍵÷2 ⋄ 1+3×⍵}`,
`{⍵≤1:⍵ ⋄ (∇⍵-1)+∇⍵-2}`, `{0=⍵:⍺ ⋄ ⍵∇⍵|⍺}`, `{(+/⍵)÷≢⍵}`, `{⌽⍵}`,
`{×/⍵⍴⍺}`.

**Tradfns:** `r←add x / r←x+1`, `halve x / ⎕←x÷2`,
`r←a gcd b / :If b=0 / r←a / :Else / r←b gcd b|a / :EndIf`.

**Namespaces:** variable-only (`ns.x←42 ⋄ ns.name←'Neil'`), function with
literal (`ns.double←{⍵×2}`), function without literal
(`ns.avg←{(+/⍵)÷≢⍵}`).

---

## 7. Known Gaps

The following are **not yet documented** because they have not been tested or
reverse-engineered:

- Type codes `0x26`, `0x2B`–`0x2D`, `0x2F`–`0x3F` (if they exist)
- Remaining token gaps in the byte space (primitives and operators are fully mapped via `(2031⌶)6`)
- System functions and variables beyond `⎕IO` and `⎕←`
- Error guards (`::`) in dfn bytecode
- Multi-line dfns
- Multi-character variable names in dfns
- Tradfn string literals and locals (`;x;y` in header)
- Namespace member-value ordering when multiple functions and variables coexist
- Nested namespaces
- Class instances
- Operator ⎕ORs (as distinct from function ⎕ORs)
- The `XX` byte in `XX YY 6F` markers (appears to be an offset/counter)
- The meaning of `int16(220)` in the dfn literal pool
- The bytecode header's 18 metadata bytes (after `FF FF`)
- 32-bit format differences beyond the pointer size
- Cross-version compatibility (only v20 tested)
