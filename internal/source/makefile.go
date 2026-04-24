package source

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
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
//
// Inline tags in the description drive per-command flags:
//
//	## tail-logs: follow the app log [stream]   → marks as streaming
//	## build: compile [no-stream]               → forces non-stream
//
// Targets whose recipe contains follow patterns (tail -f, docker logs -f,
// journalctl -f, kubectl … -f, watch …) are auto-detected as streams unless
// [no-stream] is set.
func (m *MakefileSource) Load() ([]Command, error) {
	f, err := os.Open(m.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var commands []Command
	skipTarget := ""

	// Buffer holds recipe lines of the *current* target so we can auto-detect
	// streaming after the whole target has been seen. idx points to the command
	// whose recipe is being collected (-1 = none).
	recipeFor := -1

	flushRecipe := func(body []string) {
		if recipeFor < 0 || recipeFor >= len(commands) {
			return
		}
		if commands[recipeFor].Stream { // already set via tag override
			return
		}
		if isStreamRecipe(body) {
			commands[recipeFor].Stream = true
		}
	}

	var currentBody []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Documented comment: ## name: description
		if strings.HasPrefix(line, "## ") {
			// New target starting → flush previous recipe buffer.
			flushRecipe(currentBody)
			currentBody = currentBody[:0]
			recipeFor = -1

			rest := strings.TrimPrefix(line, "## ")
			if idx := strings.Index(rest, ":"); idx != -1 {
				name := strings.TrimSpace(rest[:idx])
				desc := strings.TrimSpace(rest[idx+1:])
				if name == "" {
					continue
				}
				cleanDesc, stream, streamSet := extractStreamTag(desc)
				cleanDesc, shortcut, shortcutSet := extractShortcutTag(cleanDesc)
				cmd := Command{
					Name: name,
					Desc: cleanDesc,
					Tags: inferTags(name),
				}
				if shortcutSet {
					cmd.Shortcut = shortcut
				} else {
					cmd.Shortcut = autoShortcut(name, commands)
				}
				if streamSet {
					cmd.Stream = stream
				}
				commands = append(commands, cmd)
				recipeFor = len(commands) - 1
				skipTarget = name
			}
			continue
		}

		// Bare target line: "name:" not indented
		if strings.HasSuffix(strings.TrimSpace(line), ":") &&
			!strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {

			flushRecipe(currentBody)
			currentBody = currentBody[:0]
			recipeFor = -1

			raw := strings.TrimSuffix(strings.TrimSpace(line), ":")
			name := strings.Fields(raw)[0]
			if name == "" || strings.ContainsAny(name, ".$()") {
				skipTarget = ""
				continue
			}

			if name == skipTarget {
				skipTarget = ""
				recipeFor = len(commands) - 1 // continue collecting recipe for docstring'd target
				continue
			}
			skipTarget = ""

			commands = append(commands, Command{
				Name:     name,
				Tags:     inferTags(name),
				Shortcut: autoShortcut(name, commands),
			})
			recipeFor = len(commands) - 1
			continue
		}

		// Recipe line — tab-indented.
		if strings.HasPrefix(line, "\t") {
			if recipeFor >= 0 {
				currentBody = append(currentBody, line)
			}
			continue
		}

		// Any other non-recipe line ends the current recipe block.
		flushRecipe(currentBody)
		currentBody = currentBody[:0]
		recipeFor = -1
		skipTarget = ""
	}

	// Flush tail buffer (file may end inside a target recipe).
	flushRecipe(currentBody)

	return commands, scanner.Err()
}

// shortcutTagRe matches `[sc=X]` or `[shortcut=X]` (X = single char or empty
// for "no shortcut"). Trailing whitespace-tolerant.
var shortcutTagRe = regexp.MustCompile(`(?i)\s*\[(?:sc|shortcut)=([^\]]*)\]\s*$`)

// scTagAnywhereRe matches `[sc=X]` or `[shortcut=X]` with surrounding spaces,
// wherever it appears in a line. Used to strip a pre-existing tag before we
// append an updated one.
var scTagAnywhereRe = regexp.MustCompile(`(?i)\s*\[(?:sc|shortcut)=[^\]]*\]`)

