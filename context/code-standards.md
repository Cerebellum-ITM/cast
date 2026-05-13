# Code Standards

## General

- Keep packages small and single-purpose. The current split
  (`config`, `source`, `runner`, `library`, `db`, `tui`, `tui/views`,
  `tui/splash`, `version`) is the canonical layout; new functionality
  goes into the package whose responsibility it matches, not a new
  catch-all.
- Fix root causes; do not layer workarounds. If a rendering bug shows
  up in a view, fix the view (or its Props), not the model.
- Do not mix unrelated concerns in one file. `model.go` orchestrates;
  rendering belongs in `views/`; subprocess work belongs in `runner/`.

## Go conventions

- Target Go 1.25+. Use the stdlib first; only add a third-party
  dependency when no reasonable stdlib path exists.
- Exported identifiers carry godoc comments only when the meaning is
  non-obvious from the name. Default to no comments; do not narrate
  what the code does.
- Errors are values: return them, wrap with `fmt.Errorf("...: %w", err)`
  when adding context, never `panic` outside `main`.
- Avoid global mutable state. Config flows through explicit values
  (`cfg`, `palette`, `iconStyle`).

## TUI conventions (`internal/tui`)

- The Bubble Tea Model is in `model.go`. `Update` is the single
  ordering authority — all state changes go through messages, never
  through goroutines mutating the model directly.
- Long-running work (subprocess streams) runs in a goroutine that emits
  messages via `program.Send`; the goroutine does not touch the Model.
- Keyboard shortcuts are defined in `keys.go` as `key.Binding` values.
  Do not hardcode key strings in `Update`; reference `m.keys.<Field>`.

## Views package (`internal/tui/views`)

- Every view function is pure: takes explicit Props + a `Palette`,
  returns a `string`. It must compile with no `tui` import.
- Do not call `lipgloss` outside of `views/` (or `tui/styles.go` which
  builds palettes). Building styles inline in `model.go` is a bug.
- When adding a new view, follow the existing pattern: a Props struct
  in the same file, a single exported function, no package-level state.

## Icons

- All icons are Nerd Font glyphs by default. The `emoji` fallback
  exists for users without a patched terminal.
- Add new icons to `IconSet` in `internal/tui/views/icons.go`:
  Nerd Font codepoint in the `IconNerdFont` branch, emoji in
  `IconEmoji`.
- Reference icons via `Icons(style).<Field>` — never inline a literal
  glyph in any view, model, or other package.
- Outside the `views` package (e.g. `internal/tui/picker.go`), pass
  `views.IconStyle` in and resolve via `views.Icons(style)`.

## Config

- The five-layer load order (defaults → global → local → env → flags)
  is fixed. New config fields must:
  1. Get a default in `config.Default()`.
  2. Be declared in the TOML struct in `internal/config/toml.go`.
  3. Be propagated to whoever consumes it (no global state).

## Runner / subprocess

- `runner.Run` is synchronous and returns the captured output.
- `runner.StreamRun` runs in a goroutine and emits `RunOutputMsg` /
  `RunDoneMsg` via the Bubble Tea program reference.
- Subprocesses inherit the env from `os.Environ()` plus any `$(CAST_PICK_N)`
  / `$(ALIAS)` exposed by the picker. Never strip the parent env.

## File organization

- `cmd/cast/` — entry point only. No business logic.
- `internal/<pkg>/` — one logical concern per directory.
- Test files live next to the code they test (`foo.go` ↔ `foo_test.go`).
  No separate `tests/` tree.

## Persistence

- Per-run history: JSON via `runner.LoadHistory`/`SaveHistory`. Path
  comes from `cfg.HistoryPath`.
- Chain history: SQLite (`internal/db/`). Migrations live as numbered
  `.sql` files; never edit a migration after it has been applied —
  add a new one.
- Snippets: one `.mk` file per snippet under
  `~/.config/cast/snippets/`. Use `library.SanitizeName` for the
  filename basename.

## Versioning (HARD RULE)

- `internal/version/version.go` (`version.Current`) is the single
  source of truth. Edit it **in the same commit** as the change that
  bumps it — never in a separate bookkeeping commit.
- Bumps follow semver:
  - **PATCH** — bug fixes, copy tweaks, internal refactors with no
    observable behavior change.
  - **MINOR** — new feature, new keybinding, new CLI subcommand, new
    Makefile tag, or any additive change a user could notice.
  - **MAJOR** — breaking change to config schema, CLI flags, or
    Makefile tag grammar.
- Current: `0.17.0` (kept in sync via `version.Current`).

## Changelog (HARD RULE)

Every version bump in `internal/version/version.go` **must** be
accompanied by a matching entry in `CHANGELOG.md` at the repo root, in
the same commit. No exceptions.

- Always written in **English**, regardless of conversation language.
- One section per version, newest first:
  `## [<version>] – YYYY-MM-DD`.
- Sub-headings: `### Added`, `### Changed`, `### Fixed`, `### Removed`.
  Skip empty headings.
- Each item is a one-line summary leading with the surface that
  changed (`Library tab:`, `Picker:`, `Theme tab:`).
- **Include usage examples for new user-facing features** — a new
  keybinding, Makefile tag, CLI subcommand, or config key without a
  fenced code block snippet (`make`, `toml`, `shell`) is incomplete.
- Bug fixes name the symptom and, when known, the version that
  introduced the bug.
- Behaviour changes affecting existing users go in `### Changed` with
  a short rationale.

If a change does not bump the version (exploratory or aborted work),
do **not** edit the changelog. The contract is: version bump ↔
changelog entry, both or neither.

Format reference: <https://keepachangelog.com/en/1.1.0/>.

## Build

```bash
make build          # → bin/cast
make run            # build + run
make test           # go test ./...
make lint           # golangci-lint
```

Before declaring a unit complete, `make build` and `make test` must
both pass.
