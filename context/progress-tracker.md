# Progress Tracker

Update this file after every meaningful implementation change.

## Current Phase

- Brownfield adoption of the Six-File Context Methodology. cast is
  already at v0.17.0 with significant functionality shipped; the
  context files were generated from existing code and `CLAUDE.md`.

## Current Goal

- Adopt the spec-driven workflow on top of the existing codebase.
  Future features should be specced under `context/specs/NN-name.md`
  before implementation.

## Completed

The following features are already shipped in cast as of v0.17.0
(confirmed from `CHANGELOG.md` and the codebase — please verify and
prune anything stale):

- Layered config (`defaults → global → local → env → flags`) via
  `internal/config`.
- Makefile parser with `## name: desc [tags…]` doc-lines, auto-tags,
  auto-shortcuts, and walk-up via `source.lookup_depth`.
- Per-command flag tags: `[stream]`, `[confirm]`, `[interactive]`,
  `[sc=X]`, `[tags=…]`, `[pick=SPEC]`, `[as=…]`.
- Pick flow with multi-step picks, filters, glob semantics, friendly
  aliases.
- Snippet library (`~/.config/cast/snippets/<name>.mk`) with browse,
  preview, insert (`Enter`), delete (`dd`), and extract (`ctrl+y`).
- Single ↔ chain mode toggle (`ctrl+s`), explicit chain builder
  (`ctrl+a`), auto-queue, fail-fast policy, auto-save by sha1
  fingerprint, `history.chain_max` cap.
- Three themes (catppuccin, dracula, nord) × three env accents
  (local, staging, prod).
- Splash animation, real-time progress bar, command preview that
  follows sidebar selection, quit shortcut, command deletion from CLI.
- Versioning + CHANGELOG contract enforced via `internal/version`.
- Icon system with Nerd Font + emoji fallback via `views.IconSet`.

## In Progress

- None tracked yet.

## Next Up

- TBD — pick the next unit and run `/spec-driven-dev spec NN <name>`.
- Deferred from unit 02 (revisit later): multi-provider switch
  (Anthropic/Ollama), response caching by recipe hash, and parallel
  batching when a Makefile exceeds `ai.max_targets`.

## Recently Completed (post-adoption)

- **Unit 02 — ai-annotate-makefile** (v0.25.0, 2026-06-19). New
  `internal/ai/` package + `cast ai annotate` CLI
  (`--target/--all/--dry-run/--yes/--json`, exit codes 0/1/2/3) and a
  `ctrl+i` TUI popup (t/a/A menu → spinner → diff → apply+reparse). Adds
  the `[ai]` config section and `source.WriteDocLine`. Groq client is
  hand-written over `net/http`; description + categorical tags only — no
  behavioural-flag inference. Spec at
  `context/specs/02-ai-annotate-makefile.md`.
- **Unit 01 — stream-popup-fullscreen-copy** (shipped in v0.24.0,
  2026-05-12). Output popup now respects `ctrl+x` global quit, cycles
  Hidden → Popup → Fullscreen via `ctrl+e` (status bar pinned in
  fullscreen), and copies the buffer via OSC52 with `y`. Spec at
  `context/specs/01-stream-popup-fullscreen-copy.md`.
- **Fix — auto-shortcut blacklist** (2026-05-28). `autoShortcut` in
  `internal/source/makefile.go` now skips letters reserved by the TUI
  KeyMap (`q`, `g`, `G`, `s`, `r`, `y`) exposed via
  `source.ReservedShortcuts`. Prevents Makefile targets like `query:`
  from hijacking global hotkeys. User-assigned shortcuts (`[sc=X]`,
  `.cast.toml`) bypass the filter.

## Open Questions

- None.

## Architecture Decisions

- Adopted spec-driven-dev as the project's documentation backbone on
  2026-05-12. `CLAUDE.md` was rewritten to the skill's template
  (routes the agent through the six context files); the previous
  manual was migrated into `context/` rather than discarded.
- 2026-05-12: `context/specs/00-build-plan.md` will be **planned
  forward only**. Already-shipped functionality (v0.17.0) stays in the
  Completed section of this tracker; the build plan is reserved for
  upcoming units. Reason: backfilling would duplicate history and
  bloat the plan without informing future work.

## Known Gaps (carried over from previous `CLAUDE.md`)

- **Streaming output wiring**: `runner.StreamRun` exists but
  `dispatchRun` historically used the sync `runner.Run`. Verify
  whether v0.17.0 already wires `program.Send` and update this entry.
- **History persistence wiring**: `LoadHistory` / `SaveHistory` are
  implemented; verify they are called in `main.go` (load on start)
  and `model.go` `RunDoneMsg` handler (save on each run).
- **TabEnv (.env viewer)**: placeholder stub last time it was noted.
  Verify current state.

## Session Notes

- 2026-05-12: Migrated `CLAUDE.md` to spec-driven-dev structure. Six
  context files generated from the existing manual + codebase
  inspection. Original manual content is preserved across the six
  files — nothing was discarded.
