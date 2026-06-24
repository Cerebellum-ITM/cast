package source

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestWriteDocLine_InsertAboveBareTarget(t *testing.T) {
	lines := []string{
		"build:",
		"\t@go build ./...",
		"",
		"test:",
		"\t@go test ./...",
	}
	got, err := WriteDocLine(lines, "build", "Compile the binary", []string{"build", "go"})
	if err != nil {
		t.Fatalf("WriteDocLine: %v", err)
	}
	want := []string{
		"## build: Compile the binary [tags=build,go]",
		"build:",
		"\t@go build ./...",
		"",
		"test:",
		"\t@go test ./...",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("insert mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestWriteDocLine_ReplaceExistingPreservesFlags(t *testing.T) {
	lines := []string{
		"## build: old desc [sc=b] [stream]",
		"build:",
		"\t@go build ./...",
	}
	got, err := WriteDocLine(lines, "build", "New description", []string{"build"})
	if err != nil {
		t.Fatalf("WriteDocLine: %v", err)
	}
	// The existing [sc=b] and [stream] flags must survive; only desc + tags change.
	doc := got[0]
	for _, must := range []string{"## build: New description", "[sc=b]", "[tags=build]", "[stream]"} {
		if !strings.Contains(doc, must) {
			t.Errorf("doc-line %q missing %q", doc, must)
		}
	}
	if strings.Contains(doc, "old desc") {
		t.Errorf("doc-line still carries old description: %q", doc)
	}
	if len(got) != 3 {
		t.Fatalf("replace should not change line count: got %d lines", len(got))
	}
}

func TestWriteDocLine_EmptyTagsDropsMarker(t *testing.T) {
	lines := []string{
		"## build: desc [tags=build,old]",
		"build:",
		"\t@go build ./...",
	}
	got, err := WriteDocLine(lines, "build", "desc", nil)
	if err != nil {
		t.Fatalf("WriteDocLine: %v", err)
	}
	if strings.Contains(got[0], "[tags=") {
		t.Errorf("expected [tags=…] removed, got %q", got[0])
	}
}

func TestWriteDocLine_TargetNotFound(t *testing.T) {
	lines := []string{"build:", "\t@go build ./..."}
	_, err := WriteDocLine(lines, "deploy", "desc", nil)
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("expected ErrTargetNotFound, got %v", err)
	}
}

func TestWriteDocLine_DoesNotTouchOtherTargets(t *testing.T) {
	lines := []string{
		"## test: run tests",
		"test:",
		"\t@go test ./...",
		"",
		"build:",
		"\t@go build ./...",
	}
	got, err := WriteDocLine(lines, "build", "Compile", []string{"build"})
	if err != nil {
		t.Fatalf("WriteDocLine: %v", err)
	}
	// Everything above the build target must be byte-for-byte identical.
	for i := 0; i < 4; i++ {
		if got[i] != lines[i] {
			t.Errorf("line %d changed: got %q want %q", i, got[i], lines[i])
		}
	}
}
