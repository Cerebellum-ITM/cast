package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_SourceFileLayering verifies the source-file override chain:
// default Makefile < local [source].path < CAST_MAKEFILE < -f flag.
func TestLoad_SourceFileLayering(t *testing.T) {
	t.Setenv("HOME", t.TempDir())       // isolate the global config write
	t.Setenv("CAST_ENV", "")            // don't inherit a real env
	t.Setenv("CAST_MAKEFILE", "")       // start clean; subtests set it

	write := func(dir, name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x:\n\t@true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("default when nothing set", func(t *testing.T) {
		dir := t.TempDir()
		write(dir, "Makefile")
		t.Chdir(dir)
		cfg, err := Load(LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got := filepath.Base(cfg.SourcePath); got != "Makefile" {
			t.Fatalf("default source = %q, want Makefile", got)
		}
		if cfg.SourceFile != "Makefile" {
			t.Fatalf("SourceFile = %q, want Makefile", cfg.SourceFile)
		}
	})

	t.Run("local [source].path overrides default", func(t *testing.T) {
		dir := t.TempDir()
		write(dir, "Makefile")
		write(dir, "Makefile.personal")
		if err := os.WriteFile(filepath.Join(dir, ".cast.toml"),
			[]byte("[source]\npath = \"Makefile.personal\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		cfg, err := Load(LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got := filepath.Base(cfg.SourcePath); got != "Makefile.personal" {
			t.Fatalf("local source = %q, want Makefile.personal", got)
		}
	})

	t.Run("env overrides local, flag overrides env", func(t *testing.T) {
		dir := t.TempDir()
		write(dir, "Makefile")
		write(dir, "Makefile.personal")
		write(dir, "Makefile.env")
		write(dir, "Makefile.flag")
		if err := os.WriteFile(filepath.Join(dir, ".cast.toml"),
			[]byte("[source]\npath = \"Makefile.personal\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)

		t.Setenv("CAST_MAKEFILE", "Makefile.env")
		cfg, err := Load(LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got := filepath.Base(cfg.SourcePath); got != "Makefile.env" {
			t.Fatalf("env source = %q, want Makefile.env", got)
		}

		cfg, err = Load(LoadOptions{SourceFile: "Makefile.flag"})
		if err != nil {
			t.Fatal(err)
		}
		if got := filepath.Base(cfg.SourcePath); got != "Makefile.flag" {
			t.Fatalf("flag source = %q, want Makefile.flag", got)
		}
	})
}
