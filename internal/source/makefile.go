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
	// streamPinned[i] is true when the Makefile explicitly declared a stream
	// tag ([stream] or [no-stream]) for command i — auto-detect must not
	// override the user's choice in that case.
	var streamPinned []bool
	skipTarget := ""

	// Buffer holds recipe lines of the *current* target so we can auto-detect
	// streaming after the whole target has been seen. idx points to the command
	// whose recipe is being collected (-1 = none).
	recipeFor := -1

	flushRecipe := func(body []string) {
		if recipeFor < 0 || recipeFor >= len(commands) {
			return
		}
		if streamPinned[recipeFor] {
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
				meta := extractDocTags(desc)
				cmd := Command{
					Name:      name,
					Desc:      meta.Desc,
					Tags:      inferTags(name),
					Confirm:   meta.state.Confirm,
					NoConfirm: meta.state.NoConfirm,
				}
				if meta.state.HasShortcut {
					cmd.Shortcut = meta.state.Shortcut
				} else {
					cmd.Shortcut = autoShortcut(name, commands)
				}
				if meta.state.StreamSet {
					cmd.Stream = meta.state.Stream
				}
				commands = append(commands, cmd)
				streamPinned = append(streamPinned, meta.state.StreamSet)
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
			streamPinned = append(streamPinned, false)
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

// DocTagState captures every flag-style tag we recognize on a `## name: desc`
// line. Exported so callers can inspect and mutate tags uniformly.
type DocTagState struct {
	Shortcut    string // single-char; "" means none
	HasShortcut bool   // true iff [sc=X] was present in source
	StreamSet   bool   // true iff [stream] or [no-stream] present
	Stream      bool   // value when StreamSet
	Confirm     bool   // [confirm] present
	NoConfirm   bool   // [no-confirm] present
}

// docMeta is the parse result for the description portion of a doc line.
type docMeta struct {
	Desc  string
	state DocTagState
}

// trailingTagRe matches a single `[...]` tag with optional surrounding space
// at the very end of a string. Content capture is whatever is inside the
// brackets (minus outer whitespace).
var trailingTagRe = regexp.MustCompile(`\s*\[([^\]]*)\]\s*$`)

// extractDocTags peels trailing `[tag]` tokens off desc one by one until the
// tail is no longer a recognized tag. Unknown tags abort the walk (left in
// desc) so hand-written tags we don't know about survive round-tripping.
func extractDocTags(desc string) docMeta {
	m := docMeta{Desc: desc}
	for {
		match := trailingTagRe.FindStringSubmatchIndex(m.Desc)
		if match == nil {
			return m
		}
		inner := strings.TrimSpace(m.Desc[match[2]:match[3]])
		remaining := strings.TrimRight(m.Desc[:match[0]], " \t")
		lower := strings.ToLower(inner)

		switch {
		case lower == "stream":
			if m.state.StreamSet {
				return m
			}
			m.state.Stream, m.state.StreamSet = true, true
		case lower == "no-stream":
			if m.state.StreamSet {
				return m
			}
			m.state.Stream, m.state.StreamSet = false, true
		case lower == "confirm":
			if m.state.Confirm || m.state.NoConfirm {
				return m
			}
			m.state.Confirm = true
		case lower == "no-confirm":
			if m.state.Confirm || m.state.NoConfirm {
				return m
			}
			m.state.NoConfirm = true
		case strings.HasPrefix(lower, "sc=") || strings.HasPrefix(lower, "shortcut="):
			if m.state.HasShortcut {
				return m
			}
			val := inner[strings.Index(inner, "=")+1:]
			val = strings.TrimSpace(val)
			if len(val) > 1 {
				val = val[:1]
			}
			m.state.Shortcut, m.state.HasShortcut = val, true
		default:
			return m
		}
		m.Desc = remaining
	}
}

// renderDocTags formats state in a canonical order:
//
//	[sc=X] [confirm]/[no-confirm] [stream]/[no-stream]
//
// Absent tags are omitted entirely — never emit `[sc=]`.
func renderDocTags(state DocTagState) string {
	var tags []string
	if state.HasShortcut && state.Shortcut != "" {
		tags = append(tags, "[sc="+state.Shortcut+"]")
	}
	switch {
	case state.Confirm:
		tags = append(tags, "[confirm]")
	case state.NoConfirm:
		tags = append(tags, "[no-confirm]")
	}
	if state.StreamSet {
		if state.Stream {
			tags = append(tags, "[stream]")
		} else {
			tags = append(tags, "[no-stream]")
		}
	}
	return strings.Join(tags, " ")
}

// parseDocLine splits a `## name: desc [tags...]` line into its stable prefix
// (`## name: desc` with no tags) and its tag state. Returns ok=false if line
// is not a recognizable doc line for the given name.
func parseDocLine(line, cmdName string) (prefix string, state DocTagState, ok bool) {
	if !strings.HasPrefix(line, "## ") {
		return "", DocTagState{}, false
	}
	rest := strings.TrimPrefix(line, "## ")
	colon := strings.Index(rest, ":")
	if colon == -1 {
		return "", DocTagState{}, false
	}
	if strings.TrimSpace(rest[:colon]) != cmdName {
		return "", DocTagState{}, false
	}
	desc := strings.TrimSpace(rest[colon+1:])
	meta := extractDocTags(desc)
	header := "## " + cmdName + ":"
	if meta.Desc != "" {
		header += " " + meta.Desc
	}
	return header, meta.state, true
}

// renderDocLine composes a doc line from the trimmed prefix and tag state.
func renderDocLine(prefix string, state DocTagState) string {
	tags := renderDocTags(state)
	if tags == "" {
		return prefix
	}
	return prefix + " " + tags
}

// mutateMakefileDocLine loads path, locates the `## cmdName: ...` line (or
// inserts one above the target if missing), applies mutate to its tag state,
// and writes the file back atomically.
func mutateMakefileDocLine(path, cmdName string, mutate func(*DocTagState)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	trailingNL := strings.HasSuffix(string(data), "\n")
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	docIdx, targetIdx := -1, -1
	for i, line := range lines {
		if docIdx == -1 {
			if _, _, ok := parseDocLine(line, cmdName); ok {
				docIdx = i
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
		prefix, state, _ := parseDocLine(lines[docIdx], cmdName)
		mutate(&state)
		lines[docIdx] = renderDocLine(prefix, state)
	case targetIdx >= 0:
		var state DocTagState
		mutate(&state)
		lines = append(lines[:targetIdx], append([]string{renderDocLine("## "+cmdName+":", state)}, lines[targetIdx:]...)...)
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

// ReadDocTagState parses path and returns the tag state declared on the
// `## cmdName: ...` line. Missing docstring returns a zero DocTagState and
// ok=false. Useful for UIs that want to reflect the current source-of-truth
// without rebuilding the full Command slice.
func ReadDocTagState(path, cmdName string) (DocTagState, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DocTagState{}, false, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, line := range lines {
		if _, state, ok := parseDocLine(line, cmdName); ok {
			return state, true, nil
		}
	}
	return DocTagState{}, false, nil
}

// UpdateMakefileShortcut rewrites path so the `## cmdName: ...` docstring
// carries shortcut as a `[sc=X]` tag. Passing "" removes the tag.
func UpdateMakefileShortcut(path, cmdName, shortcut string) error {
	return mutateMakefileDocLine(path, cmdName, func(s *DocTagState) {
		if shortcut == "" {
			s.HasShortcut = false
			s.Shortcut = ""
			return
		}
		s.HasShortcut = true
		s.Shortcut = shortcut
	})
}

// UpdateMakefileFlag toggles a presence-only flag (`stream`, `no-stream`,
// `confirm`, `no-confirm`) on the doc line of cmdName. on=false removes the
// flag; on=true adds it and clears the mutually-exclusive partner so the pair
// never both exist.
func UpdateMakefileFlag(path, cmdName, flag string, on bool) error {
	return mutateMakefileDocLine(path, cmdName, func(s *DocTagState) {
		switch flag {
		case "stream":
			if on {
				s.StreamSet, s.Stream = true, true
			} else if s.StreamSet && s.Stream {
				s.StreamSet, s.Stream = false, false
			}
		case "no-stream":
			if on {
				s.StreamSet, s.Stream = true, false
			} else if s.StreamSet && !s.Stream {
				s.StreamSet = false
			}
		case "confirm":
			if on {
				s.Confirm, s.NoConfirm = true, false
			} else {
				s.Confirm = false
			}
		case "no-confirm":
			if on {
				s.NoConfirm, s.Confirm = true, false
			} else {
				s.NoConfirm = false
			}
		}
	})
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
