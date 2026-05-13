# Build Plan

Decomposition of upcoming work into ordered, scoped, verifiable units.
Each unit becomes its own spec file in this folder (`NN-name.md`).

> **Brownfield note.** cast is already shipping at v0.17.0. This plan
> is for **future** units — already-shipped work is summarised in
> `context/progress-tracker.md` (Completed). Run
> `/spec-driven-dev plan` to populate the table below once the next
> features are agreed.

## Ordering Rules

1. **Dependencies first** — if B requires A, build A first.
2. **Invariants before features** — if a change requires touching
   `architecture.md` invariants, write/refine the invariant first.
3. **Backend before frontend wiring** — runner/source/db changes
   before the TUI views that surface them.
4. **UI shells before live data** — render the view against a fake
   Props struct first, wire to the model second.
5. **Install dependencies just in time** — only when first needed.

## Units

| # | Name | What it builds | Depends on |
|---|------|----------------|------------|
| 01 | stream-popup-fullscreen-copy | Global quit (`ctrl+x`) inside the stream popup, second-`ctrl+e` fullscreen mode preserving the status bar, OSC52 copy with `y`. | none |

## Notes

- Merge units that always get done in the same session with no
  standalone verifiable result.
- Split units that try to do too much (the "split" rules in
  `ai-workflow-rules.md` apply).
- Each unit must produce one visible, verifiable result — either in
  the TUI, in the CLI output, in the SQLite/JSON state, or in the
  test suite.
