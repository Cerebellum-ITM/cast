# _setup.tape — shared VHS settings + hidden shell prep.
# Each per-command tape does `Source demo/tapes/_setup.tape`.
# Run vhs from the REPO ROOT (paths below are repo-relative).

Set Shell bash
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 16
# Width/Height are set per-tape (before Source) so each GIF is sized to fit.
Set Padding 22
Set Margin 16
Set BorderRadius 10
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set PlaybackSpeed 1.0

Hide
# Truecolor so cast's hex palette renders exactly (no 256-color downsampling).
Type "export COLORTERM=truecolor TERM=xterm-256color" Enter
# Remember the repo root (for sourcing the sim script after we cd away), put
# the freshly built demo binary first on PATH, then isolate HOME so cast starts
# from clean defaults (fresh ~/.config/cast, empty history db).
Type `export CASTREPO="$PWD"` Enter
Type `export PATH="$PWD/demo/.bin:$PATH"` Enter
Type `export HOME="$(mktemp -d)"` Enter
Type `export PS1='\[\e[38;5;141m\]❯\[\e[0m\] '` Enter
Type "cd demo/example" Enter
Type "clear" Enter
Show
Sleep 700ms
