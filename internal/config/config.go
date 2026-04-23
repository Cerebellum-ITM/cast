package config

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

// Theme holds the color palette name.
type Theme string

const (
	ThemeCatppuccin Theme = "catppuccin"
	ThemeDracula    Theme = "dracula"
	ThemeNord       Theme = "nord"
)

// Config is the parsed representation of ~/.config/cast/config.toml.
type Config struct {
	Env   Env
	Theme Theme

	SourceType string // "makefile" | "taskfile" | "yaml"
	SourcePath string

	HistoryMax  int
	HistoryPath string
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Env:         EnvLocal,
		Theme:       ThemeCatppuccin,
		SourceType:  "makefile",
		SourcePath:  "./Makefile",
		HistoryMax:  100,
		HistoryPath: "~/.config/cast/history.json",
	}
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
