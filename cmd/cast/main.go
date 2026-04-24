package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/db"
	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui"
)

const usage = `cast — a beautiful task runner

Usage:
  cast [flags]          launch the TUI
  cast init             create .cast.toml template in the current directory
  cast config           show active config file paths

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
}
