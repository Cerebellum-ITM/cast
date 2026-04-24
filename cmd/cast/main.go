package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/db"
	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui"
)

const usage = `cast — a beautiful task runner

Usage:
  cast [flags]              launch the TUI
  cast init                 create .cast.toml template in the current directory
  cast config               show active config file paths
  cast env                  open the TUI on the .env tab
  cast env set KEY=VALUE    set a variable (persisted to .env + db)
  cast env get KEY          print a variable's value
  cast env list             list all variables
  cast shortcut list        show assigned shortcuts for all commands
  cast shortcut set CMD K   assign single-char shortcut K to command CMD
  cast shortcut unset CMD   remove shortcut for CMD (falls back to auto)
  cast tags list            show category tags for all commands
  cast tags set CMD a,b     write [tags=a,b] on CMD's Makefile doc line
  cast tags unset CMD       remove [tags=...] from CMD's doc line

Flags:
  -env   string   environment override: local | staging | prod
  -theme string   theme override: catppuccin | dracula | nord
`

func main() {
	// Subcommands are checked before flag.Parse so they can have their own
	// argument handling without conflicting with the TUI flags.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			runInit()
			return
		case "config":
			runConfig()
			return
		case "env":
			runEnvCommand(os.Args[2:])
			return
		case "shortcut":
			runShortcutCommand(os.Args[2:])
			return
		case "tags":
			runTagsCommand(os.Args[2:])
			return
		case "-h", "--help", "help":
			fmt.Print(usage)
			return
		}
	}

	var (
		flagEnv   = flag.String("env", "", "environment override: local | staging | prod")
		flagTheme = flag.String("theme", "", "theme override: catppuccin | dracula | nord")
	)
	flag.Usage = func() { fmt.Print(usage) }
	flag.Parse()

	cfg, err := config.Load(*flagEnv, *flagTheme)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast: config: %v\n", err)
		os.Exit(1)
	}

	src := &source.MakefileSource{Path: cfg.SourcePath}
	commands, err := src.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast: warning: %v\n", err)
		commands = nil
	}

	if len(cfg.ConfirmTargets) > 0 {
		confirmSet := make(map[string]bool, len(cfg.ConfirmTargets))
		for _, t := range cfg.ConfirmTargets {
			confirmSet[t] = true
		}
		for i := range commands {
			if confirmSet[commands[i].Name] {
				commands[i].Confirm = true
			}
		}
	}

	// Shortcut overrides: cast.toml [commands.shortcuts] wins over Makefile
	// [sc=X] tags and auto-inference. Empty value = clear shortcut (icon).
	for i := range commands {
		if v, ok := cfg.Shortcuts[commands[i].Name]; ok {
			if len(v) > 1 {
				v = v[:1]
			}
			commands[i].Shortcut = v
		}
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast: db: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	m := tui.New(cfg, commands, database)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "cast: %v\n", err)
		os.Exit(1)
	}
}

