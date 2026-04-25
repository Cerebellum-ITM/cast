package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ── Global config structs ────────────────────────────────────────────────────

// GlobalFile mirrors ~/.config/cast/cast.toml.
type GlobalFile struct {
	Theme   GlobalTheme   `toml:"theme"`
	History GlobalHistory `toml:"history"`
	DB      GlobalDB      `toml:"db"`
	Layout  LayoutSection `toml:"layout"`
	Source  GlobalSource  `toml:"source"`
	UI      GlobalUI      `toml:"ui"`

	// WIP: Keybindings   GlobalKeybindings   `toml:"keybindings"`
	// WIP: Notifications GlobalNotifications `toml:"notifications"`
	// WIP: Update        GlobalUpdate        `toml:"update"`
}

// GlobalUI configures TUI presentation knobs that aren't tied to a theme.
type GlobalUI struct {
	// Icons selects the glyph set: "nerdfont" (default) or "emoji". Empty =
	// inherit. Unknown values fall back to nerdfont.
	Icons string `toml:"icons"`
}

// GlobalSource controls how cast locates the task-source file (Makefile).
// LookupDepth=0 disables the walk-up and requires the file in cwd. Default 5.
type GlobalSource struct {
	LookupDepth int `toml:"lookup_depth"`
}

// GlobalTheme controls which theme is active per environment.
type GlobalTheme struct {
	Default string            `toml:"default"`
	Env     map[string]string `toml:"env"`
}

// GlobalHistory controls command-run history retention.
type GlobalHistory struct {
	Max      int `toml:"max"`
	ChainMax int `toml:"chain_max"`
}

// GlobalDB configures the SQLite database location.
type GlobalDB struct {
	Path string `toml:"path"`
}

// LayoutSection controls TUI panel sizing. Shared between global and local.
// Zero/nil fields mean "inherit from the layer below".
type LayoutSection struct {
	// OutputWidthPct: width of the right (output) panel as % of total.
	OutputWidthPct int `toml:"output_width_pct"`
	// SidebarWidthPct: width of the left (sidebar) panel as % of total.
	SidebarWidthPct int `toml:"sidebar_width_pct"`
	// ShowCenterPanel: when false, the middle detail panel is hidden and
	// sidebar + output share the full width. Pointer so omitted == inherit.
	ShowCenterPanel *bool `toml:"show_center_panel"`
}

// ── WIP global structs (uncommented when ready) ──────────────────────────────

// GlobalKeybindings maps action names to key characters.
// type GlobalKeybindings struct {
// 	Build string `toml:"build"`
// 	Test  string `toml:"test"`
// 	Lint  string `toml:"lint"`
// }

// GlobalNotifications controls desktop/system notification behaviour.
// type GlobalNotifications struct {
// 	Enabled   bool `toml:"enabled"`
// 	OnFailure bool `toml:"on_failure"`
// 	OnSuccess bool `toml:"on_success"`
// }

// GlobalUpdate controls auto-update checks.
// type GlobalUpdate struct {
// 	Check   bool   `toml:"check"`
// 	Channel string `toml:"channel"`
// }

// ── Local config structs ─────────────────────────────────────────────────────

// LocalFile mirrors .cast.toml found in the working directory.
type LocalFile struct {
	Theme    string        `toml:"theme"` // catppuccin | dracula | nord
	Env      LocalEnv      `toml:"env"`
	Commands LocalCommands `toml:"commands"`
	Layout   LayoutSection `toml:"layout"`

	// WIP: Source  LocalSource  `toml:"source"`
	// WIP: Project LocalProject `toml:"project"`
}

// LocalCommands holds project-level command configuration.
type LocalCommands struct {
	Confirm   LocalConfirm      `toml:"confirm"`
	Shortcuts map[string]string `toml:"shortcuts"` // command name → single-char shortcut
}

// LocalConfirm lists command names that always require a confirmation modal.
type LocalConfirm struct {
	Targets []string `toml:"targets"`
}

// LocalEnv points to the .env file this project uses and declares its environment.
type LocalEnv struct {
	Name string `toml:"name"` // dev | staging | prod
	File string `toml:"file"` // path to .env file (relative to this config)
	// WIP: Type string `toml:"type"` // dotenv | direnv | chamber | ssm
}

// ── WIP local structs (uncommented when ready) ───────────────────────────────

