# Unit 03: `cast -f` — run against an arbitrary Makefile

## Goal

Let the user point cast at a Makefile other than the default `Makefile`
(e.g. `Makefile.personal`) so that **everything** — discovery, sidebar,
preview, `ai annotate`, `tags`/`shortcut`, and crucially **execution** —
operates against that file. Selectable three ways, layered like the rest
of cast: local `[source].path` < `CAST_MAKEFILE` env < `-f/--file` flag.
Selection is **always explicit** — cast never auto-prefers an alternate
file by convention.

The critical correctness requirement: today the runner invokes
`make -C <dir> <target>` **without `-f`**, so it would parse the custom
file but run the default `Makefile`. This unit makes the runner pass
`-f <basename>` so cast executes exactly the file it parsed — which also
eliminates the latent `GNUmakefile` > `Makefile` precedence footgun.

## Design

No new UI tokens or views. The active file is already surfaced: the
status bar (`views.StatusBar`) renders `m.makefilePath`, and `cast config`
prints the resolved `source`. After this unit, both reflect the
custom file, which is the visible verification surface.

### Resolution layering (extends the fixed 5-layer model)

The source-file path resolves in this order (later wins), consistent with
architecture invariant #5:

1. **default** — `./Makefile` (unchanged).
2. **global TOML** — _not used_ for the path (a global makefile name is
   meaningless across projects); `[source].lookup_depth` stays as-is.
3. **local `.cast.toml`** — `[source] path = "Makefile.personal"`.
4. **env** — `CAST_MAKEFILE=Makefile.personal`.
5. **flag** — `-f` / `--file Makefile.personal`.

After the path is chosen, the existing `resolveSourcePath` walk-up runs
unchanged — it already tries `<ancestor>/<rel-name>` at each level, so a
custom relative name (`Makefile.personal`, `build/Makefile.ci`) is found
in cwd or a parent within `lookup_depth`. If the resolved file does not
exist, behaviour matches today: the TUI shows a warning and an empty
sidebar; mutating subcommands error.

`[source].type` stays implicitly `"makefile"`; non-makefile sources
remain out of scope (see `project-overview.md`).

## Implementation

### `internal/config/config.go` + `toml.go`

- Replace the positional `Load(flagEnv, flagTheme string)` with an
  options struct so the source override threads cleanly:

  ```go
  type LoadOptions struct {
      Env        string // CLI -env override ("" = unset)
      Theme      string // CLI -theme override ("" = unset)
      SourceFile string // CLI -f/--file override ("" = unset)
  }

  func Load(opts LoadOptions) (*Config, error)
  ```

  Update **every** caller (see cmd/cast below). Existing
  `config.Load("", "")` calls become `config.Load(LoadOptions{})`.

- Source-path layering inside `Load`, applied **before**
  `resolveSourcePath`:
  1. start from `cfg.SourcePath` (default `./Makefile`);
  2. if local `[source].path != ""` → use it;
  3. if `os.Getenv("CAST_MAKEFILE") != ""` → use it;
  4. if `opts.SourceFile != ""` → use it.
  Then run the existing `resolveSourcePath(cfg.SourcePath, depth)` and
  set `cfg.SourceDir = filepath.Dir(cfg.SourcePath)`.

- New `cfg.SourceFile = filepath.Base(cfg.SourcePath)` — the basename the
  runner passes to `make -f`.

- `toml.go`: uncomment/define the local `[source]` block:

  ```go
  type LocalSource struct {
      Path string `toml:"path"` // makefile path relative to the project root
      // Type is reserved; only "makefile" is honoured this unit.
  }
  ```
  Add `Source LocalSource \`toml:"source"\`` to `LocalFile` and document
  `[source] path` in `LocalTemplateSrc` (replace the WIP comment block).

### `internal/runner/runner.go`

- Thread the makefile basename into the make invocation. Extend
  `makeArgs` and the `Run` / `StreamRun` signatures to take a `file`
  argument:

  ```go
  func makeArgs(dir, file, target string, extraVars []string) []string
  ```
  Emit `-C <dir>` then `-f <file>` (when `file != ""`) then vars then
  target: `make -C <dir> -f <file> KEY=VAL <target>`. Because `-C`
  changes directory first, `file` is the **basename** living in `<dir>`.

