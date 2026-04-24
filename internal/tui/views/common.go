package views

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/db"
)

// Palette holds all resolved color tokens for a theme + environment combination.
type Palette struct {
	Bg, BgPanel, BgDeep, BgSelected, BgHover color.Color
	Fg, FgDim, FgMuted                        color.Color
	Border, Accent                             color.Color
	Cyan, Green, Yellow, Orange, Red           color.Color
}

// Style creates a simple foreground lipgloss style.
func Style(c color.Color, bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(c)
	if bold {
		s = s.Bold(true)
	}
	return s
}

// SepLine returns a full-width separator using ─ characters.
func SepLine(p Palette, w int) string {
	return Style(p.Border, false).Render(strings.Repeat("─", w))
}

// Pad returns n spaces for manual indentation.
func Pad(n int) string { return strings.Repeat(" ", n) }

// VisWidth returns the visible column width of a (potentially styled) string.
func VisWidth(s string) int { return lipgloss.Width(s) }

// Truncate clips s to max visual characters, appending … if needed.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

// RenderKeyBadge renders a keyboard shortcut badge.
func RenderKeyBadge(p Palette, key string) string {
	if key == "" {
		key = " "
	}
	return lipgloss.NewStyle().
		Foreground(p.BgDeep).Bold(true).
		Background(p.Accent).
		Padding(0, 1).
		Render(key)
}

// RenderTagChip renders a tag with its semantic color on a dark background.
func RenderTagChip(p Palette, text string) string {
	return lipgloss.NewStyle().
		Foreground(TagColor(p, text)).
		Background(p.BgSelected).
		Padding(0, 1).
		Render(text)
}

// RenderInlineTag renders a tag inline (no extra padding).
func RenderInlineTag(p Palette, text string) string {
	return lipgloss.NewStyle().
		Foreground(TagColor(p, text)).
		Background(p.BgSelected).
		Padding(0, 1).
		Render(text)
}

// TagColor maps a tag string to a palette color.
func TagColor(p Palette, tag string) color.Color {
	switch strings.ToLower(tag) {
	case "go":
		return p.Cyan
	case "ci", "golangci":
		return p.Orange
	case "prod", "production":
		return p.Red
	case "staging":
		return p.Orange
	case "local":
		return p.Green
	default:
		return p.Accent
	}
}

// RenderProgressBar draws a 1-row block-fill progress bar using fillColor for filled blocks.
func RenderProgressBar(p Palette, w int, progress float64, fillColor color.Color) string {
	if w < 2 {
		return ""
	}
	filled := int(float64(w) * progress)
	if filled > w {
		filled = w
	}
	return lipgloss.NewStyle().Foreground(fillColor).Render(strings.Repeat("▓", filled)) +
		lipgloss.NewStyle().Foreground(p.FgMuted).Render(strings.Repeat("░", w-filled))
}

// StatusDot returns a colored ● indicator for a run status.
func StatusDot(p Palette, status db.RunStatus) string {
	switch status {
	case db.StatusSuccess:
		return Style(p.Green, false).Render("●")
	case db.StatusError:
		return Style(p.Red, false).Render("●")
	case db.StatusRunning:
		return Style(p.Yellow, false).Render("●")
	default:
		return Style(p.FgDim, false).Render("●")
	}
}

// ColorOutputLine applies syntax color to a terminal output line.
func ColorOutputLine(p Palette, line string) string {
	switch {
	case strings.HasPrefix(line, "✓"):
		return Style(p.Green, false).Render(line)
	case strings.HasPrefix(line, "✗"):
		return Style(p.Red, false).Render(line)
	case strings.HasPrefix(line, "$"):
		return Style(p.Cyan, false).Render(line)
	case strings.HasPrefix(line, "--- PASS"):
		return Style(p.Green, false).Render(line)
	case strings.HasPrefix(line, "--- FAIL"):
		return Style(p.Red, false).Render(line)
	default:
		return Style(p.FgDim, false).Render(line)
	}
}

// HighlightMakefileLine applies syntax highlighting to a single Makefile line.
func HighlightMakefileLine(p Palette, line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "##"), strings.HasPrefix(trimmed, "#"),
		strings.HasPrefix(trimmed, ".PHONY"):
		return Style(p.FgDim, false).Render(line)
	case strings.Contains(line, " = ") || strings.Contains(line, " := "):
		idx := strings.Index(line, "=")
		if idx > 0 {
			return Style(p.Cyan, false).Render(line[:idx]) +
				Style(p.FgDim, false).Render("=") +
				Style(p.Yellow, false).Render(line[idx+1:])
		}
	case !strings.HasPrefix(line, "\t") && strings.HasSuffix(trimmed, ":"):
		return Style(p.Accent, false).Render(line)
	case strings.HasPrefix(line, "\t@echo"):
		return Style(p.FgDim, false).Render("\t@") +
			Style(p.Green, false).Render(strings.TrimPrefix(line, "\t@"))
	case strings.HasPrefix(line, "\t@"):
		return Style(p.FgDim, false).Render("\t@") +
			Style(p.Fg, false).Render(strings.TrimPrefix(line, "\t@"))
	}
	return Style(p.FgDim, false).Render(line)
}