// UpdateMakefileShortcut rewrites path so the `## cmdName: ...` docstring
// carries the given shortcut as a `[sc=X]` tag. Passing an empty shortcut
// removes the tag. If no docstring exists for cmdName, a minimal one is
// inserted above the target line.
//
// A trailing `[stream]` / `[no-stream]` tag on the docstring is preserved and
// kept at the end so the Makefile parser still sees it.
func UpdateMakefileShortcut(path, cmdName, shortcut string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	trailingNL := strings.HasSuffix(string(data), "\n")
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	docIdx, targetIdx := -1, -1
	for i, line := range lines {
		if docIdx == -1 && strings.HasPrefix(line, "## ") {
			rest := strings.TrimPrefix(line, "## ")
			if idx := strings.Index(rest, ":"); idx != -1 {
				if strings.TrimSpace(rest[:idx]) == cmdName {
					docIdx = i
				}
			}
		}
		if targetIdx == -1 && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			t := strings.TrimSpace(line)
			if t == cmdName+":" || strings.HasPrefix(t, cmdName+":") || strings.HasPrefix(t, cmdName+" ") {
				targetIdx = i
			}
		}
	}

	switch {
	case docIdx >= 0:
		lines[docIdx] = applyShortcutToDocLine(lines[docIdx], shortcut)
	case targetIdx >= 0:
		newDoc := "## " + cmdName + ":"
		if shortcut != "" {
			newDoc += " [sc=" + shortcut + "]"
		}
		lines = append(lines[:targetIdx], append([]string{newDoc}, lines[targetIdx:]...)...)
	default:
		return fmt.Errorf("target %q not found in %s", cmdName, path)
	}

	out := strings.Join(lines, "\n")
	if trailingNL {
		out += "\n"
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write makefile: %w", err)
	}
	return os.Rename(tmp, path)
}

// applyShortcutToDocLine returns the `## name: desc ...` line with its
// `[sc=X]` tag set to shortcut. Any pre-existing shortcut tag is removed.
// A trailing `[stream]` / `[no-stream]` tag is preserved at the end.
func applyShortcutToDocLine(line, shortcut string) string {
	trimmed := strings.TrimRight(line, " \t")

	streamSuffix := ""
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasSuffix(lower, "[stream]"):
		streamSuffix = trimmed[len(trimmed)-len("[stream]"):]
		trimmed = strings.TrimRight(trimmed[:len(trimmed)-len("[stream]")], " \t")
	case strings.HasSuffix(lower, "[no-stream]"):
		streamSuffix = trimmed[len(trimmed)-len("[no-stream]"):]
		trimmed = strings.TrimRight(trimmed[:len(trimmed)-len("[no-stream]")], " \t")
	}

	trimmed = scTagAnywhereRe.ReplaceAllString(trimmed, "")
	trimmed = strings.TrimRight(trimmed, " \t")

	if shortcut != "" {
		trimmed += " [sc=" + shortcut + "]"
	}
	if streamSuffix != "" {
		trimmed += " " + streamSuffix
	}
	return trimmed
}

// extractShortcutTag looks for `[sc=X]` or `[shortcut=X]` in desc. An empty
// value (e.g. `[sc=]`) means "disable auto-shortcut" and returns set=true,
// shortcut="". When the tag is absent, set=false and the caller falls back to
// auto-inference.
func extractShortcutTag(desc string) (clean string, shortcut string, set bool) {
	m := shortcutTagRe.FindStringSubmatchIndex(desc)
	if m == nil {
		return desc, "", false
	}
	// m[2]/m[3] = capture group 1 (the value between `=` and `]`).
	val := strings.TrimSpace(desc[m[2]:m[3]])
	cleaned := strings.TrimSpace(desc[:m[0]])
	// Accept a single letter/digit/symbol as shortcut; silently drop longer strings.
	if len(val) > 1 {
		val = val[:1]
	}
	return cleaned, val, true
}

// extractStreamTag looks for `[stream]` or `[no-stream]` at the end of desc and
// returns the cleaned description plus the parsed flag.
func extractStreamTag(desc string) (clean string, stream bool, set bool) {
	lower := strings.ToLower(desc)
	switch {
	case strings.HasSuffix(lower, "[stream]"):
		return strings.TrimSpace(desc[:len(desc)-len("[stream]")]), true, true
	case strings.HasSuffix(lower, "[no-stream]"):
		return strings.TrimSpace(desc[:len(desc)-len("[no-stream]")]), false, true
	}
	return desc, false, false
}

var streamRecipeRe = regexp.MustCompile(
	`(?i)(^|[\s;&|])(tail\s+(-[a-zA-Z]*f|--follow)` +
		`|docker(\s+compose)?\s+logs\s+(-[a-zA-Z]*f|--follow)` +
		`|kubectl\s+logs\b[^\n]*\s(-[a-zA-Z]*f|--follow)` +
		`|journalctl\b[^\n]*\s(-[a-zA-Z]*f|--follow)` +
		`|watch\s+` +
		`)`,
)

// isStreamRecipe reports whether any recipe line matches a known follow/stream pattern.
func isStreamRecipe(body []string) bool {
	for _, ln := range body {
		if streamRecipeRe.MatchString(ln) {
			return true
		}
	}
	return false
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
