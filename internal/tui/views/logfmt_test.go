package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Note: testPalette() lives in ai_popup_test.go and is shared across the
// package's tests. Its tokens are all the same color, which is fine here — the
// assertions only check that escapes were injected and the visible text is
// preserved, not specific hues.

func TestColorizeLogLineANSIPassthrough(t *testing.T) {
	p := testPalette()
	in := "\x1b[31mERROR already colored\x1b[0m"
	if got := colorizeLogLine(p, in); got != in {
		t.Fatalf("ANSI line should pass through untouched:\n got %q\nwant %q", got, in)
	}
}

func TestAbbrevLevel(t *testing.T) {
	cases := map[string]string{
		"trace": "TRAC", "TRACE": "TRAC",
		"debug": "DEBU", "Debug": "DEBU",
		"info": "INFO", "INFO": "INFO",
		"warn": "WARN", "warning": "WARN",
		"error": "ERRO", "err": "ERRO",
		"fatal": "FATA",
	}
	for in, want := range cases {
		if got := abbrevLevel(in); got != want {
			t.Errorf("abbrevLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestColorizeLogLineLevelTag(t *testing.T) {
	p := testPalette()
	for _, tc := range []struct {
		line string
		tag  string
	}{
		{"INFO server started", "INFO"},
		{"12:34:56 warn disk almost full", "WARN"},
		{"[error] connection refused", "ERRO"},
		{"2026-01-01T00:00:00Z fatal boom", "FATA"},
		{"debug payload received", "DEBU"},
	} {
		got := colorizeLogLine(p, tc.line)
		plain := ansi.Strip(got)
		if !strings.Contains(plain, tc.tag) {
			t.Errorf("line %q: visible output %q missing tag %q", tc.line, plain, tc.tag)
		}
		if !strings.Contains(got, "\x1b[") {
			t.Errorf("line %q: expected colored output, got no escapes", tc.line)
		}
	}
}

func TestRichColorLineColorsTokensAndPreservesText(t *testing.T) {
	p := testPalette()
	in := `connecting to https://api.example.com retries=3 user="alice" path=/var/log/app.log ok=true`
	got := richColorLine(p, in)

	if stripped := ansi.Strip(got); stripped != in {
		t.Fatalf("visible text changed by coloring:\n got %q\nwant %q", stripped, in)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected color escapes in rich output, got none")
	}
}

func TestRichColorLinePlainProse(t *testing.T) {
	p := testPalette()
	in := "just some plain words here"
	got := richColorLine(p, in)
	if stripped := ansi.Strip(got); stripped != in {
		t.Fatalf("prose mangled: got %q want %q", stripped, in)
	}
}
