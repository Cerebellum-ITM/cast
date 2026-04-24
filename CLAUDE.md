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

## Build

```bash
make build          # → bin/cast
make run            # build + run
make test           # go test ./...
make lint           # golangci-lint
```

---

## WIP / known gaps

- **Streaming output**: `runner.StreamRun` exists but is not yet wired; `dispatchRun` uses the sync `runner.Run`. Wire `program.Send` once the program reference is accessible.
- **History persistence**: `LoadHistory`/`SaveHistory` are implemented but not yet called. Wire in `main.go` (load on start) and `model.go` `RunDoneMsg` handler (save on each run).
- **TabEnv** (.env viewer) and **TabTheme** (interactive theme picker) are placeholder stubs.
- **Ctrl+R** re-run last command is a TODO stub in `handleKey`.
