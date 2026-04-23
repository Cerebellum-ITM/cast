package source

import (
	"bufio"
	"os"
	"strings"
)

// MakefileSource loads commands from a Makefile using ## comments.
type MakefileSource struct {
	Path string
}

// Load parses the Makefile and returns one Command per documented target.
//
// Format:
//
//	## build: Compiles the application binary.
//	build:
//	    @go build ...
//
// The `## name:` comment wins — the bare `name:` target line that follows is
// skipped to avoid duplicates.
func (m *MakefileSource) Load() ([]Command, error) {
	f, err := os.Open(m.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var commands []Command
	// skipTarget holds the name of the next target line to skip (because it
	// was already added via a ## comment).
	skipTarget := ""

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Documented comment: ## name: description
		if strings.HasPrefix(line, "## ") {
			rest := strings.TrimPrefix(line, "## ")
			if idx := strings.Index(rest, ":"); idx != -1 {
				name := strings.TrimSpace(rest[:idx])
				desc := strings.TrimSpace(rest[idx+1:])
				if name == "" {
					continue
				}
				commands = append(commands, Command{
					Name:     name,
					Desc:     desc,
					Tags:     inferTags(name),
					Shortcut: autoShortcut(name, commands),
				})
				skipTarget = name // next bare target line is this same target
			}
			continue
		}

		// Bare target line: "name:" not indented
		if strings.HasSuffix(strings.TrimSpace(line), ":") &&
			!strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {

			raw := strings.TrimSuffix(strings.TrimSpace(line), ":")
			name := strings.Fields(raw)[0] // handle "name: dep1 dep2" style
			if name == "" || strings.ContainsAny(name, ".$()") {
				skipTarget = ""
				continue
			}

			// Skip if already registered via ## comment
			if name == skipTarget {
				skipTarget = ""
				continue
			}
			skipTarget = ""

			commands = append(commands, Command{
				Name:     name,
				Tags:     inferTags(name),
				Shortcut: autoShortcut(name, commands),
			})
			continue
		}

		// Any other line resets the skip guard only if it's not a recipe line.
		if !strings.HasPrefix(line, "\t") {
			skipTarget = ""
		}
	}

	return commands, scanner.Err()
}

// inferTags returns auto-detected category tags based on the command name.
func inferTags(name string) []string {
	n := strings.ToLower(name)
	type rule struct {
		substr string
		tag    string
	}
	rules := []rule{
		{"deploy", "prod"},
		{"prod", "prod"},
		{"release", "go"},
		{"build", "go"},
		{"golangci", "golangci"},
		{"lint", "golangci"},
		{"test", "ci"},
		{"bench", "ci"},
		{"local", "local"},
		{"run", "local"},
	}
	for _, r := range rules {
		if strings.Contains(n, r.substr) {
			return []string{r.tag}
		}
	}
	return nil
}

// autoShortcut assigns the first unused letter of name as a shortcut.
func autoShortcut(name string, existing []Command) string {
	used := make(map[string]bool, len(existing))
	for _, c := range existing {
		used[c.Shortcut] = true
	}
	for _, ch := range name {
		s := string(ch)
		if !used[s] {
			return s
		}
	}
	return ""
}
