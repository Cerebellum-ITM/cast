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
// A documented target looks like:
//
//	## build: Compiles the application binary.
//	build:
func (m *MakefileSource) Load() ([]Command, error) {
	f, err := os.Open(m.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var commands []Command
	var pendingDesc string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "## ") {
			rest := strings.TrimPrefix(line, "## ")
			if idx := strings.Index(rest, ":"); idx != -1 {
				name := strings.TrimSpace(rest[:idx])
				desc := strings.TrimSpace(rest[idx+1:])
				commands = append(commands, Command{
					Name:     name,
					Desc:     desc,
					Shortcut: autoShortcut(name, commands),
				})
				pendingDesc = ""
				continue
			}
			pendingDesc = strings.TrimPrefix(line, "## ")
			continue
		}

		// Plain target line: "name:"
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			name := strings.TrimSuffix(line, ":")
			name = strings.Split(name, " ")[0]
			if name == "" || strings.ContainsAny(name, ".$()") {
				pendingDesc = ""
				continue
			}
			commands = append(commands, Command{
				Name:     name,
				Desc:     pendingDesc,
				Shortcut: autoShortcut(name, commands),
			})
			pendingDesc = ""
		} else {
			pendingDesc = ""
		}
	}

	return commands, scanner.Err()
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
