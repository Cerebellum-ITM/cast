package config

import (
	"fmt"
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
	SourcePath  string // absolute path, resolved via walk-up when needed
	SourceDir   string // dirname(SourcePath); where recipes must execute
	// SourceLookupDepth: parent directories to walk when resolving SourcePath.
	// 0 disables walk-up. Default 5.
	SourceLookupDepth int
	EnvFilePath       string // path to the .env file for this project

	HistoryMax      int
	ChainHistoryMax int
	DBPath          string

	ConfirmTargets []string // command names that always require confirmation

	// Shortcuts maps command-name → single-char keyboard shortcut. Wins over
	// Makefile `[sc=X]` tags and auto-inference. Populated from .cast.toml's
	// [commands.shortcuts] section.
	Shortcuts map[string]string

	// OutputWidthPct is the percentage of total terminal width dedicated to
	// the live-output panel. With center panel visible: 30–60. With center
	// hidden: 30–50. Default: 30.
	OutputWidthPct int

	// SidebarWidthPct is the percentage of total terminal width dedicated to
	// the left sidebar. With center visible: 15–40. With center hidden: 30–50.
	// Default: 25.
	SidebarWidthPct int

	// ShowCenterPanel controls whether the middle detail/env panel is
	// rendered. When false, sidebar + output share the full width.
	ShowCenterPanel bool
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
		HistoryMax:       100,
		ChainHistoryMax:  100,
		SourceLookupDepth: 5,
		DBPath:          filepath.Join(home, ".config", "cast", "cast.db"),
		OutputWidthPct:  30,
		SidebarWidthPct: 25,
		ShowCenterPanel: true,
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
	if global.History.ChainMax > 0 {
		cfg.ChainHistoryMax = global.History.ChainMax
	}
	if global.Source.LookupDepth != 0 {
		cfg.SourceLookupDepth = global.Source.LookupDepth
	}
	if global.DB.Path != "" {
		cfg.DBPath = global.DB.Path
	}
	if v := global.Layout.OutputWidthPct; v > 0 {
		cfg.OutputWidthPct = v
	}
	if v := global.Layout.SidebarWidthPct; v > 0 {
		cfg.SidebarWidthPct = v
	}
	if global.Layout.ShowCenterPanel != nil {
		cfg.ShowCenterPanel = *global.Layout.ShowCenterPanel
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
		cfg.Shortcuts = local.Commands.Shortcuts
		if v := local.Layout.OutputWidthPct; v > 0 {
			cfg.OutputWidthPct = v
		}
		if v := local.Layout.SidebarWidthPct; v > 0 {
			cfg.SidebarWidthPct = v
		}
		if local.Layout.ShowCenterPanel != nil {
			cfg.ShowCenterPanel = *local.Layout.ShowCenterPanel
		}
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

	if err := validateLayout(cfg); err != nil {
		return nil, err
	}

	cfg.SourcePath = resolveSourcePath(cfg.SourcePath, cfg.SourceLookupDepth)
	cfg.SourceDir = filepath.Dir(cfg.SourcePath)

	return cfg, nil
}

// resolveSourcePath returns an absolute path to the task-source file, walking
// up to `depth` parent directories from cwd when the relative/default path
// doesn't exist in cwd. If nothing is found, returns the input unchanged so
// downstream parsers can surface their own not-found error.
//
// The relative structure of `p` is preserved at each ancestor: a value like
// "build/Makefile" is tried as "<ancestor>/build/Makefile" at each level.
func resolveSourcePath(p string, depth int) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return p
	}
	rel := p
	if len(rel) > 2 && rel[0] == '.' && (rel[1] == '/' || rel[1] == '\\') {
		rel = rel[2:]
	}

	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	// Level 0 (cwd) + depth parents.
	ancestor := cwd
	for i := 0; i <= depth; i++ {
		candidate := filepath.Join(ancestor, rel)
		if _, err := os.Stat(candidate); err == nil {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return candidate
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			break // reached filesystem root
		}
		ancestor = parent
	}
	return p
}

// validateLayout enforces panel-width invariants. When the center panel is
// visible, sidebar+output must leave at least 10% for it; when hidden, both
// panels are capped at 50% so they can split the screen without overlap.
func validateLayout(cfg *Config) error {
	sb := cfg.SidebarWidthPct
	out := cfg.OutputWidthPct

	if cfg.ShowCenterPanel {
		if sb < 15 || sb > 40 {
			return fmt.Errorf("layout.sidebar_width_pct = %d: must be between 15 and 40 when the center panel is visible", sb)
		}
		if out < 30 || out > 60 {
			return fmt.Errorf("layout.output_width_pct = %d: must be between 30 and 60 when the center panel is visible", out)
		}
		if sb+out > 90 {
			return fmt.Errorf("layout: sidebar (%d%%) + output (%d%%) = %d%% leaves no room for the center panel (need ≥ 10%%); lower one of them or disable show_center_panel", sb, out, sb+out)
		}
		return nil
	}

	// Center hidden: sidebar + output share the full width.
	if sb < 30 || sb > 50 {
		return fmt.Errorf("layout.sidebar_width_pct = %d: must be between 30 and 50 when the center panel is hidden", sb)
	}
	if out < 30 || out > 50 {
		return fmt.Errorf("layout.output_width_pct = %d: must be between 30 and 50 when the center panel is hidden", out)
	}
	if sb+out > 100 {
		return fmt.Errorf("layout: sidebar (%d%%) + output (%d%%) = %d%% exceeds 100%%", sb, out, sb+out)
	}
	return nil
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
