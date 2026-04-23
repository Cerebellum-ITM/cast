package source

// Command is a runnable entry surfaced in the sidebar list.
type Command struct {
	Name     string
	Desc     string
	Category string
	Tags     []string
	Shortcut string // single-letter keyboard shortcut, e.g. "b"
}

// EnvVar is a single variable from a .env file.
type EnvVar struct {
	Key       string
	Value     string
	Sensitive bool   // mask unless showSecrets=true
	Comment   string // optional inline comment
}

// EnvFile represents a parsed .env file.
type EnvFile struct {
	Filename string
	Vars     []EnvVar
}

// CommandSource is implemented by any backend that can supply commands.
type CommandSource interface {
	Load() ([]Command, error)
}
