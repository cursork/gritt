# Structured Editing

Structured editing of pure data values (namespaces, matrices, vectors) in the TUI. Instead of editing raw APLAN text, present data in natural visual layouts with drill-down navigation.

This would be a significant differentiator over both RIDE and the Windows IDE, which only offer text-based editing of array notation.

## Core Concept: View Stack

Every compound value has two modes:

- **Display mode** — fits in a single cell or line. A preview: `Serialize(value, UseDiamond: true)` truncated to fit.
- **Expanded mode** — takes over the editor pane with a layout natural to the type.

Selecting a compound value in display mode pushes its expanded mode onto a **view stack**. Backing out pops it. Each level of the stack is simple on its own — the complexity of deep nesting is handled by the stack, not by any single view.

### Breadcrumb Bar

The view stack is shown as a breadcrumb path at the top of the pane:

```
myns > config > defaults > timeout
```

- Namespace keys shown by name: `config`
- Matrix indices shown APL-style: `[3;2]`
- Vector indices: `[5]`
- Each segment is selectable — jump back to any level

## Type-Specific Views

### Namespace

Key-value list. Keys on the left, values on the right.

```
 name     'John'
 age      42
 scores   1 2 3 4 5
 address  (street: '...' ⋄ city: '...')
```

- Scalar values: edit inline (Enter to start editing, Esc to cancel, Enter to confirm)
- Compound values: show display-mode preview, Enter to drill in
- Simple vectors (all-numeric or short): potentially editable inline too

### Matrix

Grid layout. Navigate with arrows.

```
     [1]  [2]  [3]
[1;]  42   17    3
[2;]   8   99   11
[3;]   0    5   64
```

- Scalar cells: edit inline
- Compound cells: show preview, Enter to drill in
- Row/column headers show indices

### Vector

Vertical list (or horizontal for short all-numeric vectors).

```
[1]  42
[2]  'hello'
[3]  (x: 1 ⋄ y: 2)
[4]  [1 2 ⋄ 3 4]
```

- Same inline/drill-down rules as above

## Boundaries

### What gets structured editing

Values where `codec.APLAN` can round-trip them: scalars, vectors, matrices, namespaces composed of pure data.

### What stays as text

- Functions, operators, classes — anything with code
- Namespaces containing functions (entityType indicates this)
- Values that fail APLAN parse

The check is simple: if `ShowAsArrayNotation` gives us parseable APLAN with only data, we can structured-edit. Otherwise, fall back to the existing text editor.

## Save Path

Edit changes the Go value in-place. On save:

1. `codec.Serialize` the modified value back to APLAN
2. Send via `SaveChanges` as normal
3. The interpreter doesn't know or care that we edited structurally

## Type Glyphs

Single-character prefix for compound values in preview cells:

| Type | Glyph | Rationale |
|------|-------|-----------|
| Namespace | `#` | APL root namespace convention |
| Matrix/Array | `⊞` | Looks like a 2x2 grid |
| Vector | `≡` | Triple bar — universal list icon |

## Versioning

- **v0**: Browse-only. View stack, drill-down, scrolling. No editing.
- **v1**: Editing scalars inline. Maintain type, error if not possible. Save serializes back to APLAN.
- **Later**: Adding/removing keys/rows, pagination, type changes, live editing.

## Open Questions

- Undo: per-cell, per-drill-level, or whole-value? (v1 decision — v0 is read-only)
- Live editing (save on every change vs explicit save)? Do it if easy, skip if not.
