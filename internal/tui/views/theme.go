package views

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// ThemeOption describes one theme choice rendered in the Theme tab.
type ThemeOption struct {
	Key      string  // canonical identifier, e.g. "catppuccin"
	Label    string  // display name, e.g. "Catppuccin"
	Preview  Palette // resolved palette used to draw the swatch
	IsActive bool    // currently applied theme
	Saved    bool    // persisted in the local .cast.toml
}

// ThemeProps drives the Theme tab. Selected is the cursor row; cursor + Enter
// commits and persists the choice. Width/Height match the center-panel area.
type ThemeProps struct {
	Options    []ThemeOption
	Selected   int
	LocalPath  string // shown so the user knows where the value gets saved
	WriteError string // last error from saving, or "" if none
	Width      int
	Height     int
}

// Theme renders the theme picker pane.
func Theme(p Palette, props ThemeProps) string {
	w, h := props.Width, props.Height
	title := Style(p.Accent, true).Render("Themes")
	sub := Style(p.FgDim, false).
		Render("pick a theme · saved to ") +
		Style(p.Cyan, false).Render(props.LocalPath)

	var rows []string
	rows = append(rows, title)
	rows = append(rows, sub)
	rows = append(rows, "")

	for i, opt := range props.Options {
		rows = append(rows, renderThemeRow(p, opt, i == props.Selected, w-2))
		rows = append(rows, "")
	}

	if props.WriteError != "" {
		rows = append(rows, Style(p.Red, true).Render("  ⚠ "+props.WriteError))
	}

	hint := Style(p.FgMuted, false).
		Render("[↑/↓] move    [⏎] apply & save    [esc] close tab")
	rows = append(rows, "")
	rows = append(rows, hint)

	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).
		Background(p.BgPanel).Padding(1, 2).Render(body)
}

// renderThemeRow draws a single theme card: cursor + label + active marker +
// a swatch row showing the theme's accent / status colors so the user can
// preview without applying.
func renderThemeRow(p Palette, opt ThemeOption, focused bool, w int) string {
	cursor := "  "
	if focused {
		cursor = Style(p.Accent, true).Render("▸ ")
	}

	nameFg := p.Fg
	if focused {
		nameFg = p.Accent
	}
	name := lipgloss.NewStyle().Foreground(nameFg).Bold(focused).Render(opt.Label)

	var marker string
	switch {
	case opt.IsActive && opt.Saved:
		marker = Style(p.Green, true).Render(" · active · saved")
	case opt.IsActive:
		marker = Style(p.Yellow, true).Render(" · active (unsaved)")
	case opt.Saved:
		marker = Style(p.Cyan, false).Render(" · saved")
	}

	row1 := cursor + name + marker

	swatch := renderThemeSwatch(opt.Preview)
	row2 := "    " + swatch

	if focused {
		// Lift the focused row visually with a thick accent left border so it
		// stands out from the inactive options without changing layout width.
		block := lipgloss.NewStyle().Width(w).
			Background(p.BgSelected).
			BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(p.Accent).
			Render(row1 + "\n" + row2)
		return block
	}
	return lipgloss.NewStyle().Width(w).
		BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Border).
		Render(row1 + "\n" + row2)
}

// renderThemeSwatch draws a small horizontal strip of the theme's most
// expressive colors so the user can compare side-by-side without applying.
func renderThemeSwatch(p Palette) string {
	cell := func(c interface{ RGBA() (uint32, uint32, uint32, uint32) }, label string) string {
		return lipgloss.NewStyle().Background(c).Foreground(p.BgDeep).Bold(true).
			Padding(0, 1).Render(label)
	}
	return cell(p.Accent, "AC") + " " +
		cell(p.Cyan, "C") + " " +
		cell(p.Green, "G") + " " +
		cell(p.Yellow, "Y") + " " +
		cell(p.Orange, "O") + " " +
		cell(p.Red, "R")
}
