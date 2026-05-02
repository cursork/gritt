# BUG: `amicable.unmarshalNamespace` silently drops class-9 members

## Summary

A namespace whose members include other namespaces (or instances,
classes, interfaces — any class-9 entity) cannot round-trip through
`amicable.Unmarshal`. The class-9 members are filtered out before
values are extracted, so they vanish from the resulting `*codec.Namespace`.

This is data loss, not a representation choice. The decompile path
already detects namespace blobs (`decompile.go:16` byte-pattern
discriminator); the unmarshal path simply doesn't recurse into them.

## Where it bites

`amicable/amicable.go`, lines 109-114:

```go
// Separate namespace name from value members.
var memberList []nsMember
for _, m := range members {
    if m.class != 9 {
        memberList = append(memberList, m)
    }
}
```

Any member with `class == 9` (covers `9.1` namespace, `9.2` instance,
`9.4` class, `9.5` interface, `9.6` external class) is filtered out
before values are extracted. The corresponding sub-blob bytes are
still in `data` — they're just never read.

## Reproduction

`amicable/bug_namespace_class9_test.go` (in this PR) is a focused
failing test built in the same style as `TestUnmarshalNamespace`. It
skips if `gritt` isn't on `PATH`.

The setup:

```apl
ns←⎕NS '' ⋄ ns.x←42 ⋄ ns.y←⎕NS '' ⋄ ns.y.z←'hello'
```

Expected unmarshal output:

```
*codec.Namespace{
  Keys:   ["x", "y"],
  Values: {
    "x": 42,
    "y": *codec.Namespace{ Keys:["z"], Values:{"z":"hello"} },
  },
}
```

Actual today:

```
*codec.Namespace{
  Keys:   ["x"],
  Values: { "x": 42 },
}
```

`y` is silently dropped.

```sh
go test ./amicable -run TestUnmarshalNestedNamespace -v
```

## Impact

Every consumer that nests a namespace inside a namespace will hit this.
That includes **prapl** (a prepl-driven UI experiment) which ships
envelope namespaces over the wire:
`(tag: 'ret' ⋄ val: <value>)`. When `val` is itself a namespace, it
dies on Unmarshal.

prapl's current workaround is to pre-encode `val` to 220⌶ bytes outside
the envelope and ship it as a byte-vector field, decoded separately on
the Go side. Functional but a layering wart that leaks into both the
APL side (vendored `Prepl.apln` carries `val_kind: 'aplor'` flags) and
the Go client (special-cases bytes-typed members).

In gritt itself, any future tooling that wants to walk a workspace
namespace tree (a debugger view, a serialiser, a project-tree
explorer…) is blocked.

## Suggested fix direction

In `unmarshalNamespace`, instead of filtering out class-9 members:

1. **Detect the sub-blob boundary** for a class-9 member. This is the
   real new work. Nested-namespace blobs need their own boundary
   detection within the parent blob — which is presumably why the
   original code dodged it. The decompile side already has some logic
   for finding namespace boundaries in `decompile.go`; that may be
   reusable or at least instructive.
2. Slice the bytes for the sub-blob and **recursively** call
   `unmarshalNamespace`.
3. Add the recursed `*codec.Namespace` to `values` for that member.

Sub-classes 9.2 / 9.4 / 9.5 / 9.6 each have slightly different blob
shapes and should be considered separately when extending coverage.

## Tests to add when fixing

Beyond the repro:

- Two-level nested namespaces (`ns.y.z` is itself a namespace).
- Mixed: namespace with function (class 3) + variable (class 2) +
  nested namespace (class 9.1) members.
- Instance (class 9.2) — created via `:Class … :EndClass` then
  instantiated with `⎕NEW`.
- Class (class 9.4) member.
- Interface (class 9.5) member.
- Round-trip stability: `Marshal → Unmarshal → re-Marshal` produces
  identical bytes for a namespace-of-namespace.

## Out of scope (initially)

- Performance — straightforward correctness first; profile later.
- Cycle detection (a namespace referencing itself). 220⌶ may already
  prevent serialising cycles, but worth confirming.
- Other code paths (decompile, etc.) — only the unmarshal-to-Go-types
  path is broken in the way this bug describes.

## Backstory

Discovered while building **prapl**, a prepl-driven UI experiment that
exercises 220⌶ end-to-end as the wire format. The bug surfaced when an
envelope `(tag: 'ret' ⋄ val: <namespace>)` came back missing `val`.
Inner-claude diagnosed the filter at `amicable.go:111`, wrote the
workaround, and Neil flagged it as something a previous Claude had
written lazily and he had missed at review time.
