# Namespace Unmarshal — Status

## Done

`unmarshalNamespace` in `amicable/amicable.go` rewritten. Sequential walk from `nameTableEnd`, one value per member, reverse name-table order. All variable types work (scalar, string, vector, matrix, nested). Function members extracted as opaque `Raw` bytes via `skipFnBlob`.

### Tests

- `TestUnmarshalNamespace` in `amicable/decompile_test.go`: two_vars, three_vars, vector_val, matrix_val, fn_member, mixed_var_fn — all pass
- `TestModeAplor` scalar/string/error — all pass
- All existing amicable tests still pass

### Key functions

- `findNextSubArray(data, from)` — scans forward for next valid sub-array (flags 0x0F simple, 0x07 nested), parses with `readArray()`, skips FF FF bytecode vectors
- `skipFnBlob(data, from)` — finds FF FF bytecode marker, parses the char8 vector to find its end, skips trailing sub-arrays (literal pool)
- `validTypeCode(tc)` — whitelist of known type codes

## Remaining: embedded function decompilation

`TestModeAplor/function_roundtrip` still skipped. The problem: embedded function blobs within a namespace use a **different encoding** than standalone `⎕OR` blobs.

### What we discovered

1. **19-field (152-byte) header** before bytecode: starts with 8 zero bytes, then `07` (type), then `D5 50` (magic), then metadata fields including `EQ\x30` version marker, function flags (`0x40002`), and content size.

2. **Bytecode uses tradfn-style encoding** inside namespace blobs — literal pool indices are offset by the number of names. For `{⍵+1}`, standalone bytecode uses `00 57` (literal index 0) but namespace-embedded bytecode uses `03 57` (literal index 3, after ⍺=0 ⍵=1 ∇=2).

3. **Second and third name table repetitions** appear after the function blob. Variable values (`'hello'`) appear inline between entries in the second repetition. The third repetition contains duplicates.

4. **Variable values are NOT immediately after the first name table** in mixed namespaces. They appear after the function blob, in the second name table region. `findNextSubArray` finds them correctly by scanning forward past the function blob.

### To make function_roundtrip work

Two possible approaches:

**A. Reconstruct standalone ⎕OR from embedded bytes.** This requires:
- Understanding the full 152-byte internal header (which fields to keep, which to rewrite)
- Adjusting bytecode literal indices (subtract name count offset)
- Prepending the standard ⎕OR envelope (magic + size + type_rank)
- Heavy reverse-engineering with no guarantee of correctness across Dyalog versions

**B. Add an embedded-function decompiler.** The bytecode tokens are the same, just the framing differs:
- Names indexed from 0x70 in reverse order (tradfn-style, already handled by `decompileTradfn`)
- Literal pool indices offset by name count
- 152-byte header instead of standalone ⎕OR header
- Could share most logic with existing `decompileDfn`/`decompileTradfn`

Option B is more pragmatic. The embedded encoding is closer to tradfn than dfn — it may be that ALL functions inside namespaces get tradfn encoding regardless of whether they were written as dfns.

### Spec updates needed

`220-SPEC.md` §5.7 needs:
- The 152-byte function sub-blob header structure
- The literal index offset rule (literal index = pool index + name count)
- The second/third name table repetition discovery
- Clarification that variable values appear after function blobs in mixed namespaces
