package source

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMakefile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write makefile: %v", err)
	}
	return path
}

func TestAppendMakefileTarget_FreshTarget(t *testing.T) {
	path := writeMakefile(t, "## build: Compile.\nbuild:\n\tgo build ./...\n")
	snippet := "## test: Run tests.\ntest:\n\tgo test ./...\n"
	if err := AppendMakefileTarget(path, snippet); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "## test: Run tests.") {
		t.Errorf("snippet doc-line missing: %q", string(got))
	}
	if !strings.Contains(string(got), "test:\n\tgo test") {
		t.Errorf("snippet recipe missing: %q", string(got))
	}
	if !strings.Contains(string(got), "## build: Compile.") {
		t.Errorf("existing target lost: %q", string(got))
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Errorf("file missing trailing newline")
	}
	// Blank line between existing content and snippet.
	if !strings.Contains(string(got), "\n\n## test:") {
		t.Errorf("expected blank line separator before snippet: %q", string(got))
	}
}

func TestAppendMakefileTarget_Conflict(t *testing.T) {
	path := writeMakefile(t, "## build: Compile.\nbuild:\n\tgo build ./...\n")
	snippet := "## build: shadow\nbuild:\n\techo other\n"
	err := AppendMakefileTarget(path, snippet)
	if !errors.Is(err, ErrTargetExists) {
		t.Errorf("got %v, want ErrTargetExists", err)
	}
	// Original content must be intact.
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "echo other") {
		t.Errorf("snippet leaked into Makefile despite conflict")
	}
}

func TestAppendMakefileTarget_NoTrailingNewline(t *testing.T) {
	// No trailing newline on either side.
	path := writeMakefile(t, "build:\n\tgo build")
	snippet := "test:\n\tgo test"
	if err := AppendMakefileTarget(path, snippet); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(got), "\n") {
		t.Errorf("expected trailing newline, got %q", string(got))
	}
	if !strings.Contains(string(got), "build:\n\tgo build\n\ntest:") {
		t.Errorf("expected blank-line separation: %q", string(got))
	}
}

func TestExtractMakefileTarget_WithDocLine(t *testing.T) {
	src := `## build: Compile.
build:
	@go build ./...
	@echo done

## test: Run tests.
test:
	@go test ./...
`
	path := writeMakefile(t, src)
	body, err := ExtractMakefileTarget(path, "build")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.HasPrefix(body, "## build: Compile.\n") {
		t.Errorf("missing doc-line: %q", body)
	}
	if !strings.Contains(body, "@go build ./...") {
		t.Errorf("missing recipe: %q", body)
	}
	if strings.Contains(body, "test:") {
		t.Errorf("leaked next target: %q", body)
	}
	if !strings.HasSuffix(body, "\n") {
		t.Errorf("missing trailing newline")
	}
}

func TestExtractMakefileTarget_WithoutDocLine(t *testing.T) {
	src := "build:\n\t@go build\n"
	path := writeMakefile(t, src)
	body, err := ExtractMakefileTarget(path, "build")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if strings.HasPrefix(body, "## ") {
		t.Errorf("synthesised doc-line; should preserve absence: %q", body)
	}
	if !strings.HasPrefix(body, "build:") {
		t.Errorf("expected target line first: %q", body)
	}
}

func TestExtractMakefileTarget_NotFound(t *testing.T) {
	path := writeMakefile(t, "build:\n\t@go build\n")
	_, err := ExtractMakefileTarget(path, "missing")
	if !errors.Is(err, ErrTargetNotFound) {
		t.Errorf("got %v, want ErrTargetNotFound", err)
	}
}

// TestExtractMakefileTarget_SkipsPhonyContinuation guards against a
// regression where findTargetIndex matched the space-indented continuation
// of a `.PHONY` declaration as the target itself, returning the .PHONY
// line list as the snippet body and dropping the actual recipe.
func TestExtractMakefileTarget_SkipsPhonyContinuation(t *testing.T) {
	src := ".PHONY: build run clean \\\n" +
		"        pick_demo pick_nested pick_aliased\n" +
		"\n" +
		"## pick_demo: List a folder.\n" +
		"pick_demo:\n" +
		"\t@echo \"Elegiste: $(CAST_PICK_1)\"\n" +
		"\t@ls -la $(CAST_PICK_1)\n"
	path := writeMakefile(t, src)
	body, err := ExtractMakefileTarget(path, "pick_demo")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(body, "## pick_demo: List a folder.") {
		t.Errorf("doc-line missing: %q", body)
	}
	if !strings.Contains(body, "@ls -la $(CAST_PICK_1)") {
		t.Errorf("recipe missing: %q", body)
	}
	if strings.Contains(body, "pick_demo pick_nested pick_aliased") {
		t.Errorf(".PHONY continuation leaked into snippet: %q", body)
	}
}

// TestFindTargetIndex_RejectsAssignments ensures `VAR := name`-style
// assignments are not mistaken for a target declaration.
func TestFindTargetIndex_RejectsAssignments(t *testing.T) {
	lines := []string{
		"BIN := bin/cast",
		"build:",
		"\tgo build ./...",
	}
	got := findTargetIndex(lines, "BIN")
	if got != -1 {
		t.Errorf("findTargetIndex(BIN) = %d, want -1 (variable assignment)", got)
	}
	got = findTargetIndex(lines, "build")
	if got != 1 {
		t.Errorf("findTargetIndex(build) = %d, want 1", got)
	}
}

func TestSnippetTargetName(t *testing.T) {
	cases := []struct {
		body, want string
	}{
		{"## deploy: x\ndeploy:\n\ttrue\n", "deploy"},
		{"deploy:\n\ttrue\n", "deploy"},
		{"deploy: dep1 dep2\n\ttrue\n", "deploy"},
		{"VAR = 1\nbuild:\n\ttrue\n", "build"},
		{"# comment\n\nbuild:\n\ttrue\n", "build"},
		{"\t@echo nothing\n", ""},
	}
	for _, c := range cases {
		got := snippetTargetName(c.body)
		if got != c.want {
			t.Errorf("snippetTargetName(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}
