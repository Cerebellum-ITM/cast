# demo — README GIFs (VHS)

The animated GIFs in the project README are recorded with
[charmbracelet/vhs](https://github.com/charmbracelet/vhs) from the `.tape`
scripts in this folder, so anyone can regenerate them.

The `ai-annotate` GIF is a **simulation**: `sim/cast-sim.sh` reproduces the
`cast ai annotate --dry-run` output with invented data (the real command needs a
Groq API key + network, so it can't run in a reproducible recording). The typed
command on screen is real; the output below it is faithful but illustrative.
`tui` and `custom-makefile` record the **real** `cast` binary.

## Requirements

```sh
brew install vhs ttyd ffmpeg     # vhs needs ttyd + ffmpeg
```

A [Nerd Font](https://www.nerdfonts.com/) must be installed (the tapes use
`JetBrainsMono Nerd Font Mono`) for glyphs to render.

## Regenerate

Run from the **repo root** (tape paths are repo-relative). The tapes expect the
demo binary at `demo/.bin/cast` and a sample project under `demo/example/`:

```sh
go build -o demo/.bin/cast ./cmd/cast      # build the binary the tapes invoke

vhs demo/tapes/tui.tape                     # hero
vhs demo/tapes/ai-annotate.tape             # simulated ai annotate
vhs demo/tapes/custom-makefile.tape         # -f alternate Makefile
```

Output lands in `demo/gifs/`. `tapes/_setup.tape` holds shared settings + hidden
shell prep (truecolor, isolated `HOME`, clean prompt, `cd demo/example`); every
per-command tape `Source`s it and sets its own `Width`/`Height`.

## Layout

```
demo/
├── .bin/            # built cast binary the tapes call (git-ignored)
├── example/         # sample project: Makefile + Makefile.personal
├── gifs/            # rendered output (committed; embedded in the README)
├── sim/             # simulation shims (cast-sim.sh)
└── tapes/           # VHS scripts (_setup.tape + one per GIF)
```
