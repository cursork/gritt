# 0010 — 220⌶ Binary Array Format (Reverse-Engineered)

Dyalog's `220⌶` serializes any APL array to a vector of signed bytes (−128..127).
The format is undocumented at the byte level. This is what we found by probing
Dyalog v20 (64-bit, macOS ARM64).

## Wire Format

```
Root Array:
  [2 bytes: magic]
  [ptrSize bytes: size (LE)]
  [2 bytes: type_rank]
  [ptrSize-2 bytes: padding]
  [rank × ptrSize bytes: shape (each LE)]
  [body, padded to ptrSize]

Embedded Child (nested element, no magic):
  [ptrSize bytes: size]
  [2 bytes: type_rank]
  [ptrSize-2 bytes: padding]
  [rank × ptrSize bytes: shape]
  [body, padded to ptrSize]
```

## Magic Bytes

- Byte 0: always `0xDF`
- Byte 1: architecture — `0x94` = 32-bit (ptrSize=4), `0xA4` = 64-bit (ptrSize=8)

## Size Field

`total_root_bytes = 2 + (size - 1) × ptrSize` (simple arrays only).

For nested arrays: `size = 3 + rank + num_elements`. Children are serialized
inline sequentially after the shape. Each child starts with its own size field.

For empty nested arrays: one prototype child is serialized (num_children = 1
even when num_elements = 0).

## Type/Rank Encoding (2 bytes LE)

Low byte (rank + flags):
- `(rank << 4) | 0x0F` for simple arrays
- `(rank << 4) | 0x07` for nested/pointer arrays
- `(rank << 4) | 0x00` for opaque internal types (⎕OR, namespaces)

High byte (type code):

| Code | ⎕DR  | Type           | Element Size |
|------|-------|----------------|-------------|
| 0x21 | 11    | Boolean        | 1 bit       |
| 0x22 | 83    | Int8           | 1 byte      |
| 0x23 | 163   | Int16          | 2 bytes     |
| 0x24 | 323   | Int32          | 4 bytes     |
| 0x25 | 645   | Float64        | 8 bytes     |
| 0x27 | 80    | Char8          | 1 byte      |
| 0x28 | 160   | Char16         | 2 bytes     |
| 0x29 | 320   | Char32         | 4 bytes     |
| 0x2A | 1289  | Complex128     | 16 bytes    |
| 0x2E | 1287  | Decimal128     | 16 bytes    |
| 0x06 | 326   | Pointer/nested | recursive   |
| 0x00 | —     | Opaque         | opaque blob |

## Data Layout

- **Boolean**: bit-packed MSB first, padded to ptrSize
- **Integers**: little-endian, padded to ptrSize
- **Float64**: IEEE 754 LE
- **Characters**: 1/2/4 bytes LE per element
- **Complex**: two float64 (real, imag)
- **Nested**: children serialized inline (no magic prefix)

## Opaque Types (type code 0x00)

⎕OR (object representation) and namespace internals use type code 0x00.
The internal structure is proprietary. For round-tripping, preserve as raw bytes.

Within an ⎕OR blob, recognizable sub-structures include:
- Char8 vectors (standard 220⌶ format) containing tokenized bytecode, function
  names, and string literals
- The sub-arrays use the same size/type_rank/shape/data format

## ⎕OR Bytecode Token Table

Dfn source code is stored as tokenized bytecode, not plain text. Each APL
primitive maps to a single-byte token. The token IDs are Dyalog's internal
enumeration (NOT ⎕AV positions or Unicode code points).

### Primitive Functions

```
Arithmetic:     02=+  03=-  04=×  05=÷  06=⌈  07=⌊  08=*  09=⍟  0A=|  0B=!  0C=○
Logic:          0E=~  0F=∨  10=∧  11=⍱  12=⍲
Comparison:     13=<  14=≤  15==  16=≥  17=>  18=≠
Match:          1E=≡  1F=≢
Structural:     20=⍴  21=,  22=⍪  23=⍳  24=↑  25=↓  26=?  27=⍒  28=⍋
                29=⍉  2A=⌽  2B=⊖  2C=∊  2D=⊥  2E=⊤  2F=⍎  30=⍕  31=⌹
                32=⊂  33=⊃  36=⍷  37=⌷  4F=⊆
```

