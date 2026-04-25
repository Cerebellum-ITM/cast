package views

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Modal renders the confirmation dialog box. The caller is responsible for
// compositing it over the background UI via OverlayCenter.
//
// selected is the keyboard-focused button: 0 = cancel, 1 = confirm. The
// focused button is highlighted with a thicker accent so arrow-key
// navigation is visible. Enter on the modal commits whichever button is
// focused; y/n shortcuts still work as direct hotkeys (handled in the model).
func Modal(p Palette, cmdName string, env string, selected int) string {
	title := Style(p.Red, true).Render("⚠  Confirm run")

	body := Style(p.FgDim, false).Render("You're about to run") + "\n" +
		Style(p.Accent, true).Render("make "+cmdName) +
		Style(p.FgDim, false).Render(" against "+env+".") + "\n" +
		Style(p.FgMuted, false).Render("This cannot be undone.")

	cancelFocused := selected == 0
	confirmFocused := selected == 1

	cancelBtn := renderModalBtn(p, "cancel", p.FgMuted, p.BgSelected, cancelFocused)
	confirmBtn := renderModalBtn(p, "⏎ "+cmdName, p.BgDeep, p.Red, confirmFocused)

	buttons := cancelBtn + "  " + confirmBtn
	hint := Style(p.FgMuted, false).
		Render("[←/→] move    [⏎] commit    [y] yes    [n / esc] no")

	inner := "\n" + title + "\n\n" + body + "\n\n" + buttons + "\n\n" + hint + "\n"

	return lipgloss.NewStyle().
		Background(p.BgPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Red).
		Padding(1, 3).
		Render(inner)
}

// renderModalBtn draws one button as a single row. Focus is signalled by a
// brighter foreground + bold + a leading ▸ cursor; unfocused buttons keep
// the same bounding box (a leading space) so horizontal composition with `+`
// stays aligned. Avoiding any kind of border keeps each button exactly one
// row tall — borders would add rows and break the modal layout.
func renderModalBtn(_ Palette, label string, fg, bg color.Color, focused bool) string {
	cursor := "  "
	if focused {
		cursor = "▸ "
	}
	style := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(focused).
		Padding(0, 1)
	return style.Render(cursor + label)
}