// runInit creates .cast.toml in the current working directory.
// Optionally accepts the environment name as the first argument:
//
//	cast init           → name = "dev"
//	cast init staging   → name = "staging"
//	cast init prod      → name = "prod"
func runInit() {
	envName := "dev"
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "staging", "stg":
			envName = "staging"
		case "prod", "production", "prd":
			envName = "prod"
		case "dev", "local", "":
			envName = "dev"
		default:
			fmt.Fprintf(os.Stderr, "cast init: unknown environment %q (dev | staging | prod)\n", os.Args[2])
			os.Exit(1)
		}
	}

	dest := config.LocalPath()
	if _, err := os.Stat(dest); err == nil {
		fmt.Fprintf(os.Stderr, "cast init: %s already exists\n", dest)
		os.Exit(1)
	}

	if err := config.WriteLocalTemplate(dest, envName); err != nil {
		fmt.Fprintf(os.Stderr, "cast init: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("created %s  (env: %s)\n", dest, envName)
	fmt.Println("edit [env] file to point at your project's .env file.")
}

// runConfig prints config file paths, their status, and the resolved values.
func runConfig() {
	shortPath := func(p string) string {
		if home, err := os.UserHomeDir(); err == nil {
			if r, err := filepath.Rel(home, p); err == nil {
				return "~/" + r
			}
		}
		return p
	}

	fileStatus := func(p string) string {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return "not found"
		}
		return "present"
	}

	// Load first so EnsureGlobal() runs before we check file status.
	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("config files:")
	fmt.Printf("  global  %s  (%s)\n", shortPath(config.GlobalPath()), fileStatus(config.GlobalPath()))
	fmt.Printf("  local   %s  (%s)\n", shortPath(config.LocalPath()), fileStatus(config.LocalPath()))

	fmt.Println("\nresolved values:")
	fmt.Printf("  env        %s\n", cfg.Env)
	fmt.Printf("  theme      %s\n", cfg.Theme)
	fmt.Printf("  env-file   %s\n", cfg.EnvFilePath)
	fmt.Printf("  source     %s (%s)\n", cfg.SourcePath, cfg.SourceType)
	fmt.Printf("  history    max=%d\n", cfg.HistoryMax)
	fmt.Printf("  db         %s  (%s)\n", shortPath(cfg.DBPath), fileStatus(cfg.DBPath))
	fmt.Printf("  layout     sidebar=%d%%  output=%d%%  center=%v\n",
		cfg.SidebarWidthPct, cfg.OutputWidthPct, cfg.ShowCenterPanel)
}

// runEnvCommand dispatches cast env subcommands.
func runEnvCommand(args []string) {
	if len(args) == 0 {
		// Open TUI on the .env tab.
		cfg, err := config.Load("", "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "cast env: config: %v\n", err)
			os.Exit(1)
		}
		src := &source.MakefileSource{Path: cfg.SourcePath}
		commands, _ := src.Load()
		database, err := db.Open(cfg.DBPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cast env: db: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()
		m := tui.NewOnTab(cfg, commands, database, tui.TabEnv)
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "cast: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "set":
		runEnvSet(args[1:])
	case "get":
		runEnvGet(args[1:])
	case "list":
		runEnvList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "cast env: unknown subcommand %q\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: cast env [set KEY=VALUE | get KEY | list]")
		os.Exit(1)
	}
}

func runEnvSet(args []string) {
	fs := flag.NewFlagSet("env set", flag.ExitOnError)
	sensitive := fs.Bool("sensitive", false, "mark variable as sensitive")
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: cast env set KEY=VALUE [--sensitive]")
		os.Exit(1)
	}

	pair := fs.Arg(0)
	idx := strings.Index(pair, "=")
	if idx < 1 {
		fmt.Fprintf(os.Stderr, "cast env set: invalid format %q — expected KEY=VALUE\n", pair)
		os.Exit(1)
	}
	key := pair[:idx]
	value := pair[idx+1:]

	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast env set: config: %v\n", err)
		os.Exit(1)
	}

	ef, err := source.ParseEnvFile(cfg.EnvFilePath)
	if os.IsNotExist(err) {
		ef = &source.EnvFile{Filename: cfg.EnvFilePath}
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "cast env set: read %s: %v\n", cfg.EnvFilePath, err)
		os.Exit(1)
	}

	isSens := *sensitive || source.IsSensitiveKey(key)
	var oldValue sql.NullString
	found := false
	for i, v := range ef.Vars {
		if v.Key == key {
			oldValue = sql.NullString{String: v.Value, Valid: true}
			ef.Vars[i].Value = value
			if isSens {
				ef.Vars[i].Sensitive = true
			}
			found = true
			break
		}
	}
	if !found {
		ef.Vars = append(ef.Vars, source.EnvVar{
			Key:       key,
			Value:     value,
			Sensitive: isSens,
		})
	}

	if err := source.WriteEnvFile(ef, cfg.EnvFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "cast env set: write: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast env set: db: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = database.InsertEnvChange(ctx, db.EnvChange{
		Key:       key,
		OldValue:  oldValue,
		NewValue:  value,
		Sensitive: isSens,
		EnvFile:   cfg.EnvFilePath,
		ChangedAt: time.Now(),
		ChangedBy: "cli",
	})

	fmt.Printf("set %s in %s\n", key, cfg.EnvFilePath)
}

