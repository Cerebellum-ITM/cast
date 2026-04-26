package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := dirOverride
	SetDirForTest(dir)
	t.Cleanup(func() { SetDirForTest(prev) })
	return dir
}

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"deploy", "deploy"},
		{"  deploy  ", "deploy"},
		{"docker-clean", "docker-clean"},
		{"my_target_2", "my_target_2"},
		{"bad/name", "badname"},
		{"with spaces", "withspaces"},
		{"foo.bar", "foobar"},
		{"!!!", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := SanitizeName(c.in)
		if got != c.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSaveLoadDelete(t *testing.T) {
	setupTempDir(t)

	body := "## deploy: Deploy a service [tags=prod]\n" +
		"deploy:\n" +
		"\t@echo deploying\n" +
		"\t@./deploy.sh\n"

	if err := Save(Snippet{Name: "deploy", Body: body}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load("deploy")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != "deploy" {
		t.Errorf("Name = %q, want deploy", got.Name)
	}
	if got.Desc != "Deploy a service" {
		t.Errorf("Desc = %q, want Deploy a service", got.Desc)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "prod" {
		t.Errorf("Tags = %v, want [prod]", got.Tags)
	}
	if !strings.Contains(got.Body, "@echo deploying") {
		t.Errorf("Body missing recipe content: %q", got.Body)
	}

	if err := Delete("deploy"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Load("deploy"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load after delete: got %v, want ErrNotFound", err)
	}
	if err := Delete("deploy"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete twice: got %v, want ErrNotFound", err)
	}
}

func TestSaveNormalizesName(t *testing.T) {
	setupTempDir(t)
	// User passes name `clean-bin` but the body still references `cleanup`.
	// Save must rewrite both the doc-line and the target line.
	body := "## cleanup: Wipe build artefacts\n" +
		"cleanup:\n" +
		"\trm -rf bin/\n"
	if err := Save(Snippet{Name: "clean-bin", Body: body}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load("clean-bin")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.Contains(got.Body, "## clean-bin: Wipe build artefacts") {
		t.Errorf("doc-line not normalized: %q", got.Body)
	}
	if !strings.Contains(got.Body, "clean-bin:") {
		t.Errorf("target line not normalized: %q", got.Body)
	}
	if strings.Contains(got.Body, "cleanup:") {
		t.Errorf("old target name still present: %q", got.Body)
	}
}

func TestSaveAddsMissingDocLine(t *testing.T) {
	setupTempDir(t)
	body := "build:\n\tgo build ./...\n"
	if err := Save(Snippet{Name: "build", Body: body}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load("build")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.HasPrefix(got.Body, "## build:") {
		t.Errorf("expected synthetic doc-line, got body: %q", got.Body)
	}
}

func TestSaveInvalidName(t *testing.T) {
	setupTempDir(t)
	if err := Save(Snippet{Name: "!!!", Body: "x:\n\ttrue\n"}); !errors.Is(err, ErrInvalidName) {
		t.Errorf("Save with invalid name: got %v, want ErrInvalidName", err)
	}
}

func TestListEmptyDir(t *testing.T) {
	setupTempDir(t)
	got, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List on empty dir: got %d snippets, want 0", len(got))
	}
}

func TestListSorted(t *testing.T) {
	dir := setupTempDir(t)
	for _, name := range []string{"zebra", "alpha", "mango"} {
		body := "## " + name + ": stub\n" + name + ":\n\ttrue\n"
		if err := Save(Snippet{Name: name, Body: body}); err != nil {
			t.Fatalf("Save %s: %v", name, err)
		}
	}
	// Drop a non-.mk file to confirm it's ignored.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}
	got, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("List len = %d, want 3", len(got))
	}
	wantOrder := []string{"alpha", "mango", "zebra"}
	for i, s := range got {
		if s.Name != wantOrder[i] {
			t.Errorf("List[%d].Name = %q, want %q", i, s.Name, wantOrder[i])
		}
	}
}

func TestParseDocSummary(t *testing.T) {
	body := "## deploy: Deploy a service [tags=prod,ci] [confirm]\n" +
		"deploy:\n\ttrue\n"
	desc, tags := parseDocSummary(body)
	if desc != "Deploy a service" {
		t.Errorf("desc = %q, want Deploy a service", desc)
	}
	if len(tags) != 2 || tags[0] != "prod" || tags[1] != "ci" {
		t.Errorf("tags = %v, want [prod ci]", tags)
	}
}
