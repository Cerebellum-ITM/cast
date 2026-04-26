# Changelog

All notable changes to **cast** are recorded here. Format inspired by
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); cast follows
semantic versioning (see `CLAUDE.md` → *Versioning*).

Each entry is keyed by the value of `version.Current`
(`internal/version/version.go`) at the time the change shipped. Newest
versions on top.

## [0.17.0] – 2026-04-26

### Changed

- **Library tab is now full-width.** The sidebar (commands list) and the
  output panel are hidden while the library tab is active so the snippet
  list and preview have the whole terminal width.
- **Toast moved from the status bar to a header pill.** Internal events
  (snippet saved, theme persisted, etc.) now appear as a rounded-border
  pill on the right side of the header, immediately to the left of the
  mode pill (`SINGLE`/`CHAIN`). Severity glyphs: `✓` success (green),
  `⚠` error (red), `·` info (accent). Long messages truncate at 48
  columns; the pill auto-fades after 4 seconds.

## [0.16.1] – 2026-04-26

### Fixed

- **Snippet extraction was capturing `.PHONY` continuation lines instead
  of the real recipe** (regression in 0.16.0). When a Makefile declared
  `.PHONY: build run … \` followed by space-indented continuations,
  `findTargetIndex` matched a continuation line as the target itself and
  the saved `.mk` ended up empty (no recipe). Fixed by hardening
  `findTargetIndex` to reject any indented line and to require the name
  to appear as a whole token before the `:`. Regression test added.

## [0.16.0] – 2026-04-25

### Added

- **Snippets library** (`library` tab — the 5th tab). A per-user
  collection of reusable Makefile targets stored under
  `~/.config/cast/snippets/<name>.mk` (one file per snippet, plain Make).

  Usage:

  ```text
  ctrl+x   on a target in the commands tab → save it to the library
           (extracts the doc-line, target line and recipe verbatim)
  Tab→library  →  browse with fuzzy search and a side-by-side preview
  Enter   on a snippet  →  append to the current Makefile
                          (aborts with ErrTargetExists on conflict)
  d (×2)  on a snippet  →  delete the .mk file
  ```

  ```bash
  # The on-disk format is just plain Make:
  cat ~/.config/cast/snippets/deploy.mk
  ## deploy: Despliega un servicio [pick=services/*]
  deploy:
  	@./deploy.sh $(CAST_PICK_1)
  ```

- New keybinding `ctrl+x` (`KeyMap.ExtractSnippet`) — extracts the
  highlighted target to the global library.

- Public Makefile helpers in `internal/source/makefile.go`:
  `MakefileTargetLines`, `AppendMakefileTarget`, `ExtractMakefileTarget`,
  plus typed errors `ErrTargetExists` / `ErrTargetNotFound`.

## [0.15.5] – 2026-04-25

### Changed

- **TUI is now transparent over the terminal background.** All structural
  surfaces (panels, dividers, header bands, hint rows, pills) drop their
  explicit `Background()` so they show the user's terminal bg directly.
  Selection (`BgSelected`), shortcut/tag chips, and modal overlays remain
  opaque on purpose. Eliminates the multi-tone banding that was
  particularly visible on Nord.

## [0.15.4] – 2026-04-25

### Fixed

- Unified all panel surfaces to `BgPanel` (output, detail, env, expanded
  popups). Previous mix of `BgPanel` / `BgDeep` / `Bg` produced visible
  horizontal stripes between header, term area, hint row, and RECENT.

## [0.15.3] – 2026-04-25

### Fixed

- Output panel: replaced `BgDeep` with `BgPanel` everywhere so header,
  term, hint, and RECENT rows share one surface.

## [0.15.2] – 2026-04-25

### Fixed

- Wrapped the entire frame in `Background(p.Bg)` and forced dividers to
  `Background(p.Bg)` so empty cells and `│` separators no longer show the
  terminal default colour through panel gaps.

## [0.15.1] – 2026-04-25

### Fixed

- **Nord theme:** `Border` (`#3B4252`) was identical to `BgPanel`,
  rendering every separator and rounded box invisible. Bumped to
  `#4C566A` (Nord polar-night-3).
- Env pill (`DEV/STG/PRD`) uses palette tokens (`p.Green`, `p.Orange`,
  `p.Red`) instead of hard-coded Catppuccin hex. Each theme now provides
  its own variant.

