package ai

import (
	"fmt"
	"strings"

	"github.com/Cerebellum-ITM/cast/internal/source"
)

// BuildTargetViews parses a Makefile (as a slice of lines) and returns the
// TargetView for each target selected by `mode`. For SingleTarget, `only`
// must name an existing target or an error is returned. Recipe lines are
// returned with their leading tab stripped.
func BuildTargetViews(lines []string, mode FilterMode, only string) ([]TargetView, error) {
	names := source.MakefileTargetNames(lines)

	if mode == SingleTarget {
		found := false
		for _, n := range names {
			if n == only {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("target %q not found", only)
		}
		names = []string{only}
	}

	views := make([]TargetView, 0, len(names))
	for _, name := range names {
		doc, recipe := splitTargetSection(source.MakefileTargetLines(lines, name), name)
		if mode == OnlyMissingDoc && doc != "" {
			continue
		}
		views = append(views, TargetView{
			Name:            name,
			Recipe:          recipe,
			ExistingDocLine: doc,
		})
	}
	return views, nil
}

// splitTargetSection separates the `## name: …` doc-line (if any) from the
// tab-indented recipe lines of a target section as returned by
// source.MakefileTargetLines. Recipe lines have their leading tab removed.
func splitTargetSection(section []string, name string) (doc string, recipe []string) {
	for _, ln := range section {
		switch {
		case strings.HasPrefix(ln, "## ") &&
			strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(ln, "## ")), name+":"):
			doc = ln
		case strings.HasPrefix(ln, "\t"):
			recipe = append(recipe, strings.TrimPrefix(ln, "\t"))
		}
	}
	return doc, recipe
}
