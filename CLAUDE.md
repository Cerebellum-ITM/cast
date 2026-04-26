# cast — codebase guide

**cast** is a terminal task-runner with a Bubble Tea TUI. It reads `Makefile` targets, lets you browse/search/run them, and streams output in real time.

Module: `github.com/Cerebellum-ITM/cast`  
Go: 1.25+  
Framework: `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2`

---

## Directory structure

```
cast/
├── cmd/
│   └── cast/
│       └── main.go           # CLI entry point: flags, env resolution, subcommands
├── internal/
│   ├── config/
│   │   ├── config.go         # Config type, layered Load() (defaults → global → local → env → flags)
│   │   └── toml.go           # TOML structs, LoadGlobal/LoadLocal, EnsureGlobal, WriteLocalTemplate
│   ├── runner/
│   │   ├── runner.go         # Run() (sync) and StreamRun() (async goroutine) for `make <target>`
│   │   └── history.go        # RunRecord/RunStatus types, NewRecord(), LoadHistory(), SaveHistory()
│   ├── source/
│   │   ├── source.go         # Command, EnvVar, EnvFile types; CommandSource interface
│   │   └── makefile.go       # Makefile parser: ## name: desc comments, auto-tags, auto-shortcuts
│   └── tui/
│       ├── model.go          # Root Bubble Tea Model: Init/Update/View, key handling, run dispatch
│       ├── keys.go           # KeyMap struct and DefaultKeyMap (keyboard shortcut definitions)
│       ├── styles.go         # paletteFor(theme, env) → views.Palette; basePalette per theme
│       ├── render.go         # Header, env pill, tab bar, body orchestration; delegates to views/
│       ├── views/
│       │   ├── common.go     # Palette struct + shared render utilities (Style, SepLine, etc.)
│       │   ├── sidebar.go    # Sidebar(): search row, command list cards, keyboard hints
│       │   ├── detail.go     # Commands(), History(), EnvPane(): center panel per active tab
│       │   ├── output.go     # Output(): live terminal output + RECENT run list
│       │   ├── statusbar.go  # StatusBar(): 1-row bottom bar with command count and env
│       │   └── modal.go      # Modal(): centered production-confirm overlay
│       └── splash/
│           └── splash.go     # Splash animation model (logo → tagline → init messages → done)
```

---

## Key concepts

### Config layering (lowest → highest priority)
1. Hardcoded defaults (`config.Default()`)
2. Global file: `~/.config/cast/cast.toml`
3. Local file: `.cast.toml` in cwd
4. `CAST_ENV` environment variable
5. CLI flags (`-env`, `-theme`)

### Themes and environments
- Three color themes: **catppuccin** (default), **dracula**, **nord**
- Three environments: **local** (default accent), **staging** (orange accent), **prod** (red accent)
- `paletteFor(theme, env)` in `tui/styles.go` produces the resolved `views.Palette`

### Makefile parsing
Commands are discovered from `## name: description` comment lines above targets.
Bare targets without `##` comments are also included (description left empty).
Tags (`ci`, `go`, `prod`, etc.) and single-letter shortcuts are auto-inferred from the target name.

If `./Makefile` (or the configured source path) is missing from the working
directory, cast walks up to `source.lookup_depth` parent directories (default
`5` in `~/.config/cast/cast.toml`) looking for one. When found in a parent,
commands are executed with `make -C <dir> <target>` so recipes evaluate from
the Makefile's directory — this is the common case for monorepos and git
submodules where the workdir sits below the project root. Set
`lookup_depth = 0` to disable the walk-up.

Per-command flag tags recognized on the `## name: desc …` line:

| Tag | Effect |
|---|---|
| `[stream]` / `[no-stream]` | Force long-running log-following mode on/off |
| `[confirm]` / `[no-confirm]` | Force confirmation modal on/off regardless of env |
| `[interactive]` / `[no-interactive]` | Run with the real TTY attached: the TUI suspends, the target inherits stdin/stdout/stderr (use for `python3`, `bash`, `psql`, `vim`…), and resumes on exit. Implies `[no-stream]`. |
| `[sc=X]` / `[shortcut=X]` | Pin a keyboard shortcut letter |
| `[tags=a,b,c]` | Pin category tags |
| `[pick=SPEC]` | Open a folder picker before running (see *Pick flow*). |
| `[as=A,B,…]` | Optional alias names for the env/make vars produced by `[pick=…]`. Defaults to `CAST_PICK_1`, `CAST_PICK_2`, … |

### Pick flow

Commands tagged with `[pick=…]` open a centered folder picker before
executing. Each `*` in the spec is one picker step; selections are exposed
to the recipe both as make variables (`$(CAST_PICK_N)` / `$(ALIAS)`) and as
environment variables.

Spec grammar: `BASE/*[~FILTER]/*…` with `;` separating independent groups.

| Spec                         | Behavior                                               |
|------------------------------|--------------------------------------------------------|
| `./*`                        | Pick a folder in cwd.                                  |
| `./*~addons`                 | Pick a folder in cwd whose name contains `addons`.     |
| `./*~*addons*/*`             | First pick filtered by `*addons*`, then a subfolder.   |
| `services/*/*`               | Pick `services/<X>`, then `services/<X>/<Y>`.          |
| `./*~odoo; configs/*`        | Two independent picks: one in cwd (filter `odoo`), one in `./configs`. |

Filter syntax: `~name` is a substring match; `~*name*` enables `*` glob
semantics. Both are case-insensitive.

Use `[as=ROOT,MODULE]` to map the picks to friendly names instead of
`CAST_PICK_1` / `CAST_PICK_2`. The values stored are paths relative to cwd
(e.g. `services/api/v2`), so `cd $(ROOT)` works directly.

Cancel the modal with `esc`; `←` / empty-buffer backspace steps back to the
previous pick. The picker is also a fzf — type to filter and press `enter`
to select. Folder rows are decorated with content-aware glyphs (Odoo
modules, Git repos, Makefiles, `package.json`, …).

### Snippets library

Reusable Makefile targets live under `~/.config/cast/snippets/<name>.mk`,
one file per snippet. Each `.mk` carries the full target: a
`## name: desc [tags…]` doc-line followed by the `name:` declaration and
its tab-indented recipe. Files are plain Make so editor syntax
highlighting and dotfile sharing work without escaping.

The `library` tab (5th tab in the TUI) browses the global collection with
fuzzy search and a side-by-side preview. Keybinds inside the tab:

| Key | Action |
|---|---|
| `↑/↓` `j/k` | Move cursor |
| `/` | Focus search |
| `⏎` | Insert into current Makefile (aborts on `ErrTargetExists`) |
| `d` | Arm delete; second `d` or `⏎` confirms |
| `esc` | Cancel delete or close tab |

From the `commands` tab, `ctrl+x` extracts the highlighted target and
saves it to the library (round-trips back through `library.Save`, which
normalises the `## name:` doc-line so the basename and the embedded name
stay in sync).

Code: `internal/library/library.go` exposes `Dir`, `EnsureDir`, `List`,
`Load`, `Save`, `Delete`, `SanitizeName`. Makefile-side helpers in
`internal/source/makefile.go`: `MakefileTargetLines`,
`AppendMakefileTarget`, `ExtractMakefileTarget`, plus errors
`ErrTargetExists` / `ErrTargetNotFound`.

### App mode: single vs chain

The TUI has two top-level modes toggled with `ctrl+s`. A pill in the header shows the active mode (cyan `SINGLE` or orange `CHAIN`).