- **Always** pass `-f <basename>` (even for the default `Makefile`) so
  cast runs exactly what it parsed and the `GNUmakefile`/`makefile`/
  `Makefile` precedence ambiguity disappears. Update the godoc on
  `makeArgs` accordingly.

### `internal/tui/model.go`

- Store the basename: new field `makefileFile string`, set in `New()`
  from `cfg.SourceFile` (next to `makefileDir`).
- Pass `m.makefileFile` at every `runner.Run` / `runner.StreamRun` call
  site (single runs and chain steps). No behavioural change beyond the
  added `-f`.

### `cmd/cast/main.go` (and subcommand files)

Honour the chosen file across **all** subcommands. Local `[source].path`
and `CAST_MAKEFILE` are read inside `config.Load`, so they already reach
every subcommand for free. The `-f/--file` flag is made uniform with a
single pre-parse:

- Add `parseSourceFlag(args []string) (file string, rest []string)` that
  strips the first `-f`/`--file` occurrence (both `-f X` and `-f=X` /
  `--file=X` forms) from `args`, returning the value and the remaining
  args. No `flag.FlagSet` churn in each subcommand.
- In `main()`: `srcFile, args := parseSourceFlag(os.Args[1:])`. Dispatch
  the subcommand switch on `args` (so `cast -f X ai annotate` and
  `cast ai annotate -f X` both work), and pass `srcFile` down to each
  `run*` dispatcher, which forwards it into `config.Load(LoadOptions{…,
  SourceFile: srcFile})`.
- TUI path: build `LoadOptions{Env, Theme, SourceFile: srcFile}` from the
  parsed `-env`/`-theme` flags plus `srcFile`.
- Update `usage` with `-f, --file string  use an alternate Makefile
  (default ./Makefile)` and a one-line note that `CAST_MAKEFILE` and
  `[source] path` in `.cast.toml` do the same, layered.
- `runConfig` already prints `source` — verify it shows the custom path.

## Dependencies

- None new. All stdlib (`flag`, `os`, `path/filepath`, `os/exec`).

## Verify when done

- [ ] `cast -f Makefile.personal` launches the TUI with the targets from
      `Makefile.personal`; the status bar shows that path. Running a
      target executes its recipe from `Makefile.personal` (verify with a
      target that prints something unique to that file), not from a
      sibling `Makefile`.
- [ ] `CAST_MAKEFILE=Makefile.personal cast config` prints the custom
      path as `source`; unsetting it reverts to `./Makefile`.
- [ ] `[source]\npath = "Makefile.personal"` in `.cast.toml` makes every
      surface (TUI + `cast tags list` + `cast ai annotate --dry-run`)
      operate on that file with no flag/env.
- [ ] Layering holds: local `[source].path` is overridden by
      `CAST_MAKEFILE`, which is overridden by `-f`.
- [ ] `cast ai annotate -f Makefile.personal --dry-run` diffs against
      `Makefile.personal`; applying writes to that file only.
- [ ] With both `Makefile` and `GNUmakefile` present, running a target
      executes the file cast parsed (proven by `-f`), not make's default
      precedence pick.
- [ ] A non-existent `-f` path yields the same warning/empty-sidebar
      behaviour as a missing default Makefile (no panic).
- [ ] Walk-up still works: `cast -f Makefile.personal` from a subdir
      finds the file in a parent within `lookup_depth`.
- [ ] `make build` passes.
- [ ] `make test` passes (add a `runner.makeArgs` test covering the `-f`
      argument ordering, and a `config.Load` test for the source layering).
- [ ] `make lint` produces no new findings.
- [ ] `version.Current` bumps to `0.26.0` (additive, MINOR) with a
      matching `CHANGELOG.md` entry showing `-f`, `CAST_MAKEFILE`, and the
      `[source] path` TOML snippet.
- [ ] `context/architecture.md` — note the source-file layering under
      *System Boundaries*/invariant #5 and that the runner always passes
      `make -f`. `context/project-overview.md` — Features bullet.
      `context/progress-tracker.md` — move to Completed after merge.

## Open questions resolved up front

1. **Selection mechanism** — all three (flag + local config + env),
   layered local < env < flag. _(decided)_
2. **Auto-detection** — none; selection is always explicit. cast keeps
   defaulting to `Makefile`. _(decided)_
3. **Subcommand scope** — every subcommand honours the file (local + env
   via `config.Load`; the `-f` flag via the `main()` pre-parse).
   _(decided)_