### Operators (applied to primitives)

```
40 = /   (reduce/replicate)
42 = \   (scan/expand)
44 = .   (inner/outer product dot)
47 = ¨   (each)
4A = ⍨   (commute/selfie)
```

Operators follow their operand: `02 40` = `+/`, `02 42` = `+\`, `02 4A` = `+⍨`.
Outer product: `38 44 02` = `∘.+` (38=∘, 44=., 02=+).

### Syntax & References

```
3A = ←   (assignment)
3B = ⎕   (quad, for ⎕← output)
38 = ∘   (jot/compose)
60 = (   (open paren)
61 = )   (close paren)
62 = [   (open bracket — indexing)
63 = ]   (close bracket)
```

### References (2-byte: index + type marker)

```
XX 4C = name/arg reference (00=⍺, 01=⍵, 02=∇, higher=locals)
XX 57 = literal pool reference (XX is a pool index)
XX 3E = system variable reference (e.g. 02 3E = ⎕IO)
```

### Literal Pool

`XX 57` references a literal stored as a separate sub-array in the ⎕OR blob.
The actual value (int, float, string, vector) is in the sub-array, not in
the bytecode.

**The pool is stored in REVERSE order.** All sub-arrays after the bytecode
(including metadata like int16(220)) form the pool. The LAST sub-array maps
to pool index 0, second-to-last to index 1, etc.

Example: `{(⍵+1)×2}` has sub-arrays [i8(2), i16(220), i8(1)] after the
bytecode. Reversed: pool[0]=i8(1), pool[1]=i16(220), pool[2]=i8(2).
Bytecode `00 57` → pool[0]=1, `02 57` → pool[2]=2.

### Variable Names

Local variable names are encoded as **inline ASCII bytes** in the bytecode.
For example, variable `r` is encoded as byte 0x72 (ASCII 'r'). This is NOT
a pool reference — the name is directly in the token stream.

### Framing

```
67 = function body start
6B = function body end
01 = line-end marker (appears after each expression, before padding)
XX 1B 6F = expression/line start marker
XX 1C 6F = guard (:) — follows the guard condition
XX 1D 6F = diamond (⋄) separator
XX 1E 6F = expression/line end marker
```

The `XX` byte before `1B/1C/1D/1E 6F` appears to be a byte offset or counter.
Only the region between `1B 6F` and `1E 6F` markers contains expression tokens.
Everything outside these markers (prefix, suffix) is function metadata.

### Bytecode Structure

The bytecode char8 vector starts with a 20-byte header (FF FF + 18 bytes of
metadata), then the token stream. Zeros (0x00) between tokens are padding.

The decompiler (`decompile.go`) extracts expression regions between markers
and decodes tokens sequentially. Only the FIRST `1B`→`1E` group is the function
body — subsequent groups are operator/closure context (ignored).

Tested against dfns (all primitives), tradfns, and namespaces round-tripped
through live Dyalog v20. All primitive and operator tokens mapped via `(2031⌶)6`.

## Tradfn Bytecode

Tradfns use the same token codes but different framing:

- No FF FF bytecode header — token stream is embedded in an opaque sub-structure
- Lines separated by `0x67` (same byte as dfn fn-start)
- First line is the header: `result ← name arg` or `name arg` (no result)
- Names indexed from `0x70` in **reverse order** of char16 entries in the blob
  - char16 name entries follow `01 XX 00 88 00 00 00 00` metadata markers
  - Last entry in blob = index 0x70, second-to-last = 0x71, etc.
- Literal pool offset = number of names (bytecode index N+i → pool[i])
- Control keywords use `XX YY 6F` pattern (same as expression markers):
  - `YY=0x00` → `:If`
  - `YY=0x04` → `:Else`
  - `YY=0x05` → `:EndIf`

Token stream located by finding first `0x67` byte followed by a name-range
byte (`0x70`+), ending at first run of 3+ zero bytes.

### Unknowns

- Error guards (::)
- Multi-line dfn structure
- System functions beyond ⎕← and ⎕IO
- Multi-char variable names (only single ASCII char tested so far)
- Tradfn string literals
- Tradfn locals (`;x;y` in header)
- Namespace member-value ordering for mixed (var+fn) namespaces
- Nested namespaces