- **Single mode** (default) — sidebar lists Makefile commands, history tab shows per-run history. Pressing `Enter` (or a command's shortcut) runs the command. If a command is already running, the next `Enter`/shortcut appends the target to the active chain instead of being rejected (auto-queue). `ctrl+a` opens the explicit **chain builder**: `space` (or shortcut letters) toggle commands with an accent bar + order number shown in the sidebar; `Enter` runs the chain; `esc` cancels.
- **Chain mode** — sidebar lists auto-saved chains (most recently executed first). `Enter` on a chain re-runs it. History tab shows per-chain-execution records (start, duration, status, steps).

Failure policy: if any step errors or is interrupted, remaining steps are dropped and the chain persists as failed. The currently-running chain is rendered at the top of the sidebar (`CHAIN (N)` block) with the running step marked `▶`.

Chains with ≥2 steps are auto-saved on completion. They dedupe by fingerprint (sha1 of the ordered command list) into a single `sequences` row, incrementing `run_count`. The total number of chain executions retained is capped by `history.chain_max` (default `100`) in `cast.toml`; older rows are pruned.

Schema: reuses the pre-existing `sequences`/`sequence_steps`/`sequence_runs` tables from `001_init.sql`. Auto-saved chains live under the synthetic name `auto:<sha1-prefix>`.

### TUI data flow
```
main.go
  └─ tea.NewProgram(tui.New(cfg, commands))
       └─ model.Update(msg)
            ├─ tea.KeyPressMsg → handleKey → triggerRun → dispatchRun
            │    └─ tea.Batch(RunStartMsg, runner.Run(name) → RunDoneMsg)
            ├─ RunOutputMsg → append to m.output
            └─ RunDoneMsg   → prepend to m.history, stop spinner
       └─ model.View()
            └─ renderMain() → renderHeader + renderBody + views.StatusBar
                 └─ renderBody → views.Sidebar | renderCenter | views.Output
                      └─ renderCenter → views.Commands | views.History | views.EnvPane
```

### Package import graph
```
cmd/cast  →  tui, config, source
tui       →  views, runner, source, config, splash
views     →  runner, source, lipgloss
runner    →  bubbletea
source    →  (stdlib only)
config    →  toml
splash    →  bubbletea, lipgloss, bubbles
```
No circular imports. The `views` package never imports `tui`.

### Views package (`internal/tui/views`)
All view functions are pure: they receive explicit data structs (Props) and a `Palette`, and return a `string`. No model state is accessed directly. This makes them trivially testable and keeps render logic decoupled from Bubble Tea.

### History persistence
`runner.LoadHistory(path)` / `runner.SaveHistory(path, records, max)` read/write JSON.  
The history path comes from config (`cfg.HistoryPath`). Hook `SaveHistory` into the `RunDoneMsg` handler in `model.go` once the path is wired through.

---

## CLI subcommands

| Command | Description |
|---|---|
| `cast` | Launch TUI |
| `cast init [env]` | Create `.cast.toml` template for `dev`, `staging`, or `prod` |
| `cast config` | Print resolved config paths and active values |

Flags: `-env local\|staging\|prod`, `-theme catppuccin\|dracula\|nord`

---

## Versioning

The CLI version lives in a single constant: `internal/version/version.go`
(`version.Current`). It is read by the splash screen and by any surface that
needs to display the current build.

**Bump the version on every user-visible change.** Follow semver:

- `PATCH` (0.4.0 → 0.4.1): bug fixes, copy tweaks, internal refactors with no
  change to observable behavior.
- `MINOR` (0.4.x → 0.5.0): new feature, new keybinding, new CLI subcommand,
  new Makefile tag, or any additive change users could notice.
- `MAJOR` (0.x.y → 1.0.0): breaking change to config schema, CLI flags, or
  Makefile tag grammar.

Edit `version.Current` in the same commit that introduces the change — never
in a separate bookkeeping commit. Current: `0.17.0`.

---

## Changelog (IMPORTANT)

Every version bump in `internal/version/version.go` MUST be accompanied by a
matching entry in `CHANGELOG.md` at the repo root. Same commit. No exceptions.

Rules:

- **Always written in English**, regardless of the language used in the
  conversation that produced the change.
- **One section per version**, newest first. Heading format:
  `## [<version>] – YYYY-MM-DD`.
- Group related items under sub-headings: `### Added`, `### Changed`,
  `### Fixed`, `### Removed`. Skip headings that have no entries.
- Each item is a one-line summary. Lead with the surface that changed
  ("Library tab:", "Picker:", "Theme tab:") so the reader can scan.
- **Include usage examples for new user-facing features.** A new keybinding,
  Makefile tag, CLI subcommand, or config key without a usage snippet
  forces the next reader to dig through code. Use fenced code blocks (Make,
  TOML, or shell) right under the bullet that introduced it.
- Bug fixes name the symptom and the version that introduced the bug when
  it's known ("Fixed 0.16.0 regression where …").
- Behaviour changes that affect existing users go in `### Changed` with a
  short rationale. Keep it user-facing — internal refactors with no visible
  effect don't deserve an entry.

Format reference: <https://keepachangelog.com/en/1.1.0/>. We follow its
spirit, not its exact rules.

When the user asks for a change without bumping the version (purely
exploratory or aborted work), do not edit the changelog. The contract is:
version bump ↔ changelog entry, both or neither.

---

## Icon convention (IMPORTANT)

**All icons in cast must be Nerd Font glyphs.** The default icon style is
`nerdfont`; an `emoji` fallback exists (`[ui] icons = "emoji"` in
`~/.config/cast/cast.toml`) for users without a Nerd Font–patched terminal,
but every new icon must be added to `internal/tui/views/icons.go` —
**Nerd Font codepoint first**, with an emoji fallback alongside it. Never
hard-code an emoji directly in a view file.

When adding a new icon:

1. Pick the Nerd Font glyph from <https://www.nerdfonts.com/cheat-sheet>.
2. Add a field to `IconSet` in `internal/tui/views/icons.go` (Nerd Font
   value in the `IconNerdFont` branch, emoji fallback in `IconEmoji`).
3. Reference it from the view via `Icons(style).<Field>` — never inline a
   literal glyph in a view, model, or other package.
4. If you must show an icon outside the views package (e.g. picker icon
   resolution in `internal/tui/picker.go`), pass `views.IconStyle` in and
   resolve through `views.Icons(style)` so the user's preference wins.

The model carries the resolved `views.IconStyle` (parsed from `cfg.IconStyle`
at `tui.New`) and propagates it to views via Props.

---

## Build

```bash
make build          # → bin/cast
make run            # build + run
make test           # go test ./...
make lint           # golangci-lint
```

---

## Deep-dive docs

These topics live in `docs/ai/` to keep this file short. Read the matching
file only when the task requires it.

| File | When to read |
|---|---|
| [`docs/ai/lipgloss-pitfalls.md`](docs/ai/lipgloss-pitfalls.md) | A view is misaligned, a bordered box lost its indent, or a chip/badge looks cut at a panel boundary. |
| [`docs/ai/debugging-views.md`](docs/ai/debugging-views.md) | You need to reproduce a rendering bug in `internal/tui/views` without running the full TUI (throwaway `cmd/debugview` technique, ANSI stripping, width comparison). |

When adding a new non-trivial topic (a gotcha, a debugging recipe, a subsystem
deep-dive) that doesn't belong in code comments, create a new file in
`docs/ai/` and add a one-line row to the table above. Keep each file focused
on a single topic so future agents can pull in only what they need.

---

## WIP / known gaps

- **Streaming output**: `runner.StreamRun` exists but is not yet wired; `dispatchRun` uses the sync `runner.Run`. Wire `program.Send` once the program reference is accessible.
- **History persistence**: `LoadHistory`/`SaveHistory` are implemented but not yet called. Wire in `main.go` (load on start) and `model.go` `RunDoneMsg` handler (save on each run).
- **TabEnv** (.env viewer) is a placeholder stub.
