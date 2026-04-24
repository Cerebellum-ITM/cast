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
