# 0016 — Auto-localisation of tradfn headers

How gritt helps keep tradfn local-name lists honest: an automatic
"scope everything on save" mode, an on-demand cleanup, and a
cursor-driven toggle. All three share the pure logic in
`autolocalise.go`; the TUI wiring is in `tui.go` and the commands are
registered in `commands.go`. There is no protocol support for any of
this — RIDE has none either (its Ctrl+Up "TL" is a client-side edit).
gritt rewrites the header text locally, then saves through the normal
`SaveChanges` path.

## Why

APL tradfns require every local to be named explicitly in the header
signature (`FnName;local1;local2`). Forget one and it leaks into the
global namespace — a classic source of action-at-a-distance bugs.
Keeping the list current by hand is tedious, so gritt automates it.

## The three commands

All are palette commands (no leader key). Open the palette and type the
name, or its alias.

| Command | Alias | Effect |
|---------|-------|--------|
| `autolocalise` | `auto-scope` | Toggles autolocalise **mode**. While on, every tradfn save first appends any unlocalised assigned variables to the header. Title bar shows `[AL]`. |
| `localise` | — | One-shot cleanup of the focused editor: **adds** missing locals **and removes** stale ones (locals no longer assigned in the body). Marks the buffer modified. |
| `toggle-local` | — | Toggles the single word under the cursor in/out of the header — gritt's equivalent of RIDE's Ctrl+Up. |

`autolocalise` is add-only (it never removes a local, so hand-placed
entries are safe). `localise` is the destructive-but-tidy variant that
also prunes. `toggle-local` is the surgical one.

## What counts as a "local to add"

`findAssignedVars` scans the body for assignment targets. It **skips**:

- comments (everything from an unquoted `⍝`)
- system variables (`⎕IO`, `⎕ML`, …)
- namespace-member assignments (`obj.field←…` — the target is `obj`, already in scope)

Anything already in the signature or the existing local list is left
alone. New locals are appended; existing ones keep their original order.
Autolocalise only fires on tradfn entity types (1 = function,
2 = monadic operator, 3 = dyadic operator) — never on variables or
namespaces.

## The syntax: `⍝ GLOBALS:` exclusion comment

Some assigned names are *deliberately* global (a shared cache, a config
namespace, a log handle). To stop autolocalise from pulling them into
the header, list them in a comment anywhere in the function body:

```apl
⍝ GLOBALS: Cache Config LogFile
```

- Matched case-insensitively: `^\s*⍝\s*GLOBALS:\s*(.*)$` (`autolocalise.go:246`).
- Names are whitespace-separated after the colon.
- Every listed name is excluded from both `autolocalise` (add) and
  `localise` (add + prune), so a global assigned in the body won't be
  localised and won't be flagged stale.

An empty `⍝ GLOBALS:` line (no names) is treated as "no globals".

## Header format that gets written back

```
FnName;local1;local2 ⍝ any trailing header comment is preserved
```

`parseHeader` splits the signature, the `;`-delimited locals, and a
trailing header comment; `buildHeader` reassembles them. Dfn-style
headers and header comments survive round-trips.

## Interaction: `toggle-local` + GLOBALS

`toggleLocal(text, fnName, varName, createGlobals)` where `createGlobals`
is `m.autolocalise` (tui.go:1889). This makes the toggle DWIM depending
on mode:

- **Adding** a var to locals always *removes* it from `⍝ GLOBALS:` if
  present (it can't be both).
- **Removing** a var from locals:
  - if autolocalise mode is on (`createGlobals` true) → always add it to
    `⍝ GLOBALS:`, creating the comment line right after the header if
    none exists. Rationale: with autolocalise on, an unlisted assigned
    var would just get re-localised on the next save, so demoting to
    local ⇒ you meant it to be global, so record that intent.
  - if mode is off → add to `⍝ GLOBALS:` only if the comment already
    exists (don't clutter functions that don't use the convention).

## Where it hooks in

- `autolocaliseEditor(token)` runs before save in the editor, tracer,
  and close paths (tui.go:1710, 2124, 3347).
- `localiseEditor()` / `toggleLocalisation()` resolve the focused
  editor (falling back to the tracer or any open editor pane, since the
  palette clears focus on dismiss) and rewrite `w.Text`.
- Title-bar `[AL]` indicator: tui.go:3708.

## Source & tests

- Pure logic + doc comments: `autolocalise.go`
  (`findAssignedVars`, `stripComment`, `parseHeader`, `buildHeader`,
  `parseGlobalsComment`, `autolocaliseText`, `localiseText`, `toggleLocal`).
- Worked examples: `autolocalise_test.go` — the clearest reference for
  exact input→output behaviour on edge cases (existing locals preserved,
  globals excluded, no-change short-circuits, stale pruning).
