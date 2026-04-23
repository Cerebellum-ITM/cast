package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui"
)

func main() {
	var (
		flagEnv   = flag.String("env", "", "environment override: local | staging | prod")
		flagTheme = flag.String("theme", "", "theme override: catppuccin | dracula | nord")
	)
	flag.Parse()

	cfg := config.Default()

	// Priority: CLI flag > CAST_ENV env var > config default.
	if *flagEnv != "" {
		cfg.Env = config.ParseEnv(*flagEnv)
	} else if e := os.Getenv("CAST_ENV"); e != "" {
		cfg.Env = config.ParseEnv(e)
	}

	if *flagTheme != "" {
		cfg.Theme = config.Theme(*flagTheme)
	}

	src := &source.MakefileSource{Path: cfg.SourcePath}
	commands, err := src.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cast: warning: %v\n", err)
		commands = nil
	}

	m := tui.New(cfg, commands)
	// In bubbletea v2, AltScreen is declared on the View struct, not here.
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "cast: %v\n", err)
		os.Exit(1)
	}
}
