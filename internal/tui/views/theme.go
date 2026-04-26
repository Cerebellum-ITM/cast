package views

import (
	"image/color"
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
// commits and persists the choice. Width/Height match the body area.
type ThemeProps struct {
	Options    []ThemeOption
	Selected   int
	LocalPath  string // shown so the user knows where the value gets saved
	WriteError string // last error from saving, or "" if none
	Width      int
	Height     int
}

// listColW is the fixed width used by the theme list column. Anything wider
// would waste space — the names are short and the cards stack vertically.
const themeListColW = 38

// Theme renders the theme picker pane in two columns: a compact list on the
// left and a rich live preview on the right driven by the focused option's
// palette so the user sees how the chosen theme will look across the app
// before committing.
func Theme(p Palette, props ThemeProps) string {
	w, h := props.Width, props.Height
	if w < 60 {
		// Fall back to single-column when there isn't room for the preview.
		return themeSingleColumn(p, props)
	}

	listW := themeListColW
	if listW > w/2 {
		listW = w / 2
	}
	previewW := w - listW - 3 // 3 = gap + divider + gap

	list := themeList(p, props, listW, h-4)

	focused := props.Options[clampIdx(props.Selected, len(props.Options))]
	preview := themePreview(focused.Preview, previewW, h-4)

	divStyle := lipgloss.NewStyle().Foreground(p.Border)
	div := divStyle.Render(strings.Repeat("│\n", h-5) + "│")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		list, " ", div, " ", preview)

	header := themeHeader(p, props, w-4)
	hint := Style(p.FgMuted, false).
		Render("[↑/↓] move    [⏎] apply & save    [esc] close tab")

	rows := []string{header, "", body, "", hint}
	return lipgloss.NewStyle().Width(w).Height(h).
		Padding(1, 2).Render(strings.Join(rows, "\n"))
}

// themeHeader is the top strip: title + persisted-path hint + write error.
func themeHeader(p Palette, props ThemeProps, w int) string {
	title := Style(p.Accent, true).Render("Themes")
	sub := Style(p.FgDim, false).Render("saved to ") +
		Style(p.Cyan, false).Render(props.LocalPath)
	line := title + Style(p.FgMuted, false).Render("  ·  ") + sub
	if props.WriteError != "" {
		line += "    " + Style(p.Red, true).Render("⚠ "+props.WriteError)
	}
	_ = w
	return line
}

// themeList renders the left column: one card per theme, focused row gets a
// thick accent border so the cursor reads at a glance.
func themeList(p Palette, props ThemeProps, w, h int) string {
	var rows []string
	for i, opt := range props.Options {
		rows = append(rows, renderThemeRow(p, opt, i == props.Selected, w))
		rows = append(rows, "")
	}
	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Render(body)
}