## [0.15.0] – 2026-04-25

### Added

- **Theme tab is fully implemented.** Live preview as you navigate; press
  `Enter` to persist `theme = "<name>"` in the local `.cast.toml`.

  ```toml
  # .cast.toml at the project root
  theme = "nord"
  ```

- New TOML field `theme` in `LocalFile`, plus `WriteLocalTheme(path,
  theme)` writer that preserves comments and other sections.

### Changed

- Config priority for `Theme`: CLI flag > local `.cast.toml` > global
  per-env override > global default.

## [0.14.0] – 2026-04-25

### Added

- **Confirm modal is keyboard-navigable.**

  ```text
  ←/→ or h/l   move focus between [cancel] and [confirm]
  tab          toggle
  ⏎            commit the focused button
  y            yes (direct hotkey)
  n / esc      no  (direct hotkey)
  ```

  Default focus is `confirm` so existing single-press `Enter` flows still
  work. The focused button shows a `▸` cursor and bold weight; the other
  reserves the same column so layout doesn't jump on movement.

## [0.13.2] – 2026-04-25

### Changed

- When a chain (≥2 steps) finishes — including auto-queued chains created
  via shortcuts while another command was running — the full intent
  (`chainCommands`) is now persisted as both the sequence record AND the
  rerun card target. `Ctrl+R` and the rerun card replay the entire chain,
  even if a middle step failed.

## [0.13.1] – 2026-04-25

### Changed

- **Per-project last-run state moved from `.cast.last.json` to SQLite.**
  New table `project_last_runs(project_dir, commands, extra_vars,
  updated_at)` keyed by absolute Makefile dir. Removed the file-system
  side-channel and its `.gitignore` entry.

## [0.13.0] – 2026-04-25

### Added

- Rerun card now supports **chains** in addition to single commands.
  Single dispatches show `↻ build · last command`; chains show
  `↻ a › b › c · last chain (3)`. `Ctrl+R` and `Enter` on the card
  replay the saved sequence.
- `state.LastRun` schema: `Commands []string` (replaces `Command
  string`), with backward-compat read for files written by 0.12.0.

## [0.12.0] – 2026-04-25

### Added

- **Pinned "rerun" card** at the top of the command sidebar with a yellow
  accent. Persists across CLI restarts so the first `Ctrl+R` after
  reopening cast in a project replays the last command without
  re-prompting.

  ```text
  ↑ from the first command  →  focus the rerun card
  ⏎ on the card             →  rerun (skips picker if extras cached)
  ```

## [0.11.0] – 2026-04-25

### Added

- `Ctrl+R` now repeats the last command, **including pick-typed
  commands** — the previous picker selections are reused, the modal does
  not reopen.

### Fixed

- `Ctrl+R` was being intercepted by the `Run/RunAlt` key case before
  reaching `RerunLast`. Re-ordered the switch so `RerunLast` wins when
  the bindings collide.

## [0.10.0] – 2026-04-25

### Added

- **Configurable icon set.** New `[ui] icons` field in
  `~/.config/cast/cast.toml`:

  ```toml
  [ui]
  icons = "nerdfont"   # default; "emoji" for terminals without Nerd Font
  ```

- Central `IconSet` registry in `internal/tui/views/icons.go`. Every new
  icon must be added here (Nerd Font codepoint canonical, emoji
  fallback) and consumed via `Icons(style).<Field>` — no inline glyphs
  in views.

## [0.9.0] – 2026-04-25

### Added

- **`[pick=...]` Makefile tag.** Opens a folder picker before running
  commands that require directory input. Each `*` is one picker step.

  ```make
  ## test:    Run Odoo tests           [pick=./*~addons/*] [as=ROOT,MOD]
  ## deploy:  Deploy a service         [pick=services/*]
  ## sync:    Two independent picks    [pick=./*~odoo; configs/*]
  test:
  	cd $(ROOT) && python -m pytest $(MOD)
  ```

  Selections are exposed both as Make variables (`$(CAST_PICK_1)` /
  `$(ALIAS)`) and as environment variables. Cancel with `esc`; `←` /
  empty-buffer backspace steps back to the previous pick. The picker is
  a fzf — type to filter — and rows show content-aware glyphs (Odoo
  modules, Git repos, Makefiles, `package.json`, …).
