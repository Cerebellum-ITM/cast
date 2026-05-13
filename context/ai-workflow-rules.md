# AI Workflow Rules

## Approach

Build this project incrementally using a spec-driven workflow. The six
context files in `context/` define what cast is, how it is structured,
and which conventions must be respected. Specs in `context/specs/`
define each individual unit of work. **Always implement against an
approved spec — do not infer or invent behavior from scratch.**

## Scoping Rules

- Work on one feature unit at a time.
- Prefer small, verifiable increments over large speculative changes.
- Do not combine unrelated system boundaries in a single
  implementation step (e.g. a runner change and a view change should be
  separate units unless the spec explicitly couples them).
- A bug fix does not need surrounding cleanup. A one-shot operation
  does not need a helper. Three similar lines is better than a
  premature abstraction.

## When to Split Work

Split an implementation step if it combines:

- UI rendering changes and subprocess/runner changes.
- Multiple unrelated packages (e.g. `source` and `library` and `tui`).
- Behavior not clearly defined in the relevant spec.
- Schema changes (new SQLite migration) bundled with feature work.

If a change cannot be verified end to end quickly (run the TUI, watch
the behavior), the scope is too broad — split it.

## Handling Missing or Ambiguous Requirements

- Do not invent product behavior not defined in `project-overview.md`
  or the unit's spec.
- If a requirement is ambiguous, stop and ask. Do not assume.
- If a requirement is missing, add it as an open question in
  `progress-tracker.md` before continuing implementation.

## Protected Files

Do not modify the following unless explicitly instructed:

- `internal/db/migrations/*.sql` — applied SQL migrations. Add a new
  numbered file instead of editing an existing one.
- `go.sum` directly — let `go mod tidy` regenerate it.
- `CHANGELOG.md` for changes that do not bump the version.

## Hard Rules (from `code-standards.md`)

These are non-negotiable. Re-read them before any non-trivial change:

1. **Version bump ↔ CHANGELOG entry** — same commit, both or neither.
2. **All icons are Nerd Font glyphs** added to `IconSet`. Never inline
   a literal glyph anywhere.
3. **`views/` is pure and never imports `tui`.** Render functions take
   Props + Palette and return strings. No model state, no
   package-level globals.
4. **Bubble Tea Update is the single ordering authority.** Goroutines
   never mutate the Model directly; they emit messages via
   `program.Send`.

## Keeping Docs in Sync

Update the relevant context file whenever implementation changes:

- New package or refactored boundary → `architecture.md` (Stack table,
  System Boundaries, import graph).
- New invariant or violated/relaxed invariant → `architecture.md`
  (Invariants).
- New feature visible to the user → `project-overview.md` (Features,
  Core User Flow).
- New convention or pattern → `code-standards.md`.
- New theme/icon/aesthetic decision → `ui-context.md`.
- Anything completed/in-progress/open → `progress-tracker.md`.

If a non-trivial gotcha, debugging recipe, or subsystem deep-dive does
not belong in code comments or the six context files, add a new file
under `docs/ai/` and reference it from the relevant context file.

## Before Moving to the Next Unit

1. The current unit works end to end within its defined scope (run the
   TUI; trigger the path; observe the result).
2. No invariant in `architecture.md` was violated.
3. `progress-tracker.md` reflects the completed work.
4. If the change is user-visible: `version.Current` bumped and
   `CHANGELOG.md` updated, same commit.
5. `make build` and `make test` pass.
6. `make lint` produces no new findings.