// LocalSource overrides which task-source file cast reads.
// type LocalSource struct {
// 	Type string `toml:"type"` // makefile | taskfile | yaml
// 	Path string `toml:"path"`
// }

// LocalCommands holds project-level keyboard shortcut overrides.
// type LocalCommands struct {
// 	Shortcuts map[string]string `toml:"shortcuts"`
// }

// LocalProject holds metadata displayed in the TUI header (future).
// type LocalProject struct {
// 	Name string `toml:"name"`
// 	Team string `toml:"team"`
// }

// ── Paths ────────────────────────────────────────────────────────────────────

// GlobalPath returns the absolute path to the global config file.
func GlobalPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cast", "cast.toml")
}

// globalDir returns the directory that contains the global config.
func globalDir() string {
	return filepath.Dir(GlobalPath())
}

const localFileName = ".cast.toml"

// LocalPath returns the path where the local config is expected (cwd).
func LocalPath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, localFileName)
}

// ── Load functions ───────────────────────────────────────────────────────────

// LoadGlobal reads ~/.config/cast/cast.toml.
// If the file does not exist it is created with defaults before returning.
func LoadGlobal() (*GlobalFile, error) {
	p := GlobalPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err := EnsureGlobal(); err != nil {
			return nil, fmt.Errorf("creating global config: %w", err)
		}
	}

	var f GlobalFile
	if _, err := toml.DecodeFile(p, &f); err != nil {
		return nil, fmt.Errorf("reading global config %s: %w", p, err)
	}

	// Apply defaults for zero values so callers don't need nil checks.
	if f.Theme.Default == "" {
		f.Theme.Default = "catppuccin"
	}
	if f.History.Max == 0 {
		f.History.Max = 100
	}
	if f.History.ChainMax == 0 {
		f.History.ChainMax = 100
	}
	if f.Source.LookupDepth == 0 {
		f.Source.LookupDepth = 5
	}
	return &f, nil
}

// LoadLocal reads .cast.toml from the current working directory.
// Returns (nil, false) when no local config exists — not an error.
func LoadLocal() (*LocalFile, bool) {
	p := LocalPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil, false
	}

	var f LocalFile
	if _, err := toml.DecodeFile(p, &f); err != nil {
		return nil, false
	}
	return &f, true
}

// ── Write helpers ─────────────────────────────────────────────────────────────

// EnsureGlobal creates ~/.config/cast/ and writes the default global config
// if it does not already exist.
func EnsureGlobal() error {
	dir := globalDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(GlobalPath(), []byte(globalTemplate()), 0o644)
}

// WriteLocalTemplate writes the local config template to path with the given env name.
func WriteLocalTemplate(path, envName string) error {
	return os.WriteFile(path, []byte(LocalTemplateSrc(envName)), 0o644)
}

// WriteLocalTheme persists `theme = "<name>"` to the local .cast.toml,
// preserving the rest of the file. If the file does not exist it is created
// with a minimal stub. If a top-level `theme = "..."` line already exists it
// is replaced; otherwise a new line is inserted right after the leading
// comment block (or at the top if there is none). Other sections, comments,
// and formatting are left intact — full TOML re-encoding would discard
// those.
func WriteLocalTheme(path, theme string) error {
	theme = strings.TrimSpace(theme)
	if theme == "" {
		return fmt.Errorf("write local theme: empty value")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		stub := "theme = \"" + theme + "\"\n"
		return os.WriteFile(path, []byte(stub), 0o644)
	}
	if err != nil {
		return fmt.Errorf("read local config: %w", err)
	}
	out := setTopLevelTomlValue(string(data), "theme", theme)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write local config: %w", err)
	}
	return os.Rename(tmp, path)
}