func runEnvGet(args []string) {
	fs := flag.NewFlagSet("env get", flag.ExitOnError)
	reveal := fs.Bool("reveal", false, "show value even if sensitive")
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: cast env get KEY [--reveal]")
		os.Exit(1)
	}
	key := fs.Arg(0)

	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast env get: config: %v\n", err)
		os.Exit(1)
	}

	ef, err := source.ParseEnvFile(cfg.EnvFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast env get: read %s: %v\n", cfg.EnvFilePath, err)
		os.Exit(1)
	}

	for _, v := range ef.Vars {
		if v.Key == key {
			if v.Sensitive && !*reveal {
				fmt.Printf("%s=••••••••\n", key)
			} else {
				fmt.Printf("%s=%s\n", key, v.Value)
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "cast env get: key %q not found in %s\n", key, cfg.EnvFilePath)
	os.Exit(1)
}

func runEnvList(args []string) {
	fs := flag.NewFlagSet("env list", flag.ExitOnError)
	reveal := fs.Bool("reveal", false, "show sensitive values")
	_ = fs.Parse(args)

	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast env list: config: %v\n", err)
		os.Exit(1)
	}

	ef, err := source.ParseEnvFile(cfg.EnvFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast env list: read %s: %v\n", cfg.EnvFilePath, err)
		os.Exit(1)
	}

	for _, v := range ef.Vars {
		val := v.Value
		if v.Sensitive && !*reveal {
			val = "••••••••"
		}
		fmt.Printf("%s=%s\n", v.Key, val)
	}
}

// runShortcutCommand dispatches `cast shortcut` subcommands.
func runShortcutCommand(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "cast shortcut: missing subcommand (list | set | unset)")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		runShortcutList()
	case "set":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "cast shortcut set: expected 2 args — CMD and KEY (got "+fmt.Sprint(len(args)-1)+")")
			os.Exit(1)
		}
		runShortcutSet(args[1], args[2])
	case "unset":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "cast shortcut unset: expected 1 arg — CMD")
			os.Exit(1)
		}
		runShortcutSet(args[1], "")
	default:
		fmt.Fprintf(os.Stderr, "cast shortcut: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}

func runShortcutList() {
	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast shortcut list: config: %v\n", err)
		os.Exit(1)
	}
	src := &source.MakefileSource{Path: cfg.SourcePath}
	cmds, err := src.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast shortcut list: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("shortcuts for %d commands (source: %s):\n\n", len(cmds), cfg.SourcePath)
	fmt.Printf("  %-3s  %-24s  %s\n", "KEY", "COMMAND", "ORIGIN")
	for _, c := range cmds {
		origin := "makefile"
		if _, ok := cfg.Shortcuts[c.Name]; ok {
			origin = ".cast.toml"
		}
		// Apply local-toml override (matching main.go behavior).
		short := c.Shortcut
		if v, ok := cfg.Shortcuts[c.Name]; ok {
			if len(v) > 1 {
				v = v[:1]
			}
			short = v
		}
		if short == "" {
			fmt.Printf("  %-3s  %-24s  %s\n", "·", c.Name, origin)
		} else {
			fmt.Printf("  %-3s  %-24s  %s\n", short, c.Name, origin)
		}
	}
}

// runShortcutSet upserts [commands.shortcuts] in .cast.toml. An empty value
// deletes the entry (used by `unset`).
func runShortcutSet(cmdName, key string) {
	if len(key) > 1 {
		fmt.Fprintf(os.Stderr, "cast shortcut: KEY must be a single character (got %q)\n", key)
		os.Exit(1)
	}
	path := config.LocalPath()
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "cast shortcut: read %s: %v\n", path, err)
		os.Exit(1)
	}
	updated := upsertShortcut(string(raw), cmdName, key)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "cast shortcut: write %s: %v\n", path, err)
		os.Exit(1)
	}
	if key == "" {
		fmt.Printf("✓ cleared shortcut for %s in %s\n", cmdName, path)
	} else {
		fmt.Printf("✓ set %s → %q in %s\n", cmdName, key, path)
	}
}

