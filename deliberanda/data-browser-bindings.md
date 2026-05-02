# Data browser key bindings — defaults to revisit

The data browser now exposes five commands in the `data-browser` context. The
defaults below were picked quickly so the feature works out of the box, but
none of them are obviously right and they should get a second look.

| Command | Default | Notes |
|---|---|---|
| `append-row` | `down` | Down past the last row extends the array. Easy to fat-finger; pending guard was tried and rejected. |
| `append-column` | `right` | Symmetric with append-row. Same fat-finger concern. |
| `delete-row` | `ctrl+d` | Destructive, no confirmation. Leader chord might be safer. |
| `delete-column` | `alt+d` | Mac users may dislike `alt`/Option since it's used for symbol input elsewhere. |
| `close-discard` | `ctrl+w` | Wanted shift+escape but bubbletea v1 can't distinguish it from plain escape. `ctrl+w` may collide with terminal "delete word" muscle memory. |

## Open questions

- Is unconstrained Down spam genuinely fine, or do we want to revisit a
  pending-row guard once people have used it for a bit?
- Should `delete-row` on a 1-row matrix collapse to shape `0 N` (current plan)
  or refuse?
- `close-discard` belongs in *every* editor pane, not just the data browser
  — should we promote it to a non-context-scoped command later?
- If we ever bump bubbletea to v2, revisit `shift+escape` for discard.
