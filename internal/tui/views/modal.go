package views

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Modal renders a centered production-confirmation dialog overlaid on the TUI.
func Modal(p Palette, cmdName string, width, height int) string {
	title := Style(p.Red, true).Render("⚠  Production Run")
	msg := Style(p.FgDim, false).Render("Execute ") +
		Style(p.Accent, true).Render(cmdName) +
		Style(p.FgDim, false).Render(" on prod?")
	hint := Style(p.FgMuted, false).Render("[y / enter] confirm    [n / esc] cancel")

	inner := "\n" + title + "\n\n" + msg + "\n\n" + hint + "\n"

	box := lipgloss.NewStyle().
		Background(p.BgPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Red).
		Padding(1, 3).
		Render(inner)

	boxW := VisWidth(box)
	boxH := lipgloss.Height(box)

	leftPad := (width - boxW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	topPad := (height - boxH) / 2
	if topPad < 0 {
		topPad = 0
	}

	var lines []string
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}
	for _, l := range strings.Split(box, "\n") {
		lines = append(lines, strings.Repeat(" ", leftPad)+l)
	}

	return strings.Join(lines, "\n")
}
