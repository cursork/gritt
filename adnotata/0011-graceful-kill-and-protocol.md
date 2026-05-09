# 0011 — Graceful kill flow & Dyalog shutdown protocol

Notes on how gritt cleans up a `-l`-launched Dyalog, why the obvious "send
SIGTERM" approach is insufficient on macOS, and what the RIDE protocol
actually carries on shutdown. Distilled from a long bug-hunt around an
"idle quit takes 10 seconds" report.

## Problem statement

`gritt -l` spawns Dyalog. When gritt exits — whether through `C-] q y`,
`)off`, or because a `-e` expression finished — the spawned interpreter
must be cleaned up. A stale Dyalog hogs ports, holds workspace files
open, and leaks memory.

The first iteration was a dropdoor-style escalation: SIGTERM, wait `N`
seconds, SIGKILL. That played the modal countdown for the full timeout
on every quit, even from a clean idle session. Why?

## Four discoveries, each more surprising than the last

### 1. `cmd.Wait()` lies about zombies

Initial alive-check used `syscall.Kill(-pid, 0)` — returns nil for any
process in the group, *including unreaped zombies*. So when Dyalog did
exit, gritt couldn't tell until something called `Wait()`. We moved to a
single launch-time goroutine that owns `cmd.Wait()` and closes a
`dyalogExited` channel; aliveness is a non-blocking channel check.

That fixed the post-exit lag but the modal still appeared on every quit.

### 2. `gritt`'s launch path runs through `mapl`, not the binary

`session.FindDyalog("")` does `exec.LookPath("dyalog")` first. On macOS
that resolves to `/usr/local/bin/dyalog` → a symlink to a shell wrapper
(`mapl`) inside the app bundle. `mapl` exports config env vars
(`DYALOG`, `USERCONFIGFILE`, `APL_LANGUAGE_BAR_FILE`, `AUTO_PW`, …) and
then `${DYALOG}/dyalog ${OPTS}` — note: no `exec`, just fork-and-wait.

So gritt's process tree is:

```
gritt
└── mapl (cmd.Process.Pid — what @pid showed)
    └── dyalog (real binary, what's actually serving RIDE)
        └── helpers...
```

`@pid` was reporting *mapl's* PID. `kill -TERM <that-pid>` kills the
shell wrapper; the real Dyalog gets reparented to init and keeps serving.
gritt's `cmd.Wait()` reaps mapl and falsely declares dyalog gone, while
the user's `5+7` still works because the real interpreter is alive.

