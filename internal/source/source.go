package source

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Command is a runnable entry surfaced in the sidebar list.
type Command struct {
	Name      string
	Desc      string
	Category  string
	Tags      []string
	Shortcut  string // single-letter keyboard shortcut, e.g. "b"
	Confirm   bool   // show confirmation modal regardless of active env
	NoConfirm bool   // suppress confirmation even in staging/prod (e.g. logs -f)
	Stream    bool   // long-running log-streaming command (docker logs -f, tail -f…)
	Interactive bool // needs a real TTY (python3, bash, psql…); suspends the TUI
	Picks     []PickStep // sequential folder pickers required before run; nil = none
}

// PickStep describes one folder-picker step. Pickers run sequentially before
// dispatching the target. Each step lists the contents of BaseDirTemplate
// (with `{pickN}` placeholders substituted from previous selections), filters
// by Filter (substring match, case-insensitive; supports `*` glob), and stores
// the chosen folder name into Alias (or `CAST_PICK_<index>` if Alias is empty).
type PickStep struct {
	BaseDirTemplate string // e.g. ".", "services", "{pick1}"
	Filter          string // optional substring/glob, e.g. "*addons*"
	Alias           string // env/make var name; "" => CAST_PICK_<n>
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
	RawLines []string // verbatim lines for syntax-highlighted preview
}

// CommandSource is implemented by any backend that can supply commands.
type CommandSource interface {
	Load() ([]Command, error)
}

// sensitiveKeywords are checked (case-insensitive) against env var key names.
var sensitiveKeywords = []string{
	"PASSWORD", "PASSWD", "SECRET", "TOKEN", "KEY", "PRIVATE",
	"AUTH", "CREDENTIAL", "PASS", "API_KEY", "APIKEY",
}

// IsSensitiveKey reports whether a key name likely contains a sensitive value.
func IsSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

// ParseEnvFile reads path and returns a populated EnvFile.
// Lines starting with # are treated as comments and attached to the next
// KEY=VALUE pair. Blank lines reset the pending comment.
func ParseEnvFile(path string) (*EnvFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ef := &EnvFile{Filename: path}
	scanner := bufio.NewScanner(f)

	var pendingComment string
	for scanner.Scan() {
		line := scanner.Text()
		ef.RawLines = append(ef.RawLines, line)

		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			pendingComment = ""
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if pendingComment != "" {
				pendingComment += " " + comment
			} else {
				pendingComment = comment
			}
			continue
		}

		idx := strings.Index(trimmed, "=")
		if idx < 1 {
			pendingComment = ""
			continue
		}

		key := strings.TrimSpace(trimmed[:idx])
		raw := trimmed[idx+1:]
		value := stripQuotes(raw)

		ev := EnvVar{
			Key:       key,
			Value:     value,
			Sensitive: IsSensitiveKey(key),
			Comment:   pendingComment,
		}
		ef.Vars = append(ef.Vars, ev)
		pendingComment = ""
	}

	return ef, scanner.Err()
}

// WriteEnvFile reconstructs the .env file from ef.Vars and writes it to path.
// Comments are written as # lines before their variable. Formatting is normalized.
func WriteEnvFile(ef *EnvFile, path string) error {
	var sb strings.Builder
	for i, v := range ef.Vars {
		if i > 0 {
			sb.WriteString("\n")
		}
		if v.Comment != "" {
			sb.WriteString("# ")
			sb.WriteString(v.Comment)
			sb.WriteString("\n")
		}
		sb.WriteString(v.Key)
		sb.WriteString("=")
		sb.WriteString(quoteIfNeeded(v.Value))
		sb.WriteString("\n")
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}
	return os.Rename(tmp, path)
}

// stripQuotes removes surrounding single or double quotes from a value.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// quoteIfNeeded wraps the value in double quotes if it contains spaces,
// tabs, or special shell characters.
func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t#$&|;<>(){}") {
		escaped := strings.ReplaceAll(s, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}
