package ai

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/source"
)

// DiffColors carries the three colours RenderDiff uses. A nil *DiffColors (or
// nil fields) yields a plain, uncoloured diff — used for --json, non-TTY CLI
// output, and tests. The TUI passes its active palette colours so the popup
// matches the rest of the theme. Keeping this as image/color (stdlib) lets the
// ai package stay free of any views/tui dependency.
type DiffColors struct {
	Add     color.Color // inserted (+) lines
	Del     color.Color // removed (-) lines
	Context color.Color // unchanged context lines and hunk headers
}

const diffContextLines = 3

// RenderDiff produces a unified-diff-style preview of what ApplyPlan would
// write, one hunk per annotation. `src` is the current Makefile content.
func RenderDiff(plan Plan, src []byte, c *DiffColors) string {
	lines := splitLines(src)
	var b strings.Builder
	for i, a := range plan.Annotations {
		section := source.MakefileTargetLines(lines, a.Name)
		if section == nil {
			continue
		}
		newSection, err := source.WriteDocLine(section, a.Name, a.Desc, a.Tags)
		if err != nil {
			continue
		}
		oldDoc := docLineOf(section, a.Name)
		newDoc := docLineOf(newSection, a.Name)

		if i > 0 {
			b.WriteString("\n")
		}
		writeLine(&b, colorize(c, ctxKind, "@@ "+a.Name+" @@"))
		if oldDoc != "" {
			writeLine(&b, colorize(c, delKind, "-"+oldDoc))
		}
		writeLine(&b, colorize(c, addKind, "+"+newDoc))
		for _, ln := range contextTail(section, diffContextLines) {
			writeLine(&b, colorize(c, ctxKind, " "+ln))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// ApplyPlan reads the Makefile at path, applies every annotation via
// source.WriteDocLine, and writes the result back atomically through a
// `.<file>.tmp` rename. No backup is kept — callers rely on git.
func ApplyPlan(plan Plan, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("ai: read %s: %w", path, err)
	}
	trailingNL := strings.HasSuffix(string(data), "\n")
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	for _, a := range plan.Annotations {
		next, err := source.WriteDocLine(lines, a.Name, a.Desc, a.Tags)
		if err != nil {
			return fmt.Errorf("ai: annotate %s: %w", a.Name, err)
		}
		lines = next
	}

	out := strings.Join(lines, "\n")
	if trailingNL {
		out += "\n"
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return fmt.Errorf("ai: write %s: %w", tmp, err)
	}
	return os.Rename(tmp, path)
}

// ── helpers ──────────────────────────────────────────────────────────────────

type lineKind int

const (
	ctxKind lineKind = iota
	addKind
	delKind
)

func colorize(c *DiffColors, kind lineKind, s string) string {
	if c == nil {
		return s
	}
	var col color.Color
	switch kind {
	case addKind:
		col = c.Add
	case delKind:
		col = c.Del
	default:
		col = c.Context
	}
	if col == nil {
		return s
	}
	return lipgloss.NewStyle().Foreground(col).Render(s)
}

func writeLine(b *strings.Builder, s string) {
	b.WriteString(s)
	b.WriteString("\n")
}

func splitLines(src []byte) []string {
	return strings.Split(strings.TrimRight(string(src), "\n"), "\n")
}

// docLineOf returns the `## name: …` doc-line within section, or "".
func docLineOf(section []string, name string) string {
	for _, ln := range section {
		if strings.HasPrefix(ln, "## ") &&
			strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(ln, "## ")), name+":") {
			return ln
		}
	}
	return ""
}

// contextTail returns up to n non-doc lines from section (the rule line and
// the first recipe lines) to anchor the hunk.
func contextTail(section []string, n int) []string {
	var out []string
	for _, ln := range section {
		if strings.HasPrefix(ln, "## ") {
			continue
		}
		out = append(out, ln)
		if len(out) >= n {
			break
		}
	}
	return out
}