Workaround: `@pid` now walks the process tree (`ps -A -o pid,ppid,comm`)
and prints every descendant. The user kills the deepest "dyalog" entry.
A cleaner fix — bypass mapl, replicate its env in Go — broke the tracer
window (some env var we don't yet know is needed). Left as future work.

### 3. SIGTERM is conditional on a controlling tty

`pkill dyalog` from a separate terminal kills user-launched Dyalog every
time. SIGTERM to a gritt-launched Dyalog (`+s -q` under mapl, no
inherited stdin/stdout because Go's `exec.Command` defaults to
`/dev/null`) is silently swallowed.

When SIGTERM is delivered to a Dyalog *with* a tty, it tries to run
destructors. If a destructor errors, it surfaces a debug session — and
the protocol log shows exactly what's emitted:

```
HadError {error:1002, dmx:0}
AppendSessionOutput type=5 "\n"
AppendSessionOutput type=5 "UnMake[1]\n"
OpenWindow {debugger:1, name:"UnMake", text:[...destructor body...]}
SetHighlightLine {win:1, line:1}
UpdateSessionCaption "CLEAR WS (...) - Dyalog APL"
SetPromptType type=1
Disconnect "Dyalog Session has ended"
CloseWindow {win:1}
```

The `Disconnect` and `CloseWindow` are the actual exit signal — Dyalog
*did* terminate.

For headless gritt-launched Dyalog the same shutdown path probably runs
internally, but the destructor-surface step (which expects a UI to
debug) hangs because there's nowhere to render and no client driving the
debug protocol back. SIGTERM appears to be ignored. SIGKILL is the only
reliable terminator.

### 4. `)off` works regardless

`)off` is the canonical APL graceful exit. It runs through the normal
`Execute` path, bypasses signal handling entirely, and Dyalog tears down
cleanly. Empirically tested via a probe (RIDE handshake → Execute
`)off\n` → Dyalog exits in well under a second).

## Resulting design — one three-stage process for every gritt-owned exit

Whether the exit is triggered by `C-] q y` in the TUI, a `-e` expression
finishing, a closed `-stdin`, a SIGINT to gritt, or a panic — the same
three stages run, in the same order, escalating only when a tier fails:

| Stage | Mechanism                       | What it asks Dyalog to do                             | When it fails / times out                                    |
|-------|---------------------------------|-------------------------------------------------------|--------------------------------------------------------------|
| **1** | `)off` via RIDE `Execute`       | Run APL's canonical clean-exit (destructors, save WS) | Connection isn't there, Dyalog is busy/stuck, or it's slow   |
| **2** | SIGTERM to the process group    | Handle the signal, run cleanup, exit                  | Headless Dyalog (no tty) hangs trying to surface debug — see §3 |
| **3** | SIGKILL to the process group    | (no negotiation — kernel-enforced)                    | Cannot fail                                                  |

Each tier has its own deadline:

- Stage 1 → 2: **5 s grace period** (constant `quitGracePeriod`). Long
  enough to absorb Dyalog's tty-cleanup printout (~½ s observed) plus
  margin.
- Stage 2 → 3: **`kill_timeout` seconds** (default 10, configurable in
  `gritt.json`).

A single launch-time goroutine owns `cmd.Wait()` and closes a
`dyalogExited` channel when the process is reaped. Every stage races
its deadline against this channel, so a Dyalog that responds *during*
any tier short-circuits the rest.

### TUI presentation

The TUI shows the user what's happening:

```
m.quit()
  ├─ Dyalog already exited?               → tea.Quit
  ├─ Connected & idle (m.ready)?          → Stage 1: )off, 5 s grace, no modal
  │     ├─ exits within grace?            → reapAndQuit, tea.Quit
  │     └─ grace elapses?                 → fall through to startKillModal
  └─ Busy / disconnected / Stage 1 done?  → startKillModal
                                              ├─ Stage 2: SIGTERM, open countdown modal
                                              ├─ exits during countdown? → reapAndQuit
                                              ├─ user presses [esc]?      → cancel — gritt + Dyalog stay alive
                                              ├─ user presses [k]?        → Stage 3 now (SIGKILL)
                                              └─ countdown hits 0?        → Stage 3 (SIGKILL)
```

`[esc]` is unique to the TUI — there's no equivalent in `-e` because
there's no user to interrupt the cleanup.

### Non-TUI presentation (-e / -stdin / -sock / -fmt with `-l`)

Same three stages, silent, no UI. Each non-TUI branch uses one helper:

```go
defer closeClient(client, *launch)
```

`closeClient` runs Stage 1 (sends `)off` via Execute) and drops the RIDE
socket. After `main` returns, the safety-net defer runs:

```go
gracefulKillDyalog(dyalogCmd, dyalogExited, killTimeout)
```

…which is Stages 2 and 3: try `<-exited` immediately (Stage 1 may have
already won), else SIGTERM + wait `kill_timeout`, else SIGKILL + wait
for reap. Empirically, idle Dyalog completes the whole sequence within
~½ s — `gritt -l -e '5+5'` measures at ~2 s wall clock end-to-end
including launch.

All silent — exit code is the only feedback channel.

## Connect mode (no `-l`)

When gritt connects to an already-running interpreter (via multapl,
plain `RIDE_INIT=SERVE`, etc.), `dyalogCmd` is nil and `dyalogExited` is
pre-closed. `m.dyalogStillRunning()` short-circuits. `quit()` returns
`tea.Quit` immediately — no signals, no `)off` injection, no modal.
Disconnect from gritt; Dyalog is unaffected. If the user types `)off`
manually it still flows through to the interpreter (correct: that's
what `)off` means).

## Open questions / future work

- **Bypass `mapl` properly**. Drop the wrapper, replicate the env in Go,
  get clean PID/wait semantics. Earlier attempt broke the tracer; need
  to bisect which env var matters (suspect `USERCONFIGFILE` or
  `APL_LANGUAGE_BAR_FILE`).
- **Pre-emptive `CloseWindow`s**. When gritt initiates quit, close every
  editor/tracer it tracks before sending `)off`. Dyalog tries to surface
  a debug during cleanup → we already told it we're done.
- **Reactive `CloseWindow`s**. While `m.killWaitActive`, immediately
  reply `CloseWindow` to any `OpenWindow {debugger:1}`. Lets Dyalog
  complete its destructor-error path even when no tty is around.
- **Allocate a pty for the spawned Dyalog**. Restores SIGTERM behaviour
  but adds significant launch complexity.
