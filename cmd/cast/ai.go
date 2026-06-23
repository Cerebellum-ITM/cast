package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/ai"
	"github.com/Cerebellum-ITM/cast/internal/config"
)

// Exit codes for `cast ai annotate` (documented in usage and the spec):
//
//	0  annotations applied, or --dry-run/--json finished with no error
//	1  config error, missing API key, or Makefile not found
//	2  LLM error (timeout, non-200, invalid JSON) or write failure
//	3  user answered "n" at the confirmation prompt; nothing changed
const (
	exitOK       = 0
	exitConfig   = 1
	exitLLM      = 2
	exitDeclined = 3
)

const aiUsage = `cast ai annotate — autocomplete Makefile doc-lines with an LLM

Usage:
  cast ai annotate              annotate every target missing a doc-line
  cast ai annotate --all        annotate every target, overwriting existing doc-lines
  cast ai annotate --target X   annotate only target X (errors if X is absent)
  cast ai annotate --dry-run    print the proposed diff and exit, writing nothing
  cast ai annotate --yes        apply without the confirmation prompt
  cast ai annotate --json       print the proposed Plan as JSON (no changes)

The API key is read from the env var named by [ai].api_key_env (default
GROQ_API_KEY). Set it in your shell or ~/.config/cast/.env.
`

// cliDiffColors mirrors the catppuccin/dracula success/danger/dim tokens so
// the CLI diff reads the same as the TUI popup.
var cliDiffColors = &ai.DiffColors{
	Add:     lipgloss.Color("#50FA7B"),
	Del:     lipgloss.Color("#FF5555"),
	Context: lipgloss.Color("#7A88B8"),
}

// runAICommand dispatches `cast ai <subcommand>`.
func runAICommand(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(aiUsage)
		return
	}
	switch args[0] {
	case "annotate":
		runAIAnnotate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "cast ai: unknown subcommand %q\n\n%s", args[0], aiUsage)
		os.Exit(exitConfig)
	}
}

func runAIAnnotate(args []string) {
	fs := flag.NewFlagSet("ai annotate", flag.ExitOnError)
	var (
		target = fs.String("target", "", "annotate only this target")
		all    = fs.Bool("all", false, "annotate every target, overwriting existing doc-lines")
		dryRun = fs.Bool("dry-run", false, "print the diff and exit without writing")
		yes    = fs.Bool("yes", false, "apply without confirmation")
		asJSON = fs.Bool("json", false, "print the Plan as JSON and exit")
	)
	fs.Usage = func() { fmt.Print(aiUsage) }
	_ = fs.Parse(args)

	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast ai: config: %v\n", err)
		os.Exit(exitConfig)
	}

	data, err := os.ReadFile(cfg.SourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast ai: Makefile not found at %s: %v\n", cfg.SourcePath, err)
		os.Exit(exitConfig)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	mode := ai.OnlyMissingDoc
	switch {
	case *target != "":
		mode = ai.SingleTarget
	case *all:
		mode = ai.All
	}

	views, err := ai.BuildTargetViews(lines, mode, *target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast ai: %v\n", err)
		os.Exit(exitConfig)
	}
	if len(views) == 0 {
		fmt.Println("nothing to annotate — every target already has a doc-line")
		os.Exit(exitOK)
	}

	apiKey := os.Getenv(cfg.AIAPIKeyEnv)
	if apiKey == "" {
		fmt.Fprintf(os.Stderr,
			"cast ai: %s is not set. Export it in your shell or add it to ~/.config/cast/.env:\n  export %s=...\n",
			cfg.AIAPIKeyEnv, cfg.AIAPIKeyEnv)
		os.Exit(exitConfig)
	}

	provider := &ai.GroqProvider{
		APIKey:   apiKey,
		Model:    cfg.AIModel,
		Endpoint: cfg.AIEndpoint,
		HTTP:     &http.Client{Timeout: time.Duration(cfg.AITimeoutSecs) * time.Second},
	}
	req := ai.Request{
		Targets:      views,
		AllowedTags:  cfg.AIAllowedTags,
		OverwriteAll: mode != ai.OnlyMissingDoc,
		Model:        cfg.AIModel,
		MaxTargets:   cfg.AIMaxTargets,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AITimeoutSecs)*time.Second)
	defer cancel()

	start := time.Now()
	plan, err := provider.Annotate(ctx, req)
	elapsed := time.Since(start)
	// Single-line telemetry to stderr, kept out of the stdout diff.
	fmt.Fprintf(os.Stderr, "provider=%s model=%s targets=%d elapsed=%dms\n",
		cfg.AIProvider, cfg.AIModel, len(views), elapsed.Milliseconds())
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast ai: %v\n", err)
		os.Exit(exitLLM)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(plan); err != nil {
			fmt.Fprintf(os.Stderr, "cast ai: encode json: %v\n", err)
			os.Exit(exitLLM)
		}
		os.Exit(exitOK)
	}

	if len(plan.Annotations) == 0 {
		fmt.Println("the model proposed no annotations.")
		printSkipped(plan)
		os.Exit(exitOK)
	}

	colors := cliDiffColors
	if !isTTY(os.Stdout) {
		colors = nil
	}
	fmt.Println(ai.RenderDiff(plan, data, colors))
	printSkipped(plan)

	if *dryRun {
		os.Exit(exitOK)
	}

	if !*yes {
		fmt.Printf("\nApply %d annotation(s) to %s? [y/N] ", len(plan.Annotations), cfg.SourcePath)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("aborted; no changes written.")
			os.Exit(exitDeclined)
		}
	}

	if err := ai.ApplyPlan(plan, cfg.SourcePath); err != nil {
		fmt.Fprintf(os.Stderr, "cast ai: %v\n", err)
		os.Exit(exitLLM)
	}
	fmt.Printf("annotated %d target(s) in %s\n", len(plan.Annotations), cfg.SourcePath)
}

func printSkipped(plan ai.Plan) {
	if len(plan.Skipped) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\nskipped %d target(s):\n", len(plan.Skipped))
	for _, s := range plan.Skipped {
		fmt.Fprintf(os.Stderr, "  %s — %s\n", s.Name, s.Reason)
	}
}

// isTTY reports whether f is a character device (an interactive terminal), so
// the CLI can fall back to a plain diff when piped or redirected.
func isTTY(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
