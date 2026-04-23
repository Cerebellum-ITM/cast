package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ── Global config structs ────────────────────────────────────────────────────

// GlobalFile mirrors ~/.config/cast/cast.toml.
type GlobalFile struct {
	Theme   GlobalTheme   `toml:"theme"`
	History GlobalHistory `toml:"history"`

	// WIP: Keybindings   GlobalKeybindings   `toml:"keybindings"`
	// WIP: Notifications GlobalNotifications `toml:"notifications"`
	// WIP: Update        GlobalUpdate        `toml:"update"`
}

// GlobalTheme controls which theme is active per environment.
type GlobalTheme struct {
	Default string            `toml:"default"`
	Env     map[string]string `toml:"env"`
}

// GlobalHistory controls command-run history persistence.
type GlobalHistory struct {
	Max  int    `toml:"max"`
	Path string `toml:"path"`
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
	Env LocalEnv `toml:"env"`

	// WIP: Source   LocalSource            `toml:"source"`
	// WIP: Commands LocalCommands          `toml:"commands"`
	// WIP: Project  LocalProject           `toml:"project"`
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
max  = 100
path = ""   # empty = ~/.config/cast/history.json

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
