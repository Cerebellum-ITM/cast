package ai

import (
	"fmt"
	"strings"
)

// maxRecipeLines caps how many recipe lines are sent per target so a single
// huge target cannot blow the prompt budget.
const maxRecipeLines = 40

// SystemPrompt returns the system message. `allowed` is the categorical tag
// vocabulary; when empty the model is told to choose tags freely.
//
// The model must answer with strict JSON of the shape:
//
//	{
//	  "annotations": [{"name": "...", "desc": "...", "tags": ["..."]}],
//	  "skipped":     [{"name": "...", "reason": "..."}]
//	}
func SystemPrompt(allowed []string) string {
	var tagRule string
	if len(allowed) > 0 {
		tagRule = "1–3 categorical tags from this allowed list: " +
			strings.Join(allowed, ", ") + ". If none fits, return an empty array."
	} else {
		tagRule = "0–3 short, lowercase categorical tags. If none fits, return an empty array."
	}

	return strings.Join([]string{
		"You annotate GNU Make targets with doc-lines for the `cast` task runner.",
		"For each target, return:",
		"  - desc: a concise English description (<= 80 chars), starting with an",
		"    infinitive verb (e.g. \"Build the production Docker image\"). No trailing period.",
		"  - tags: " + tagRule,
		"Never invent target names: only use names present in the input.",
		"Skip a target when its recipe is empty or you cannot infer its purpose;",
		"list each skipped target in `skipped` with a short reason.",
		"Do not infer behavioural flags ([stream], [confirm], [interactive], etc.) —",
		"only the description and categorical tags.",
		"Respond with STRICT JSON only, no prose, no code fences:",
		`  {"annotations":[{"name":"...","desc":"...","tags":["..."]}],"skipped":[{"name":"...","reason":"..."}]}`,
	}, "\n")
}

// UserPayload formats the request's targets into the per-target block the
// model consumes. Recipe lines beyond maxRecipeLines are truncated.
func UserPayload(req Request) string {
	var b strings.Builder
	if req.OverwriteAll {
		b.WriteString("Re-annotate every target below, replacing any EXISTING_DOC.\n\n")
	} else {
		b.WriteString("Annotate the targets below.\n\n")
	}
	for i, t := range req.Targets {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "TARGET: %s\nRECIPE:\n", t.Name)
		recipe := t.Recipe
		truncated := false
		if len(recipe) > maxRecipeLines {
			recipe = recipe[:maxRecipeLines]
			truncated = true
		}
		if len(recipe) == 0 {
			b.WriteString("  (no recipe)\n")
		}
		for _, ln := range recipe {
			b.WriteString("  ")
			b.WriteString(ln)
			b.WriteString("\n")
		}
		if truncated {
			b.WriteString("  … (truncated)\n")
		}
		if t.ExistingDocLine != "" {
			fmt.Fprintf(&b, "EXISTING_DOC: %s\n", t.ExistingDocLine)
		} else {
			b.WriteString("EXISTING_DOC: <empty>\n")
		}
	}
	return b.String()
}
