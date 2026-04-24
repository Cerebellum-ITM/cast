package config

import (
	"os"
	"path/filepath"
	"testing"
)

// chdirTo switches into dir for the test and restores cwd on cleanup.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func TestResolveSourcePathFindsParent(t *testing.T) {
	tmp := t.TempDir()
	// Structure: tmp/Makefile, tmp/a/b/c (cwd).
	if err := os.WriteFile(filepath.Join(tmp, "Makefile"), []byte("ok:\n\t@:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	chdirTo(t, deep)

	got := resolveSourcePath("./Makefile", 5)
	want, _ := filepath.Abs(filepath.Join(tmp, "Makefile"))
	// tmpdir may resolve symlinks differently; compare via Stat.
	gi, err := os.Stat(got)
	if err != nil {
		t.Fatalf("resolved path %q not found: %v", got, err)
	}
	wi, _ := os.Stat(want)
	if !os.SameFile(gi, wi) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSourcePathDepthCap(t *testing.T) {
	tmp := t.TempDir()
	// Makefile is 3 levels up; depth=2 must NOT find it.
	if err := os.WriteFile(filepath.Join(tmp, "Makefile"), []byte("ok:\n\t@:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	chdirTo(t, deep)

	got := resolveSourcePath("./Makefile", 2)
	if got != "./Makefile" {
		t.Errorf("depth cap leaked: got %q, want unchanged input", got)
	}
}

func TestResolveSourcePathZeroDisablesWalkUp(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "Makefile"), []byte("ok:\n\t@:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	chdirTo(t, sub)

	got := resolveSourcePath("./Makefile", 0)
	if got != "./Makefile" {
		t.Errorf("depth=0 should not walk up: got %q", got)
	}
}

func TestResolveSourcePathCwdWins(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "Makefile"), []byte("root:\n\t@:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "Makefile"), []byte("sub:\n\t@:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdirTo(t, sub)

	got := resolveSourcePath("./Makefile", 5)
	// Should pick the one in sub/, not the one in tmp/.
	want, _ := filepath.Abs(filepath.Join(sub, "Makefile"))
	gi, _ := os.Stat(got)
	wi, _ := os.Stat(want)
	if !os.SameFile(gi, wi) {
		t.Errorf("cwd match lost: got %q, want %q", got, want)
	}
}

func TestResolveSourcePathAbsolutePassthrough(t *testing.T) {
	tmp := t.TempDir()
	abs := filepath.Join(tmp, "Makefile")
	if err := os.WriteFile(abs, []byte("ok:\n\t@:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveSourcePath(abs, 0)
	if got != abs {
		t.Errorf("abs path changed: got %q want %q", got, abs)
	}
}