// setTopLevelTomlValue replaces (or inserts) a top-level `key = "value"`
// assignment in src. "Top-level" means before the first `[section]` header.
// Existing lines are matched case-sensitively. Quoted string values only —
// numeric/boolean keys aren't needed for cast's surfaces yet.
func setTopLevelTomlValue(src, key, value string) string {
	trailingNL := strings.HasSuffix(src, "\n")
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	newLine := key + " = \"" + value + "\""

	// Find existing assignment before the first [section] header.
	matchPrefix := key + " ="
	matchPrefixSp := key + "="
	insertIdx := -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && !strings.HasPrefix(t, "[[") {
			break
		}
		if strings.HasPrefix(t, matchPrefix) || strings.HasPrefix(t, matchPrefixSp) {
			lines[i] = newLine
			out := strings.Join(lines, "\n")
			if trailingNL {
				out += "\n"
			}
			return out
		}
		// Track the last non-comment, non-blank line above any section so we
		// can append after it on insertion.
		if t != "" && !strings.HasPrefix(t, "#") {
			insertIdx = i
		}
	}

	// Not found → insert. Place after the last top-level assignment, or at
	// the very top if there are none.
	if insertIdx >= 0 {
		out := append([]string(nil), lines[:insertIdx+1]...)
		out = append(out, newLine)
		out = append(out, lines[insertIdx+1:]...)
		lines = out
	} else {
		lines = append([]string{newLine}, lines...)
	}
	res := strings.Join(lines, "\n")
	if trailingNL {
		res += "\n"
	}
	return res
}

// ── Templates ─────────────────────────────────────────────────────────────────

func globalTemplate() string {
	return `# cast — global configuration
# https://github.com/Cerebellum-ITM/cast

[theme]
default = "catppuccin"   # catppuccin | dracula | nord

# Per-environment theme base. The accent color is always overridden:
#   staging → orange  |  prod → red
[theme.env]
dev     = "catppuccin"
staging = "catppuccin"
prod    = "catppuccin"

[history]
max       = 100   # max run-history rows retained in the SQLite db
chain_max = 100   # max chain-execution rows retained in the SQLite db

[db]
path = ""   # empty = ~/.config/cast/cast.db

[source]
# When no Makefile (or configured source) is found in cwd, cast walks up to
# this many parent directories looking for one. Useful for monorepos / git
# submodules where the workdir sits below the Makefile. 0 disables walk-up.
lookup_depth = 5

[ui]
# Icon set used across the TUI (sidebar status dots, picker folder glyphs,
# splash, etc.).
#   "nerdfont" — Nerd Font private-use glyphs (default; recommended). Requires
#                a Nerd Font–patched terminal font.
#   "emoji"    — Generic Unicode emoji fallback for terminals without Nerd Font.
icons = "nerdfont"

[layout]
# Panel widths as % of total terminal width.
# With show_center_panel = true:  sidebar 15–40, output 30–60, sum ≤ 90.
# With show_center_panel = false: sidebar 30–50, output 30–50, sum ≤ 100.
# Runtime keys: [ / ]  shrink/grow output · { / }  shrink/grow sidebar.
sidebar_width_pct  = 25
output_width_pct   = 30
show_center_panel  = true

# ── WIP ──────────────────────────────────────────────────────────────────────
# Uncomment and fill these sections when the features are ready.

# [keybindings]
# build = "b"
# test  = "t"
# lint  = "l"

# [notifications]
# enabled    = false
# on_failure = true
# on_success = false

# [update]
# check   = true
# channel = "stable"   # stable | nightly
`
}

// LocalTemplateSrc returns the local config template as a string.
// LocalTemplateSrc returns the local config template as a string with the given env name.
func LocalTemplateSrc(envName string) string {
	return `# cast — local configuration
# Place this file in your project root and commit it.
# https://github.com/Cerebellum-ITM/cast

[env]
name = "` + envName + `"  # dev | staging | prod  — controls accent color in the TUI
file = ".env"             # path to .env file (relative to this config)

# [layout]
# sidebar_width_pct  = 20      # 15–40 with center on, 30–50 with center off
# output_width_pct   = 35      # 30–60 with center on, 30–50 with center off
# show_center_panel  = true    # false hides the middle detail panel

# [commands.shortcuts]
# # Command name → single-char keyboard shortcut. Wins over Makefile [sc=X]
# # tags and auto-inference. Manage via: cast shortcut set/unset/list.
# build = "b"
# test  = "t"

# ── WIP ──────────────────────────────────────────────────────────────────────
# Uncomment and fill these sections when the features are ready.

# [env]
# type = "dotenv"   # dotenv | direnv | chamber | ssm

# [source]
# type = "makefile"   # makefile | taskfile | yaml
# path = "./Makefile"

# [commands.shortcuts]
# "b" = "build"
# "t" = "test:unit"

# [project]
# name = ""
# team = ""
`
}
