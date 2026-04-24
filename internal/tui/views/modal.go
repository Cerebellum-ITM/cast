package views

import (
	"charm.land/lipgloss/v2"
)

// Modal renders the confirmation dialog box. The caller is responsible for
// compositing it over the background UI via OverlayCenter.
func Modal(p Palette, cmdName string, env string) string {
	title := Style(p.Red, true).Render("⚠  Confirm run")

	body := Style(p.FgDim, false).Render("You're about to run") + "\n" +
		Style(p.Accent, true).Render("make "+cmdName) +
		Style(p.FgDim, false).Render(" against "+env+".") + "\n" +
		Style(p.FgMuted, false).Render("This cannot be undone.")

	cancelBtn := lipgloss.NewStyle().
		Background(p.BgSelected).
		Foreground(p.FgDim).
		Padding(0, 1).
		Render("cancel")

	confirmBtn := lipgloss.NewStyle().
		Background(p.Red).
		Foreground(p.BgDeep).
		Bold(true).
		Padding(0, 1).
		Render("⏎ " + cmdName)

	buttons := cancelBtn + "  " + confirmBtn
	hint := Style(p.FgMuted, false).Render("[y / enter] confirm    [n / esc] cancel")

	inner := "\n" + title + "\n\n" + body + "\n\n" + buttons + "\n\n" + hint + "\n"

	return lipgloss.NewStyle().
		Background(p.BgPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Red).
		Padding(1, 3).
		Render(inner)
}
