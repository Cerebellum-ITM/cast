# cast

## Overview

**cast** is a terminal task-runner with a Bubble Tea TUI. It reads `Makefile`
targets from the current working directory (or walks up to a configurable
depth to find one), lets the user browse, search and run them, and streams
output in real time. It also persists per-run history, supports reusable
snippet libraries, and can chain commands into named sequences.

Module: `github.com/Cerebellum-ITM/cast`
Runtime: Go 1.25+
UI framework: `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2`

## Goals

1. Make every `Makefile` target a first-class, discoverable command with
   description, tags, and a single-key shortcut — no need to memorize
   target names.
2. Stream command output live inside the TUI with a real-time progress
   bar, while persisting structured run history for later inspection.
3. Support production-safety guardrails (confirmation modal, env-aware
   accents) so the same TUI can run against `local`, `staging`, and
   `prod` without accidents.
4. Provide reusable building blocks (snippet library, chains, pickers)
   so common operational workflows can be assembled without re-typing.

## Core User Flow

1. User launches `cast` in a project directory.
2. cast resolves config in layers (defaults → `~/.config/cast/cast.toml` →
   `./.cast.toml` → `CAST_ENV` → CLI flags) and locates a `Makefile`
   (walking up parent directories up to `source.lookup_depth`).
3. The splash animation plays while the Makefile is parsed into commands.
4. The TUI opens on the `commands` tab: sidebar with searchable commands,
   center panel with the highlighted command's preview, output panel for
   live execution.
5. User presses a command's shortcut (or `Enter` on the highlighted row)
   to run it. If `[interactive]`, the TUI suspends and the target inherits
   the real TTY. If `[pick=…]`, a folder picker opens first.
6. Output streams into the output panel with a real-time progress bar;
   on completion, a `RunRecord` is prepended to history and persisted.
7. User can switch to the `history`, `env`, `theme`, or `library` tabs;
   toggle single ↔ chain mode with `ctrl+s`; or build an explicit chain
   with `ctrl+a` and run multiple commands in sequence.

## Features

### Command discovery & execution

- Parse `Makefile` targets, including `## name: desc [tags…]` doc-lines
  with auto-tag and auto-shortcut inference.
- Per-command flag tags: `[stream]`/`[no-stream]`, `[confirm]`/`[no-confirm]`,
  `[interactive]`/`[no-interactive]`, `[sc=X]`, `[tags=a,b,c]`, `[pick=SPEC]`,
  `[as=A,B,…]`.
- Walk-up to find a `Makefile` in parent directories; execute with
  `make -C <dir> <target>` so recipes evaluate from the right root.

### AI annotation

- `cast ai annotate` (CLI) and `ctrl+i` (TUI `commands` tab) propose the
  missing `## name: desc [tags=…]` doc-lines for a Makefile via an LLM
  (Groq by default, configured in the `[ai]` config section). Scope is
  limited to description + categorical tags — behavioural flags
  (`[stream]`, `[confirm]`, …) are never inferred. Both surfaces show a diff
  and require explicit confirmation before writing; the API key is read from
  an env var and never stored in config.

### Pick flow

- Centered folder picker triggered by `[pick=…]`, with substring/glob
  filters, multi-step picks (`./*~addons/*`), independent groups (`;`),
  and friendly aliases via `[as=…]`.
- Picks are exposed to the recipe as both make variables and env vars.

### Snippet library

- Reusable Makefile targets stored under `~/.config/cast/snippets/<name>.mk`.
- Browse with side-by-side preview in the `library` tab; insert into the
  current Makefile with `Enter`; delete with `dd`.
- `ctrl+y` on the `commands` tab extracts the highlighted target into the
  library, normalising the doc-line.

### Chains

- Toggle single/chain mode with `ctrl+s`; build a chain explicitly with
  `ctrl+a` or implicitly by pressing shortcuts during a run (auto-queue).
- Chains with ≥2 steps auto-save on completion, deduped by sha1 of the
  ordered command list; capped by `history.chain_max`.

### Run history

- Per-run records persisted to JSON (`runner.LoadHistory` /
  `SaveHistory`); per-chain executions persisted to the SQLite
  `sequence_runs` table.

### Theming & env safety

- Three color themes (catppuccin, dracula, nord) and three environments
  (local, staging, prod) with env-aware accents.
- Confirmation modal forced on for `prod` (and overridable per command).

## Scope

### In Scope

- Reading `Makefile` targets from the cwd or a parent within
  `source.lookup_depth`.
- Running commands synchronously (`runner.Run`) and streaming live
  (`runner.StreamRun`).
- Persisting run history (JSON) and chain history (SQLite).
- Snippet library stored under `~/.config/cast/snippets/`.
- Pickers, chains, themes, env modes.

### Out of Scope

- Non-Make build systems (Bazel, Just, npm scripts, Taskfile). The
  source interface (`source.CommandSource`) is generic, but only the
  Makefile implementation exists.
- Remote execution. cast only runs commands on the local machine.
- Editing Makefile targets in-place (only the snippet library writes
  back; `ctrl+y` extracts, it does not edit recipes).

## Success Criteria

1. Launching `cast` in a directory with a `Makefile` shows all targets
   with their `## name: desc` lines parsed, including auto-tagged and
   auto-shortcutted entries.
2. A target running under `[stream]` produces live output in the output
   panel and updates a real-time progress bar.
3. Running a `[pick=…]` target opens the picker, propagates the
   selection as `$(CAST_PICK_N)` / `$(ALIAS)` make variables, and
   executes the recipe with the chosen path.
4. A chain of ≥2 commands runs in order, fails fast on any step error,
   and auto-saves under `auto:<sha1-prefix>` in the `sequences` table.
5. Switching `-env prod` re-tints the TUI with the prod accent and
   forces the confirmation modal on for every command without
   `[no-confirm]`.
