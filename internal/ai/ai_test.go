package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleMakefile = `## test: run the suite
test:
	@go test ./...

build:
	@go build -o bin/cast ./cmd/cast

deploy:
	@echo deploying
	@scp bin/cast server:/usr/local/bin/
`

func TestBuildTargetViews_OnlyMissingDoc(t *testing.T) {
	lines := strings.Split(strings.TrimRight(sampleMakefile, "\n"), "\n")
	views, err := BuildTargetViews(lines, OnlyMissingDoc, "")
	if err != nil {
		t.Fatalf("BuildTargetViews: %v", err)
	}
	got := make([]string, len(views))
	for i, v := range views {
		got[i] = v.Name
	}
	// "test" already has a doc-line and must be excluded.
	want := []string{"build", "deploy"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("OnlyMissingDoc names = %v, want %v", got, want)
	}
	// deploy must carry its two recipe lines, tab-stripped.
	if len(views[1].Recipe) != 2 || views[1].Recipe[0] != "@echo deploying" {
		t.Fatalf("deploy recipe = %#v", views[1].Recipe)
	}
}

func TestBuildTargetViews_SingleTargetNotFound(t *testing.T) {
	lines := strings.Split(strings.TrimRight(sampleMakefile, "\n"), "\n")
	if _, err := BuildTargetViews(lines, SingleTarget, "nope"); err == nil {
		t.Fatal("expected error for missing target")
	}
}

func TestApplyPlan_WritesDocLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(path, []byte(sampleMakefile), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Annotations: []Annotation{
		{Name: "build", Desc: "Compile the cast binary", Tags: []string{"build"}},
		{Name: "deploy", Desc: "Ship the binary to the server", Tags: []string{"deploy"}},
	}}
	if err := ApplyPlan(plan, path); err != nil {
		t.Fatalf("ApplyPlan: %v", err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	for _, must := range []string{
		"## build: Compile the cast binary [tags=build]",
		"## deploy: Ship the binary to the server [tags=deploy]",
		"## test: run the suite", // untouched
	} {
		if !strings.Contains(s, must) {
			t.Errorf("output missing %q\n--- got ---\n%s", must, s)
		}
	}
	// The temp file must not linger.
	if _, err := os.Stat(filepath.Join(dir, ".Makefile.tmp")); !os.IsNotExist(err) {
		t.Errorf("temp file was not renamed away")
	}
}

func TestValidatePlan_DropsUnknownAndFiltersTags(t *testing.T) {
	req := Request{
		Targets:     []TargetView{{Name: "build"}},
		AllowedTags: []string{"build", "ci"},
	}
	var rp rawPlan
	rp.Annotations = append(rp.Annotations, struct {
		Name string   `json:"name"`
		Desc string   `json:"desc"`
		Tags []string `json:"tags"`
	}{Name: "build", Desc: "Build it", Tags: []string{"build", "bogus"}})
	rp.Annotations = append(rp.Annotations, struct {
		Name string   `json:"name"`
		Desc string   `json:"desc"`
		Tags []string `json:"tags"`
	}{Name: "ghost", Desc: "nope", Tags: nil})

	plan := validatePlan(rp, req)
	if len(plan.Annotations) != 1 || plan.Annotations[0].Name != "build" {
		t.Fatalf("annotations = %#v", plan.Annotations)
	}
	if strings.Join(plan.Annotations[0].Tags, ",") != "build" {
		t.Errorf("tags not filtered to allowed: %v", plan.Annotations[0].Tags)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].Name != "ghost" {
		t.Errorf("unknown target not demoted to skipped: %#v", plan.Skipped)
	}
}

func TestRenderDiff_PlainNoColor(t *testing.T) {
	lines := strings.Split(strings.TrimRight(sampleMakefile, "\n"), "\n")
	_ = lines
	plan := Plan{Annotations: []Annotation{{Name: "build", Desc: "Compile", Tags: []string{"build"}}}}
	diff := RenderDiff(plan, []byte(sampleMakefile), nil)
	if !strings.Contains(diff, "+## build: Compile [tags=build]") {
		t.Errorf("diff missing added doc-line:\n%s", diff)
	}
	if strings.Contains(diff, "\x1b[") {
		t.Errorf("plain diff should carry no ANSI escapes:\n%q", diff)
	}
}