// upsertShortcut rewrites src so that [commands.shortcuts] contains cmdName=key.
// An empty key removes the entry. Comments and unrelated sections are preserved
// verbatim because we only edit the target block line-by-line.
func upsertShortcut(src, cmdName, key string) string {
	header := "[commands.shortcuts]"
	lines := strings.Split(src, "\n")

	// Locate block start.
	blockStart := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == header {
			blockStart = i
			break
		}
	}

	// No block yet: append one at the end.
	if blockStart == -1 {
		if key == "" {
			return src // nothing to remove
		}
		suffix := fmt.Sprintf("\n%s\n%q = %q\n", header, cmdName, key)
		// Avoid double blank line if src already ends with newline.
		if strings.HasSuffix(src, "\n\n") || src == "" {
			suffix = suffix[1:]
		}
		return src + suffix
	}

	// Find block end (next section header or EOF).
	blockEnd := len(lines)
	for i := blockStart + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			blockEnd = i
			break
		}
	}

	// Scan existing entries; update or remove.
	keyPrefix := fmt.Sprintf("%q", cmdName) // "cmdName"
	altPrefix := cmdName                    // bare form
	found := false
	for i := blockStart + 1; i < blockEnd; i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		// Extract key up to '='
		eq := strings.Index(t, "=")
		if eq < 0 {
			continue
		}
		lhs := strings.TrimSpace(t[:eq])
		if lhs == keyPrefix || lhs == altPrefix {
			found = true
			if key == "" {
				// Remove this line.
				lines = append(lines[:i], lines[i+1:]...)
				blockEnd--
			} else {
				lines[i] = fmt.Sprintf("%q = %q", cmdName, key)
			}
			break
		}
	}
	if !found && key != "" {
		// Insert just after header.
		newLine := fmt.Sprintf("%q = %q", cmdName, key)
		lines = append(lines[:blockStart+1],
			append([]string{newLine}, lines[blockStart+1:]...)...)
	}
	return strings.Join(lines, "\n")
}

// runTagsCommand dispatches `cast tags` subcommands. Unlike shortcut overrides
// (which live in .cast.toml), tags are stored as a `[tags=a,b,c]` marker on
// the Makefile doc line itself — the same place `cast` reads them from.
func runTagsCommand(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "cast tags: missing subcommand (list | set | unset)")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		runTagsList()
	case "set":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "cast tags set: expected 2 args — CMD and comma-separated TAGS")
			os.Exit(1)
		}
		runTagsSet(args[1], args[2])
	case "unset":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "cast tags unset: expected 1 arg — CMD")
			os.Exit(1)
		}
		runTagsSet(args[1], "")
	default:
		fmt.Fprintf(os.Stderr, "cast tags: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}

func runTagsList() {
	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast tags list: config: %v\n", err)
		os.Exit(1)
	}
	cmds, err := (&source.MakefileSource{Path: cfg.SourcePath}).Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast tags list: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("tags for %d commands (source: %s):\n\n", len(cmds), cfg.SourcePath)
	fmt.Printf("  %-24s  %s\n", "COMMAND", "TAGS")
	for _, c := range cmds {
		tags := "·"
		if len(c.Tags) > 0 {
			tags = strings.Join(c.Tags, ", ")
		}
		fmt.Printf("  %-24s  %s\n", c.Name, tags)
	}
}

func runTagsSet(cmdName, csv string) {
	cfg, err := config.Load("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast tags: config: %v\n", err)
		os.Exit(1)
	}
	var tags []string
	if csv != "" {
		for _, p := range strings.Split(csv, ",") {
			if p = strings.TrimSpace(p); p != "" {
				tags = append(tags, p)
			}
		}
	}
	if err := source.UpdateMakefileTags(cfg.SourcePath, cmdName, tags); err != nil {
		fmt.Fprintf(os.Stderr, "cast tags: %v\n", err)
		os.Exit(1)
	}
	if len(tags) == 0 {
		fmt.Printf("✓ cleared tags for %s in %s\n", cmdName, cfg.SourcePath)
	} else {
		fmt.Printf("✓ set %s → [%s] in %s\n", cmdName, strings.Join(tags, ","), cfg.SourcePath)
	}
}