// themeSingleColumn is the narrow-terminal fallback (no live preview).
func themeSingleColumn(p Palette, props ThemeProps) string {
	w, h := props.Width, props.Height
	rows := []string{
		Style(p.Accent, true).Render("Themes"),
		Style(p.FgDim, false).Render("pick a theme · saved to ") +
			Style(p.Cyan, false).Render(props.LocalPath),
		"",
	}
	for i, opt := range props.Options {
		rows = append(rows, renderThemeRow(p, opt, i == props.Selected, w-2))
		rows = append(rows, "")
	}
	if props.WriteError != "" {
		rows = append(rows, Style(p.Red, true).Render("  ⚠ "+props.WriteError))
	}
	rows = append(rows, "",
		Style(p.FgMuted, false).
			Render("[↑/↓] move    [⏎] apply & save    [esc] close tab"))
	return lipgloss.NewStyle().Width(w).Height(h).
		Padding(1, 2).Render(strings.Join(rows, "\n"))
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
	cell := func(c color.Color, label string) string {
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

// themePreview renders a live mock of the actual UI surfaces using the
// focused theme's palette so the user judges color choices in context: a
// fake sidebar card, a tag chip row, a status-bar strip, a swatch grid
// with the full token list, and a sample status-dot legend.
func themePreview(fp Palette, w, h int) string {
	if w < 20 {
		w = 20
	}
	heading := Style(fp.Accent, true).Render("Live preview") +
		Style(fp.FgMuted, false).Render("  ·  rendered with the focused theme")

	card := mockSidebarCard(fp, w-2)
	chips := mockChipRow(fp)
	bar := mockStatusBar(fp, w-2)
	tokens := tokenGrid(fp, w-2)
	dots := mockStatusDots(fp)

	parts := []string{
		heading,
		"",
		Style(fp.FgDim, false).Render("Sidebar card (focused)"),
		card,
		"",
		Style(fp.FgDim, false).Render("Tag chips"),
		chips,
		"",
		Style(fp.FgDim, false).Render("Status bar"),
		bar,
		"",
		Style(fp.FgDim, false).Render("Status dots") + "    " + dots,
		"",
		Style(fp.FgDim, false).Render("Palette tokens"),
		tokens,
	}
	body := strings.Join(parts, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Render(body)
}

// mockSidebarCard reproduces the focused-row look of the commands sidebar so
// the user can judge the selection background against the foreground colors.
func mockSidebarCard(fp Palette, w int) string {
	if w < 24 {
		w = 24
	}
	badge := lipgloss.NewStyle().
		Foreground(fp.BgDeep).Bold(true).Background(fp.Accent).
		Padding(0, 1).Render("b")
	name := Style(fp.Accent, true).Render("build")
	desc := Style(fp.FgDim, false).Render("  compile the binary")
	tags := RenderInlineTag(fp, "go") + " " + RenderInlineTag(fp, "ci")
	row1 := badge + " " + name + desc
	row2 := "    " + tags
	return lipgloss.NewStyle().Width(w).
		Background(fp.BgSelected).
		BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(fp.Accent).
		Padding(0, 1).
		Render(row1 + "\n" + row2)
}

// mockChipRow shows the four named tag colors so palette differences in the
// chip layer (often the noisiest in dark themes) are obvious.
func mockChipRow(fp Palette) string {
	return RenderInlineTag(fp, "go") + " " +
		RenderInlineTag(fp, "ci") + " " +
		RenderInlineTag(fp, "prod") + " " +
		RenderInlineTag(fp, "local")
}

// mockStatusBar mirrors the real bottom status bar: muted background with
// accent-colored counts and an env pill on the right.
func mockStatusBar(fp Palette, w int) string {
	if w < 30 {
		w = 30
	}
	left := Style(fp.FgDim, false).Render(" 12 commands · ") +
		Style(fp.Accent, true).Render("local")
	right := lipgloss.NewStyle().Background(fp.Accent).Foreground(fp.BgDeep).
		Bold(true).Padding(0, 1).Render("LOCAL")
	pad := w - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if pad < 1 {
		pad = 1
	}
	return lipgloss.NewStyle().Background(fp.BgPanel).Width(w).
		Render(left + strings.Repeat(" ", pad) + right + " ")
}

// mockStatusDots renders the same dots used in the runs table so the user
// can verify status colors are distinguishable at a glance.
func mockStatusDots(fp Palette) string {
	return Style(fp.Green, false).Render("● ok") + "  " +
		Style(fp.Yellow, false).Render("● run") + "  " +
		Style(fp.Orange, false).Render("⏹ stop") + "  " +
		Style(fp.Red, false).Render("● err")
}

// tokenGrid lists every named token in the palette with its label drawn
// on the token's own background — the canonical "show me every color"
// reference for theme authors and users alike.
func tokenGrid(fp Palette, w int) string {
	type tok struct {
		label string
		bg    color.Color
		fg    color.Color
	}
	toks := []tok{
		{"Accent", fp.Accent, fp.BgDeep},
		{"Cyan", fp.Cyan, fp.BgDeep},
		{"Green", fp.Green, fp.BgDeep},
		{"Yellow", fp.Yellow, fp.BgDeep},
		{"Orange", fp.Orange, fp.BgDeep},
		{"Red", fp.Red, fp.BgDeep},
		{"Border", fp.Border, fp.Fg},
		{"BgPanel", fp.BgPanel, fp.Fg},
		{"BgSelected", fp.BgSelected, fp.Fg},
		{"BgHover", fp.BgHover, fp.Fg},
		{"FgDim", fp.FgDim, fp.BgDeep},
		{"FgMuted", fp.FgMuted, fp.BgDeep},
	}
	cells := make([]string, 0, len(toks))
	for _, t := range toks {
		cells = append(cells, lipgloss.NewStyle().
			Background(t.bg).Foreground(t.fg).Bold(true).
			Padding(0, 1).Render(t.label))
	}
	// Wrap into rows so the grid fits the preview width.
	var rows []string
	cur := ""
	for _, c := range cells {
		if cur != "" && lipgloss.Width(cur)+1+lipgloss.Width(c) > w {
			rows = append(rows, cur)
			cur = c
			continue
		}
		if cur == "" {
			cur = c
		} else {
			cur += " " + c
		}
	}
	if cur != "" {
		rows = append(rows, cur)
	}
	return strings.Join(rows, "\n")
}

func clampIdx(i, n int) int {
	if n == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
