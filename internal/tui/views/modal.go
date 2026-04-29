package views

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// ConfirmModalProps describes the content of a generic two-button confirm
// dialog. Body is rendered verbatim (the caller styles it) so each modal
// can compose its own coloring.
type ConfirmModalProps struct {
	Title        string
	Body         string
	ConfirmLabel string
	CancelLabel  string
	// Selected is the keyboard-focused button: 0 = cancel, 1 = confirm.
	Selected int
	// Accent drives both the border color and the confirm button background
	// so destructive vs. neutral confirms can be distinguished at a glance.
	Accent color.Color
}

// ConfirmModal renders a centered two-button confirmation dialog. The
// caller is responsible for compositing it over the background UI via
// OverlayCenter. Used by Modal (run-confirm) and the delete-command flow.
func ConfirmModal(p Palette, props ConfirmModalProps) string {
	cancelLabel := props.CancelLabel
	if cancelLabel == "" {
		cancelLabel = "cancel"
	}
	confirmLabel := props.ConfirmLabel
	if confirmLabel == "" {
		confirmLabel = "confirm"
	}
	accent := props.Accent
	if accent == nil {
		accent = p.Red
	}

	title := Style(accent, true).Render(props.Title)

	cancelFocused := props.Selected == 0
	confirmFocused := props.Selected == 1

	cancelBtn := renderModalBtn(p, cancelLabel, p.FgMuted, p.BgSelected, cancelFocused)
	confirmBtn := renderModalBtn(p, confirmLabel, p.BgDeep, accent, confirmFocused)

	buttons := cancelBtn + "  " + confirmBtn
	hintRendered := renderModalHints(p, [][2]string{
		{"⏎ / y", "yes"},
		{"esc / n", "no"},
		{"←/→", "move"},
	})

	inner := "\n" + title + "\n\n" + props.Body + "\n\n" + buttons + "\n\n" + hintRendered + "\n"

	return lipgloss.NewStyle().
		Background(p.BgPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 3).
		Render(inner)
}

// Modal renders the run-confirm dialog. Thin wrapper over ConfirmModal that
// fills in the run-specific copy and accent.
//
// selected is the keyboard-focused button: 0 = cancel, 1 = confirm. The
// focused button is highlighted with a thicker accent so arrow-key
// navigation is visible. Enter on the modal commits whichever button is
// focused; y/n shortcuts still work as direct hotkeys (handled in the model).
func Modal(p Palette, cmdName string, env string, selected int) string {
	body := Style(p.FgDim, false).Render("You're about to run") + "\n" +
		Style(p.Accent, true).Render("make "+cmdName) +
		Style(p.FgDim, false).Render(" against "+env+".") + "\n" +
		Style(p.FgMuted, false).Render("This cannot be undone.")

	return ConfirmModal(p, ConfirmModalProps{
		Title:        "⚠  Confirm run",
		Body:         body,
		ConfirmLabel: "⏎ " + cmdName,
		Selected:     selected,
		Accent:       p.Red,
	})
}

// DeleteCommandModal renders the destructive confirm dialog shown before
// removing a Makefile target from disk via the commands tab.
func DeleteCommandModal(p Palette, cmdName string, selected int) string {
	body := Style(p.FgDim, false).Render("You're about to delete") + "\n" +
		Style(p.Red, true).Render("make "+cmdName) +
		Style(p.FgDim, false).Render(" from the Makefile.") + "\n" +
		Style(p.FgMuted, false).Render("This rewrites the file on disk and cannot be undone.")

	return ConfirmModal(p, ConfirmModalProps{
		Title:        "⚠  Delete command",
		Body:         body,
		ConfirmLabel: "⏎ delete " + cmdName,
		Selected:     selected,
		Accent:       p.Red,
	})
}

// renderModalHints renders key/label pairs with the same accented bracket
// style used by the sidebar hints row, so the confirm modal's footer
// matches the rest of the TUI.
func renderModalHints(p Palette, pairs [][2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, h := range pairs {
		key := Style(p.Accent, true).Render("[" + h[0] + "]")
		lbl := Style(p.FgDim, false).Render(h[1])
		parts = append(parts, key+" "+lbl)
	}
	return strings.Join(parts, "    ")
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
