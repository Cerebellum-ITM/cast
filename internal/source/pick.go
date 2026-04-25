package source

import (
	"fmt"
	"strings"
)

// ParsePickSpec converts the raw `[pick=…]` value into a sequence of pick
// steps. Aliases (from `[as=a,b,…]`) are mapped to steps in order; remaining
// steps fall back to `CAST_PICK_<n>` when emitted by the runner.
//
// Grammar (informal):
//
//	spec     := group (";" group)*
//	group    := segment ("/" segment)*
//	segment  := literal | "*" ("~" filter)?
//	literal  := any path component without "*"
//	filter   := substring or glob (e.g. `addons`, `*addons*`)
//
// Each `*` becomes one PickStep. Within a group, subsequent `*` segments are
// nested under the previous selection (BaseDir contains a `{pickN}`
// placeholder). Across groups (separated by `;`) selections are independent
// and resolved from the working directory.
//
// Examples:
//
//	./*~addons/*       → 2 steps. step1: "." filter "addons*", step2: "{pick1}"
//	services/*/*       → 2 steps. step1: "services", step2: "services/{pick1}"
//	./*~odoo; cfg/*    → 2 independent steps. step1: "." filter "odoo*",
//	                     step2: "cfg"
func ParsePickSpec(raw string, aliases []string) []PickStep {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var steps []PickStep
	groups := strings.Split(raw, ";")
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		steps = append(steps, parsePickGroup(group, len(steps))...)
	}
	for i := range steps {
		if i < len(aliases) && aliases[i] != "" {
			steps[i].Alias = aliases[i]
		}
	}
	return steps
}

// parsePickGroup parses one slash-delimited group. baseStepIdx is the global
// index of the first step emitted by this group (used to number {pickN}
// placeholders when nesting).
func parsePickGroup(group string, baseStepIdx int) []PickStep {
	segs := strings.Split(group, "/")
	var literalPrefix []string
	var steps []PickStep
	stepCountInGroup := 0
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			// leading "./..." case → represent cwd as "."
			if len(literalPrefix) == 0 && len(steps) == 0 {
				literalPrefix = append(literalPrefix, ".")
			}
			continue
		}
		if !strings.HasPrefix(seg, "*") {
			// literal directory component
			if len(steps) == 0 {
				literalPrefix = append(literalPrefix, seg)
			} else {
				// literal after a pick: append to the previous template so the
				// next pick descends into it.
				steps[len(steps)-1].BaseDirTemplate = joinPath(steps[len(steps)-1].BaseDirTemplate, seg)
			}
			continue
		}
		// "*" segment, possibly with "~filter"
		filter := ""
		if idx := strings.Index(seg, "~"); idx >= 0 {
			filter = strings.TrimSpace(seg[idx+1:])
		}
		var base string
		if stepCountInGroup == 0 {
			base = strings.Join(literalPrefix, "/")
			if base == "" {
				base = "."
			}
		} else {
			// Nested pick: descend into the previous selection. The placeholder
			// uses the global 1-based index of the previous step.
			prev := steps[len(steps)-1]
			placeholder := fmt.Sprintf("{pick%d}", baseStepIdx+stepCountInGroup)
			base = joinPath(prev.BaseDirTemplate, placeholder)
		}
		steps = append(steps, PickStep{BaseDirTemplate: base, Filter: filter})
		stepCountInGroup++
	}
	return steps
}

// joinPath concatenates two path fragments with a single "/", trimming a
// trailing slash from a and a leading slash from b. Empty fragments are
// skipped so `joinPath(".", "x") == "./x"`.
func joinPath(a, b string) string {
	a = strings.TrimRight(a, "/")
	b = strings.TrimLeft(b, "/")
	switch {
	case a == "" && b == "":
		return ""
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "/" + b
}

// ResolvePickBase substitutes `{pickN}` placeholders in tmpl using the
// previously chosen folder names. selections[0] corresponds to {pick1}.
func ResolvePickBase(tmpl string, selections []string) string {
	out := tmpl
	for i, sel := range selections {
		ph := fmt.Sprintf("{pick%d}", i+1)
		out = strings.ReplaceAll(out, ph, sel)
	}
	return out
}

// PickVarName returns the alias for step i (1-based), or `CAST_PICK_<i>` when
// no alias was supplied.
func PickVarName(step PickStep, oneBasedIdx int) string {
	if step.Alias != "" {
		return step.Alias
	}
	return fmt.Sprintf("CAST_PICK_%d", oneBasedIdx)
}

// MatchPickFilter reports whether name matches the pick filter. Empty filter
// always matches. A filter without `*` is treated as a substring match;
// otherwise simple `*`-glob semantics apply (case-insensitive).
func MatchPickFilter(name, filter string) bool {
	if filter == "" {
		return true
	}
	n := strings.ToLower(name)
	f := strings.ToLower(filter)
	if !strings.Contains(f, "*") {
		return strings.Contains(n, f)
	}
	return globMatch(f, n)
}

// globMatch is a tiny recursive `*`-only glob; sufficient for pick filters.
func globMatch(pattern, s string) bool {
	for {
		if pattern == "" {
			return s == ""
		}
		star := strings.IndexByte(pattern, '*')
		if star < 0 {
			return pattern == s
		}
		head := pattern[:star]
		if !strings.HasPrefix(s, head) {
			return false
		}
		s = s[len(head):]
		pattern = pattern[star+1:]
		if pattern == "" {
			return true
		}
		// Find the smallest s suffix that matches the rest.
		for i := 0; i <= len(s); i++ {
			if globMatch(pattern, s[i:]) {
				return true
			}
		}
		return false
	}
}
