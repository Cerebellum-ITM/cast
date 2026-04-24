package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// StatusBar renders the 1-row bottom status bar.
func StatusBar(p Palette, cmdCount int, envStr string, width int) string {
	left := fmt.Sprintf("⬡ cast  %d commands  ● %s", cmdCount, envStr)
	right := "~/projects/myapp  Makefile  v0.1.0"

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
