// Package ai proposes and applies Makefile doc-lines (`## name: desc [tags=…]`)
// by asking an LLM to describe each target. It is deliberately decoupled from
// the rest of cast: the core types and the Groq client import only the stdlib
// and internal/source. Only apply.go reaches for lipgloss, and only to colour
// the diff — the plain (uncoloured) render is exposed for callers that bring
// their own palette.
package ai

import "context"

// Annotation is what the LLM proposes for a single target.
type Annotation struct {
	Name string   // target name; must match an existing target
	Desc string   // proposed description (≤ 80 chars, infinitive verb)
	Tags []string // categorical tags from the allowed list
}

// SkipReason records a target the LLM declined or the filter excluded.
type SkipReason struct {
	Name   string
	Reason string // e.g. "already documented", "no recipe to infer from"
}

// Plan is the result of a single Annotate() call.
type Plan struct {
	Annotations []Annotation
	Skipped     []SkipReason
}

// TargetView is the per-target input handed to the provider.
type TargetView struct {
	Name            string
	Recipe          []string
	ExistingDocLine string
}

// Request is one annotation request. When len(Targets) exceeds MaxTargets the
// provider splits it into sequential batches and merges the results.
type Request struct {
	Targets      []TargetView
	AllowedTags  []string // empty = let the model decide freely
	OverwriteAll bool     // targets that already have a doc-line are included
	Model        string   // overrides the provider's default model when set
	MaxTargets   int      // batch size; 0 = no splitting
}

// Provider abstracts the LLM backend. Only GroqProvider is wired today; the
// interface exists so a future Ollama/Anthropic backend slots in without
// touching call sites.
type Provider interface {
	Annotate(ctx context.Context, req Request) (Plan, error)
}

// FilterMode selects which targets BuildTargetViews includes.
type FilterMode int

const (
	// OnlyMissingDoc includes targets that have no `## name:` doc-line.
	OnlyMissingDoc FilterMode = iota
	// All includes every target, even already-documented ones.
	All
	// SingleTarget includes exactly the target named in `only`.
	SingleTarget
)
