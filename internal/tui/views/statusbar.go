package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/version"
)

// NoticeKind classifies a transient toast surfaced in the header pill area
// (renderNoticePill in tui/render.go). It maps to a glyph and foreground
// colour so the user can scan severity at a glance without reading the
// message. The type lives in views so model and the header renderer share
// the same vocabulary without dragging in a tui→views→tui cycle.
type NoticeKind int

const (
	NoticeInfo NoticeKind = iota
	NoticeSuccess
	NoticeError
)

// StatusBar renders the 1-row bottom status bar: command count on the left,
// working directory + source filename on the right.
func StatusBar(p Palette, cmdCount int, sourcePath string, width int) string {
	left := fmt.Sprintf("⬡ cast v%s  %d commands", version.Current, cmdCount)
	right := statusBarRight(sourcePath)

	usedW := VisWidth(left) + VisWidth(right)
	gap := width - usedW - 2 // -2 for Padding(0,1) left+right
	if gap < 0 {
		gap = 0
	}

	return lipgloss.NewStyle().
		Background(p.Accent).
		Foreground(p.BgDeep).
		Bold(true).
		Width(width).
		Padding(0, 1).
		Render(left + strings.Repeat(" ", gap) + right)
}

// statusBarRight builds the "cwd  source" fragment. Absolute paths are
// rewritten with `~` when they fall under $HOME so the bar stays compact on
// typical setups.
func statusBarRight(sourcePath string) string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "?"
	} else if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(cwd, home) {
		cwd = "~" + strings.TrimPrefix(cwd, home)
	}
	source := filepath.Base(sourcePath)
	if source == "" || source == "." {
		source = "Makefile"
	}
	return cwd + "  " + source
}
