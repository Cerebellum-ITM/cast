package views

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// LibrarySnippet is the view-side projection of a saved snippet. Views
// stay decoupled from internal/library so the package import graph keeps
// flowing one way (views → nothing besides db/source).
type LibrarySnippet struct {
	Name string
	Desc string
	Tags []string
	Body string // verbatim Makefile content for the preview pane
}

// LibraryProps drives the Library tab. The pane is laid out as two
// columns: a left list (~40%) and a right preview (~60%).
type LibraryProps struct {
	Snippets       []LibrarySnippet
	Selected       int
	Search         string
	SearchFocused  bool
	Error          string // sticky red banner when the last action failed
	Feedback       string // sticky green banner on success
	ConfirmDelete  bool   // shows "press d again to delete" affordance
	IconStyle      IconStyle
	Width          int
	Height         int
}

// Library renders the snippet library tab.
func Library(p Palette, props LibraryProps) string {
	w, h := props.Width, props.Height
	if w < 30 || h < 6 {
		return Style(p.FgDim, false).Render("library: panel too small")
	}

	// Header row: title + saved-path hint on top.
	icons := Icons(props.IconStyle)
	title := Style(p.Accent, true).Render(icons.Snippet+"  Snippets") + " " +
		Style(p.FgDim, false).
			Render("· global library, press e on a command to capture · enter to insert")

	leftW := w * 4 / 10
	if leftW < 24 {
		leftW = 24
	}
	rightW := w - leftW - 1
	bodyH := h - 4 // title + spacer + banner row + hint
	if bodyH < 3 {
		bodyH = 3
	}

	left := renderLibraryList(p, props, leftW, bodyH)
	right := renderLibraryPreview(p, props, rightW, bodyH)

	divCol := Style(p.Border, false).Render(strings.Repeat("│\n", bodyH-1) + "│")
	twoCol := lipgloss.JoinHorizontal(lipgloss.Top, left, divCol, right)

	banner := renderLibraryBanner(p, props, w)
	hint := Style(p.FgMuted, false).Render(libraryHint(props))

	return title + "\n" + banner + "\n" + twoCol + "\n" + hint
}

func libraryHint(props LibraryProps) string {
	if props.ConfirmDelete {
		return "press d again or [⏎] to confirm delete · [esc] cancel"
	}
	return "[↑/↓] nav   [/] search   [⏎] insert   [d] delete   [esc] back"
}

// renderLibraryBanner shows a one-row error or success message above the
// two-column body. Empty string when nothing to report.
func renderLibraryBanner(p Palette, props LibraryProps, w int) string {
	switch {
	case props.Error != "":
		return Style(p.Red, true).Render("⚠ " + Truncate(props.Error, w-4))
	case props.Feedback != "":
		return Style(p.Green, true).Render("✓ " + Truncate(props.Feedback, w-4))
	}
	return " "
}

func renderLibraryList(p Palette, props LibraryProps, w, h int) string {
	icons := Icons(props.IconStyle)
	rows := []string{renderLibrarySearchRow(p, props.Search, props.SearchFocused, w)}
	rows = append(rows, SepLine(p, w))

	if len(props.Snippets) == 0 {
		rows = append(rows,
			Style(p.FgMuted, false).Render("  no snippets yet — press e on any command in the commands tab"))
		return padRows(rows, w, h)
	}

	// Window the list around Selected so the cursor stays visible.
	maxRows := h - len(rows)
	if maxRows < 1 {
		maxRows = 1
	}
	start := 0
	if props.Selected >= maxRows {
		start = props.Selected - maxRows + 1
	}
	end := start + maxRows
	if end > len(props.Snippets) {
		end = len(props.Snippets)
	}

	for i := start; i < end; i++ {
		s := props.Snippets[i]
		row := renderLibraryRow(p, icons, s, i == props.Selected, w)
		rows = append(rows, row)
	}
	return padRows(rows, w, h)
}

// renderLibrarySearchRow mirrors the sidebar's search row but is local to
// the library tab so its focused-state indicator doesn't compete with the
// global search.
func renderLibrarySearchRow(p Palette, q string, focused bool, w int) string {
	icon := Style(p.FgDim, false).Render("⌕ ")
	avail := w - VisWidth(icon) - 2
	var inputStr string
	switch {
	case q == "" && !focused:
		inputStr = Style(p.FgDim, false).Render("search snippets…")
	default:
		inputStr = Style(p.Fg, false).Render(Truncate(q, avail))
		if focused {
			inputStr += Style(p.Accent, false).Render("▌")
		}
	}
	return lipgloss.NewStyle().Width(w).Padding(0, 1).Render(icon + inputStr)
}

func renderLibraryRow(p Palette, icons IconSet, s LibrarySnippet, focused bool, w int) string {
	cursor := "  "
	nameFg := p.Fg
	descFg := p.FgDim
	if focused {
		cursor = Style(p.Accent, true).Render("▸ ")
		nameFg = p.Accent
	}
	icon := icons.Snippet
	if icon == "" {
		icon = icons.Folder
	}
	contentW := w - 2
	nameStr := lipgloss.NewStyle().Foreground(nameFg).Bold(focused).Render(s.Name)

	row1 := cursor + Style(p.Yellow, false).Render(icon) + " " + nameStr
	desc := s.Desc
	if desc == "" {
		desc = "(no description)"
	}
	row2 := "    " + lipgloss.NewStyle().Foreground(descFg).
		Render(Truncate(desc, contentW-4))

	if focused {
		bar := lipgloss.NewStyle().Foreground(p.Accent).Bold(true).Render("▎")
		return bar + row1 + "\n" + bar + row2
	}
	return " " + row1 + "\n" + " " + row2
}

func renderLibraryPreview(p Palette, props LibraryProps, w, h int) string {
	rows := []string{Style(p.FgDim, false).Render("preview")}
	rows = append(rows, SepLine(p, w))

	if len(props.Snippets) == 0 || props.Selected >= len(props.Snippets) {
		rows = append(rows, Style(p.FgMuted, false).Render("  (nothing to show)"))
		return padRows(rows, w, h)
	}
	body := props.Snippets[props.Selected].Body
	avail := h - len(rows)
	for i, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if i >= avail {
			rows = append(rows, Style(p.FgMuted, false).Render("  …"))
			break
		}
		// Truncate visually but keep makefile-aware highlighting.
		raw := Truncate(line, w-3)
		styled := HighlightMakefileLine(p, raw)
		rows = append(rows, "  "+styled)
	}
	return padRows(rows, w, h)
}

// padRows tops up rows with blanks so the panel always renders exactly h
// rows and JoinHorizontal aligns cleanly with the divider column.
func padRows(rows []string, w, h int) string {
	for len(rows) < h {
		rows = append(rows, lipgloss.NewStyle().Width(w).Render(""))
	}
	if len(rows) > h {
		rows = rows[:h]
	}
	return strings.Join(rows, "\n")
}
