// Package library manages a per-user collection of reusable Makefile target
// snippets stored under ~/.config/cast/snippets/. Each snippet is a single
// .mk file containing a complete target: a `## name: desc [tags…]` doc-line
// followed by the target line and its tab-indented recipe body. cast can
// list, save, load, insert into a project Makefile, and delete snippets.
package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dirOverride lets tests redirect Dir() to a temp directory without touching
// the user's real ~/.config/cast/snippets/. Production callers leave this
// empty.
var dirOverride string

// SetDirForTest overrides the snippets directory. Tests use t.TempDir() so
// they don't pollute the real config dir. Pass "" to restore default.
func SetDirForTest(d string) { dirOverride = d }

// Dir returns the absolute path to the snippets directory. The directory is
// not created here — callers that write should EnsureDir first.
func Dir() string {
	if dirOverride != "" {
		return dirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cast", "snippets")
}

// EnsureDir creates the snippets directory if it doesn't exist. Idempotent.
func EnsureDir() error {
	return os.MkdirAll(Dir(), 0o755)
}

// Snippet is a single reusable Makefile target. Body is the verbatim file
// content (doc-line + target + recipe), ready to append into another
// Makefile. Name is the canonical identifier (matches the file's basename
// without the .mk extension).
type Snippet struct {
	Name string
	Desc string
	Tags []string
	Path string // absolute path to the .mk file
	Body string // full file content
}

// ErrInvalidName is returned when a snippet name sanitises to empty.
var ErrInvalidName = errors.New("library: invalid snippet name")

// ErrNotFound is returned when Load/Delete reference a missing snippet.
var ErrNotFound = errors.New("library: snippet not found")

// SanitizeName reduces name to [A-Za-z0-9_-]. The Makefile target grammar
// accepts more characters, but restricting to a safe filesystem-friendly
// subset avoids surprises (no slashes, no dots, no spaces in basenames).
// Returns "" if everything sanitised away — callers should treat that as
// ErrInvalidName.
func SanitizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// List returns every snippet currently stored under Dir(). Files that fail
// to parse are silently skipped — a malformed file shouldn't break the
// whole listing. The result is sorted alphabetically by Name.
func List() ([]Snippet, error) {
	if err := EnsureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return nil, fmt.Errorf("library: read dir: %w", err)
	}
	var out []Snippet
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".mk") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".mk")
		s, err := Load(name)
		if err != nil {
			continue
		}
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Load reads the snippet whose basename matches name. ErrNotFound when no
// such file exists. Body carries the verbatim file content.
func Load(name string) (*Snippet, error) {
	clean := SanitizeName(name)
	if clean == "" {
		return nil, ErrInvalidName
	}
	path := filepath.Join(Dir(), clean+".mk")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("library: read %s: %w", clean, err)
	}
	body := string(data)
	desc, tags := parseDocSummary(body)
	return &Snippet{
		Name: clean,
		Desc: desc,
		Tags: tags,
		Path: path,
		Body: body,
	}, nil
}

// Save writes s to <Dir>/<sanitized-name>.mk atomically (tmp + Rename). It
// rewrites the first `## ...:` doc-line so the target name embedded inside
// the body matches the canonical file basename, preventing drift between
// filename and what cast displays. ErrInvalidName when the sanitised name
// is empty.
func Save(s Snippet) error {
	clean := SanitizeName(s.Name)
	if clean == "" {
		return ErrInvalidName
	}
	if err := EnsureDir(); err != nil {
		return err
	}
	body := normalizeSnippetBody(s.Body, clean)
	path := filepath.Join(Dir(), clean+".mk")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return fmt.Errorf("library: write %s: %w", clean, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("library: rename %s: %w", clean, err)
	}
	return nil
}

// Delete removes the snippet file. ErrNotFound when missing.
func Delete(name string) error {
	clean := SanitizeName(name)
	if clean == "" {
		return ErrInvalidName
	}
	path := filepath.Join(Dir(), clean+".mk")
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("library: delete %s: %w", clean, err)
	}
	return nil
}

// parseDocSummary extracts the description and tag list from the first
// `## name: desc [tags…]` line in body. Both are best-effort — empty
// strings/nil slice when not present. Used to populate Snippet metadata
// without depending on the source package's tag parser (the body itself is
// authoritative; this is just for UI listings).
func parseDocSummary(body string) (desc string, tags []string) {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		rest := strings.TrimPrefix(line, "## ")
		colon := strings.Index(rest, ":")
		if colon < 0 {
			return "", nil
		}
		descPart := strings.TrimSpace(rest[colon+1:])
		// Strip trailing [tag] groups for the visible description and
		// collect them as the tag list.
		for {
			lb := strings.LastIndex(descPart, "[")
			rb := strings.LastIndex(descPart, "]")
			if lb < 0 || rb < lb || rb != len(descPart)-1 {
				break
			}
			tagSrc := descPart[lb+1 : rb]
			descPart = strings.TrimSpace(descPart[:lb])
			lower := strings.ToLower(tagSrc)
			if strings.HasPrefix(lower, "tags=") {
				for _, t := range strings.Split(tagSrc[len("tags="):], ",") {
					if t = strings.TrimSpace(t); t != "" {
						tags = append(tags, t)
					}
				}
			}
		}
		return descPart, tags
	}
	return "", nil
}

// normalizeSnippetBody rewrites the first `## oldName: …` doc-line so the
// embedded target name matches the canonical basename, and rewrites the
// first bare target line `oldName:` accordingly. If no doc-line is present,
// a minimal one is prepended. Trailing newline is guaranteed.
func normalizeSnippetBody(body, name string) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")

	docIdx, targetIdx := -1, -1
	var oldName string
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if docIdx < 0 && strings.HasPrefix(line, "## ") {
			rest := strings.TrimPrefix(line, "## ")
			if c := strings.Index(rest, ":"); c >= 0 {
				docIdx = i
				oldName = strings.TrimSpace(rest[:c])
			}
			continue
		}
		if targetIdx < 0 &&
			!strings.HasPrefix(line, "\t") &&
			!strings.HasPrefix(line, " ") &&
			strings.HasSuffix(t, ":") &&
			!strings.HasPrefix(t, "#") {
			targetIdx = i
			break
		}
	}

	// Rewrite doc-line: replace `## oldName: …` with `## name: …` preserving
	// the description+tags suffix.
	if docIdx >= 0 && oldName != "" && oldName != name {
		rest := strings.TrimPrefix(lines[docIdx], "## ")
		c := strings.Index(rest, ":")
		suffix := rest[c:]
		lines[docIdx] = "## " + name + suffix
	} else if docIdx < 0 {
		// No doc-line: prepend a minimal one so the snippet keeps cast's
		// `## name: desc` contract.
		lines = append([]string{"## " + name + ":"}, lines...)
		if targetIdx >= 0 {
			targetIdx++
		}
	}

	// Rewrite bare target line.
	if targetIdx >= 0 {
		t := strings.TrimSpace(lines[targetIdx])
		// Replace only the leading "<oldName>" up to the first `:` or space.
		end := strings.IndexAny(t, ": ")
		if end > 0 {
			lines[targetIdx] = name + t[end:]
		}
	}

	return strings.Join(lines, "\n") + "\n"
}
