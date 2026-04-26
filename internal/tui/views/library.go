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
	// Height budget: 1 title + 1 banner + N twoCol + 1 hint = h. Solve for
	// twoCol height. Floor at 3 so the list/preview don't collapse into
	// nothing when the terminal is very short.
	bodyH := h - 3
	if bodyH < 3 {
		bodyH = 3
	}

	left := renderLibraryList(p, props, leftW, bodyH)
	right := renderLibraryPreview(p, props, rightW, bodyH)

	divCol := Style(p.Border, false).Render(strings.Repeat("│\n", bodyH-1) + "│")
	twoCol := lipgloss.JoinHorizontal(lipgloss.Top, left, divCol, right)

	banner := renderLibraryBanner(p, props, w)
	hint := renderLibraryHints(p, props, w)

	return title + "\n" + banner + "\n" + twoCol + "\n" + hint
}

// renderLibraryHints mirrors the sidebar's renderHintsRow visual contract
// so every tab shows its keys the same way: accent-coloured `[key]`
// brackets followed by a dim short label, packed onto one row capped at
// the panel's width. This is the only "where do I press what" affordance
// inside the library tab — making it match the rest of the UI is the
// difference between visible and forgotten.
func renderLibraryHints(p Palette, props LibraryProps, w int) string {
	var hints [][2]string
	if props.ConfirmDelete {
		hints = [][2]string{
			{"d", "confirm"},
			{"⏎", "confirm"},
			{"esc", "cancel"},
		}
	} else {
		hints = [][2]string{
			{"↑↓", "nav"},
			{"⏎", "insert"},
			{"/", "search"},
			{"d", "delete"},
			{"esc", "back"},
		}
	}
	avail := w - 2
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1)

	var parts []string
	used := 0
	for _, h := range hints {
		key := Style(p.Accent, true).Render("[" + h[0] + "]")
		lbl := Style(p.FgDim, false).Render(h[1])
		part := key + " " + lbl
		partW := VisWidth(part)
		gap := 0
		if len(parts) > 0 {
			gap = 1
		}
		if used+gap+partW > avail {
			break
		}
		parts = append(parts, part)
		used += gap + partW
	}
	return rowStyle.Render(strings.Join(parts, " "))
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
	// We accumulate one terminal row per slice element so padRows can pad
	// to exactly h lines without the count getting out of sync with the
	// rendered output. Cards contribute two elements (row1 + row2)
	// instead of a single multi-line string.
	lines := []string{renderLibrarySearchRow(p, props.Search, props.SearchFocused, w)}
	lines = append(lines, SepLine(p, w))

	if len(props.Snippets) == 0 {
		lines = append(lines,
			Style(p.FgMuted, false).Render("  no snippets yet — press e on any command in the commands tab"))
		return padRows(lines, w, h)
	}

	const rowsPerItem = 2
	avail := h - len(lines)
	if avail < rowsPerItem {
		avail = rowsPerItem
	}
	maxItems := avail / rowsPerItem
	if maxItems < 1 {
		maxItems = 1
	}
	start := 0
	if props.Selected >= maxItems {
		start = props.Selected - maxItems + 1
	}
	end := start + maxItems
	if end > len(props.Snippets) {
		end = len(props.Snippets)
	}

	for i := start; i < end; i++ {
		s := props.Snippets[i]
		row1, row2 := renderLibraryRow(p, icons, s, i == props.Selected, w)
		lines = append(lines, row1, row2)
	}
	return padRows(lines, w, h)
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

// renderLibraryRow returns the two visual rows that make up one snippet
// card: row1 holds the icon + name, row2 the dim description. Returning
// them separately (instead of a `row1 + "\n" + row2` string) lets the
// caller append each as its own slice element and keep padRows' element
// count aligned with the actual line count.
func renderLibraryRow(p Palette, icons IconSet, s LibrarySnippet, focused bool, w int) (string, string) {
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
		return bar + row1, bar + row2
	}
	return " " + row1, " " + row2
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
