package config

import (
	"os"
	"path/filepath"
)

// Env represents the active deployment environment.
type Env int

const (
	EnvLocal Env = iota
	EnvStaging
	EnvProd
)

func (e Env) String() string {
	switch e {
	case EnvStaging:
		return "staging"
	case EnvProd:
		return "prod"
	default:
		return "local"
	}
}

// envKey returns the map key used in [theme.env] for this environment.
func (e Env) envKey() string {
	switch e {
	case EnvStaging:
		return "staging"
	case EnvProd:
		return "prod"
	default:
		return "dev"
	}
}

// Theme holds the color palette name.
type Theme string

const (
	ThemeCatppuccin Theme = "catppuccin"
	ThemeDracula    Theme = "dracula"
	ThemeNord       Theme = "nord"
)

// Config is the resolved runtime configuration.
// Priority: CLI flags > CAST_ENV > local .cast.toml > global cast.toml > defaults.
type Config struct {
	Env   Env
	Theme Theme

	SourceType  string // "makefile" | "taskfile" | "yaml"
	SourcePath  string
	EnvFilePath string // path to the .env file for this project

	HistoryMax int
	DBPath     string

	ConfirmTargets []string // command names that always require confirmation
}

// Default returns a Config with sensible hardcoded defaults.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Env:         EnvLocal,
		Theme:       ThemeCatppuccin,
		SourceType:  "makefile",
		SourcePath:  "./Makefile",
		EnvFilePath: ".env",
		HistoryMax:  100,
		DBPath:      filepath.Join(home, ".config", "cast", "cast.db"),
	}
}

// Load builds the resolved Config by layering:
//  1. Hardcoded defaults
//  2. Global config (~/.config/cast/cast.toml) — created if missing
//  3. Local config (.cast.toml in cwd) — optional
//  4. CAST_ENV environment variable
//  5. flagEnv / flagTheme CLI overrides (empty = not set)
func Load(flagEnv, flagTheme string) (*Config, error) {
	cfg := Default()

	// ── Layer 2: global file ──────────────────────────────────────────────
	global, err := LoadGlobal()
	if err != nil {
		return nil, err
	}

	if t := Theme(global.Theme.Default); t != "" {
		cfg.Theme = t
	}
	if global.History.Max > 0 {
		cfg.HistoryMax = global.History.Max
	}
	if global.DB.Path != "" {
		cfg.DBPath = global.DB.Path
	}

	// ── Layer 3: local file ───────────────────────────────────────────────
	local, ok := LoadLocal()
	if ok {
		if local.Env.Name != "" {
			cfg.Env = ParseEnv(local.Env.Name)
		}
		if local.Env.File != "" {
			cfg.EnvFilePath = local.Env.File
		}
		cfg.ConfirmTargets = local.Commands.Confirm.Targets
	}

	// ── Layer 4: CAST_ENV env var ─────────────────────────────────────────
	if e := os.Getenv("CAST_ENV"); e != "" {
		cfg.Env = ParseEnv(e)
	}

	// ── Layer 5: CLI flags ────────────────────────────────────────────────
	if flagEnv != "" {
		cfg.Env = ParseEnv(flagEnv)
	}
	if flagTheme != "" {
		cfg.Theme = Theme(flagTheme)
	}

	// Resolve per-environment theme from global (only when theme wasn't set
	// by a CLI flag, so the flag always wins).
	if flagTheme == "" {
		if key := cfg.Env.envKey(); key != "" {
			if t, found := global.Theme.Env[key]; found && t != "" {
				cfg.Theme = Theme(t)
			}
		}
	}

	return cfg, nil
}

// ParseEnv converts a string to an Env, defaulting to EnvLocal.
func ParseEnv(s string) Env {
	switch s {
	case "staging":
		return EnvStaging
	case "prod", "production":
		return EnvProd
	default:
		return EnvLocal
	}
}
