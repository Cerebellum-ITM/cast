# Architecture Context

## Stack

| Layer        | Technology                                | Role                                                                |
| ------------ | ----------------------------------------- | ------------------------------------------------------------------- |
| Runtime      | Go 1.25+                                  | Single static binary, no runtime deps                               |
| TUI          | `charm.land/bubbletea/v2`                 | Event loop, Model/Update/View                                       |
| Layout       | `charm.land/lipgloss/v2`                  | Styling, borders, joins                                             |
| Widgets      | `charm.land/bubbles/v2`                   | Spinner, viewport, textinput                                        |
| Config       | `github.com/BurntSushi/toml`              | TOML parsing for `cast.toml` / `.cast.toml`                         |
| Persistence  | `modernc.org/sqlite` (pure Go)            | `sequences`, `sequence_steps`, `sequence_runs` chain history        |
| History (run) | `encoding/json` (stdlib)                 | Per-run records JSON                                                |
| Subprocess  | `os/exec` (stdlib)                        | Running `make <target>` and `make -C <dir> <target>`                |

## System Boundaries

- `cmd/cast/` — CLI entry point. Resolves flags, env, and subcommands
  (`cast`, `cast init [env]`, `cast config`). Does not contain business
  logic.
- `internal/config/` — Config type and layered `Load()` (defaults →
  global TOML → local TOML → env → flags). `toml.go` owns the on-disk
  TOML structs, `EnsureGlobal`, and `WriteLocalTemplate`.
- `internal/source/` — Command discovery. `source.go` defines the
  generic `CommandSource` interface plus `Command`/`EnvVar`/`EnvFile`
  types; `makefile.go` is the only implementation. Also owns
  `MakefileTargetLines`, `AppendMakefileTarget`, `ExtractMakefileTarget`.
- `internal/runner/` — Subprocess execution. `runner.go` exposes
  `Run()` (sync) and `StreamRun()` (async goroutine emitting Bubble Tea
  msgs). `history.go` owns `RunRecord`, `RunStatus`, `NewRecord()`,
  `LoadHistory()`, `SaveHistory()`.
- `internal/library/` — Snippet library. `Dir`, `EnsureDir`, `List`,
  `Load`, `Save`, `Delete`, `SanitizeName`. Plus errors
  `ErrTargetExists` / `ErrTargetNotFound`.
- `internal/db/` — SQLite migrations and access layer for chain
  history (`sequences`, `sequence_steps`, `sequence_runs`).
- `internal/tui/` — Bubble Tea Model, Update, View. `model.go` is the
  root; `keys.go` is the KeyMap; `styles.go` resolves
  `paletteFor(theme, env)`; `render.go` orchestrates header/body;
  `picker.go` is the folder picker.
- `internal/tui/views/` — **Pure render functions.** Every view takes
  explicit Props + Palette and returns a string. Never imports `tui`.
- `internal/tui/splash/` — Bubble Tea sub-model for the splash animation.
- `internal/ai/` — LLM-backed Makefile annotation. `ai.go` defines the
  `Provider` interface and the `Request`/`Plan`/`Annotation`/`TargetView`
  types; `groq.go` is the OpenAI-compatible Groq client (hand-written over
  `net/http`, zero SDK); `prompt.go` builds the system prompt + payload;
  `filter.go` selects targets (`BuildTargetViews`); `apply.go` renders the
  diff and writes annotations atomically (`RenderDiff` / `ApplyPlan`).
  Imports only the stdlib and `internal/source`; only `apply.go` touches
  `lipgloss` (and only to colour the diff — the plain render is exposed for
  callers that bring their own palette).
- `internal/version/` — Single source of truth for `version.Current`.

## Storage Model

- **SQLite (`~/.config/cast/cast.db`)** — Chain definitions and chain
  execution history. Tables `sequences`, `sequence_steps`,
  `sequence_runs`. Auto-saved chains use the synthetic name
  `auto:<sha1-prefix>`; capped by `history.chain_max` (default 100).
- **JSON file (config-driven `cfg.HistoryPath`)** — Per-run records
  (`RunRecord`). Loaded on start, saved on each `RunDoneMsg`.
- **Filesystem (`~/.config/cast/snippets/<name>.mk`)** — Snippet
  library, one `.mk` file per snippet. Plain Make so editor
  highlighting works.
- **TOML (`~/.config/cast/cast.toml` + `./.cast.toml`)** — Config.
  Global is auto-created via `EnsureGlobal`; local is opt-in via
  `cast init [env]`.

## Auth and Access Model

- cast is a local CLI; there is no authentication or remote access.
- The "auth-equivalent" concept is the **environment pill** (`local` /
  `staging` / `prod`). `prod` forces the confirmation modal on for
  every command that does not declare `[no-confirm]`.

## AI / Background Task Model

- **`cast ai annotate`** is the only AI component. It calls an OpenAI-compatible
  LLM endpoint (Groq by default) over `net/http` to propose Makefile doc-lines.
  In the TUI the call runs off the Update loop as a `tea.Cmd` that emits an
  `aiPlanMsg`; the popup blocks input (spinner) until it returns. `Annotate()`
  has zero side-effects — only `ApplyPlan()` writes to disk, atomically.
- No long-running background tasks beyond subprocess execution. Streaming
  output is implemented as a goroutine that emits Bubble Tea messages via
  `program.Send`; the Update loop remains the single ordering authority.

## Invariants

1. **`views` package never imports `tui`.** View functions are pure
   (Props + Palette → string) so they can be tested in isolation and
   debugged via a throwaway `cmd/debugview` (see
   `docs/ai/debugging-views.md`).
2. **No circular imports.** The import graph is strictly:
   `cmd/cast → {tui, ai, config, source, db}`;
   `tui → {views, runner, source, config, splash, library, db, ai}`;
   `views → {runner, source, ai, lipgloss}`;
   `ai → {source, stdlib, net/http}` (+ `lipgloss` in `apply.go` only);
   `source → stdlib only`. `ai` never imports `views`/`tui`, so the
   `views → ai` edge (the popup reads `ai.Plan`/`RenderDiff`) is cycle-free.
3. **All icons are Nerd Font glyphs by default**, with an emoji
   fallback. New icons are added to `IconSet` in
   `internal/tui/views/icons.go` and accessed via `Icons(style).<Field>`.
   Never hard-code a glyph (Nerd Font or emoji) inline in a view, model,
   or any other package.
4. **`version.Current` is the single source of truth for the CLI
   version.** Every user-visible change bumps it (semver) and adds a
   matching entry in `CHANGELOG.md` in the same commit. Both or neither.
5. **Config layering order is fixed**: defaults → global TOML → local
   TOML → `CAST_ENV` → CLI flags. Later sources override earlier ones.
   Do not add new sources without updating all five layers and the
   docs.
6. **Failure in a chain step drops the remaining steps.** The chain
   persists as failed; do not introduce "continue on error" semantics
   without an explicit user-facing opt-in.
