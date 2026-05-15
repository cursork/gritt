# 0012 — `-sock` extension and rejected data-protocol modes

Context: branch `socket-inject`, commit `40b225f` extends `-sock` to keep
the TUI alive while also opening a socket server. Question that came up
during review: should `-sock` also grow data-structure modes (APLAN,
220⌶/aplor) like `grittles/aplsock` has, so external tooling can get
structured responses from a TUI-attached gritt?

## Options considered

1. **Modeline-style mode switching**: per-connection or per-request
   `⍝ MODE: aplor` comments — harmless to interpret as APL (it's a
   comment), parsed by gritt's socket handler to switch the response
   format. Sticky like vim modelines, any line. Plain stays the
   default for nc users.
2. **Two flags, two channels**: `-sock` stays plain-text, a new
   `-prepl PORT` bootstraps an aplsock-style prepl alongside the TUI
   for structured responses. No mode-switching. Cleaner separation,
   no dual-backend complexity.
3. **Do nothing in gritt**: `aplsock` already exists for headless
   data-protocol use, and a user with a running gritt can `⎕FIX` the
   prepl namespace from inside the session — `2 ⎕FIX'file://…/Prepl.apln'`
   then `Prepl.Start :4200` — and they're done. No new flag, no new
   code, no new failure modes.

## Decision

Option 3. The gritt `-sock` channel stays as the nc-friendly primitive.
Anyone wanting structured data has two existing answers:

- **Headless**: run `aplsock` directly. Already supports `-mode plain`,
  `-mode aplan`, `-mode aplor`.
- **Alongside a TUI**: `⎕FIX` the prepl from the gritt session. The
  prepl runs on its own APL thread (not thread 0, which deadlocks with
  RIDE — both use Conga). Trivial to do, no gritt code needed.

## Why option 1 was rejected

The plain↔aplan switching scenario it solved isn't a workflow anyone
actually has. If you want structured data, you stay in that mode. The
modeline mechanism imposed real cost (mode parsing, sticky-state, dual
backend on one connection — RIDE for plain, prepl for aplor) for a
usability gain no one was asking for.

## Why option 2 was rejected

Same payload as option 3 (bootstrap a prepl on a side thread), more
machinery (a flag, a listener, lifecycle code). Since prepl bootstrap
from inside an APL session is a one-liner, the flag is sugar.

## What this *doesn't* close off

The decision is about the socket-inject branch's `-sock`, not about
gritt's data-exploration story.

`prapl` (`~/dev/prapl`) is a separate PoC web app proving the
text-in / 220⌶-out substrate is sufficient for a Reveal/REBL-shaped
data inspector — Editor / Session / Prints / Navigator panes,
breadcrumb drill-down, tap channels for live `⎕←`-style output.
Prototype quality, not a product, but a useful **idea mine** for
exploration UIs that could live in gritt's TUI itself.

The interesting next step there isn't "wire gritt to prapl" — it's
"port prapl's exploration patterns into a gritt TUI pane fed by aplor
from a prepl bootstrapped inside the session". `amicable.Unmarshal`
already returns the same Go types `data_browser.go` knows how to
render, so the plumbing is mostly type-routing rather than new UI.
Captured in FACIENDA under `## aplsock / Prepl` →
"prapl-style exploration UIs in gritt's TUI".

## Files touched on this branch

- `socket_inject.go` — new, holds the listener + per-connection handler
- `tui.go` — `socketLineMsg` integration, `drainSocketQueue`,
  `activeSocket` state machine. `drainSocketQueue` mirrors the
  injected expression into `m.lines` above the active input line
  before sending Execute, so the user sees the source of any output
  that follows; the existing `lastExecute` skip eats Dyalog's
  type=14 echo to avoid duplication.
- `main.go` — flag parsing, `parseSockAddr`, listener startup
- `tui_test.go` — integration tests covering the round-trip,
  including a positive assertion that the injected expression text is
  mirrored into the session. Also tightened the startup `WaitFor`
  from `"gritt"` to `"╭─ gritt"` — the splash satisfies the bare
  string before the framed UI is up, which raced the first `Test()`
  against an `IsAlive`-false screen.
