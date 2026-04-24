package views

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// OverlayCenter composites fg centered over bg, painting fg's lines directly
// into the corresponding rows of bg. Both strings may contain ANSI escape codes;
// the compositor uses ansi-aware truncation to avoid color bleeding.
func OverlayCenter(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	bgH := len(bgLines)
	bgW := 0
	for _, l := range bgLines {
		if w := lipgloss.Width(l); w > bgW {
			bgW = w
		}
	}

	fgH := len(fgLines)
	fgW := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}

	startX := (bgW - fgW) / 2
	startY := (bgH - fgH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	for i, fgLine := range fgLines {
		bgRow := startY + i
		if bgRow < 0 || bgRow >= len(bgLines) {
			continue
		}
		bgLine := bgLines[bgRow]

		// Pad the bg line to at least startX so Truncate doesn't under-cut.
		if bw := lipgloss.Width(bgLine); bw < startX {
			bgLine += strings.Repeat(" ", startX-bw)
		}

		left := ansi.Truncate(bgLine, startX, "")
		right := ansi.TruncateLeft(bgLine, startX+fgW, "")
		bgLines[bgRow] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}
