package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// ExpandedOutput renders a scrollable popup box showing the full command output.
// popupW and popupH are the desired outer dimensions (including border).
// offset is the index of the first visible line.
// The caller should composite the result over the background via OverlayCenter.
func ExpandedOutput(p Palette, lines []string, offset, popupW, popupH int, cmd string) string {
	// innerW: width of each content row (border:2 + padding:2 = 4 overhead)
	innerW := popupW - 4
	if innerW < 20 {
		innerW = 20
	}

	// visH: number of scrollable content rows
	// overhead: title(1) + sep(1) + sep(1) + hint(1) = 4, plus border rows(2) = 6 total
	visH := popupH - 6
	if visH < 1 {
		visH = 1
	}

	total := len(lines)
	end := offset + visH
	if end > total {
		end = total
	}

	// ── Title row ────────────────────────────────────────────────────────────
	titleText := Style(p.Fg, true).Render("OUTPUT")
	if cmd != "" {
		titleText += Style(p.FgDim, false).Render("  —  ") + Style(p.Accent, true).Render(cmd)
	}
	var scrollText string
	if total > 0 {
		scrollText = Style(p.FgMuted, false).Render(fmt.Sprintf("%d–%d / %d", offset+1, end, total))
	} else {
		scrollText = Style(p.FgMuted, false).Render("empty")
	}
	gap := innerW - VisWidth(titleText) - VisWidth(scrollText)
	if gap < 1 {
		gap = 1
	}
	titleRow := lipgloss.NewStyle().Width(innerW).Background(p.BgPanel).
		Render(titleText + strings.Repeat(" ", gap) + scrollText)

	sep := SepLine(p, innerW)

	// ── Content rows ─────────────────────────────────────────────────────────
	rows := make([]string, visH)
	emptyRow := lipgloss.NewStyle().Width(innerW).Background(p.BgDeep).Render("")
	for i := range rows {
		rows[i] = emptyRow
	}
	for i := 0; i < visH && offset+i < total; i++ {
		colored := ColorOutputLine(p, lines[offset+i])
		line := ansi.Truncate(colored, innerW, "")
		rows[i] = lipgloss.NewStyle().Width(innerW).Background(p.BgDeep).Render(line)
	}

	// ── Hint row ─────────────────────────────────────────────────────────────
	hintText := "↑↓ / j k    pgup pgdn    g G top/end    ctrl+e  esc  close"
	hintText = ansi.Truncate(hintText, innerW, "")
	hintRow := lipgloss.NewStyle().Width(innerW).Background(p.BgPanel).
		Render(Style(p.FgMuted, false).Render(hintText))

	inner := titleRow + "\n" + sep + "\n" +
		strings.Join(rows, "\n") + "\n" +
		sep + "\n" + hintRow

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Accent).
		Padding(0, 1).
		Render(inner)
}
