package views

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// PickerEntry is one folder shown in the picker list.
type PickerEntry struct {
	Name string // bare directory name (basename)
	Icon string // pre-resolved nerd-font glyph (with trailing space)
}

// PickerProps drives the folder-picker modal that runs before commands tagged
// with [pick=…]. The modal is rendered as a centered overlay.
type PickerProps struct {
	CmdName    string        // command waiting for picks
	StepIdx    int           // 0-based index of the current step
	StepCount  int           // total number of pick steps
	BaseDir    string        // resolved directory being listed (with previous picks substituted)
	Filter     string        // active filter pattern (informational)
	Search     string        // current fuzzy-search buffer
	Entries    []PickerEntry // post-filter list
	Cursor     int           // selected index into Entries
	Selections []string      // already-chosen folders (for the breadcrumb)
	IconStyle  IconStyle     // controls the title glyph; entry icons are pre-baked
	Width      int
	Height     int
}

// Picker renders the modal box. Caller composes it via OverlayCenter.
func Picker(p Palette, props PickerProps) string {
	w := props.Width
	if w < 40 {
		w = 40
	}
	h := props.Height
	if h < 10 {
		h = 10
	}

	titleTxt := Icons(props.IconStyle).PickerTitle + "  Select folder"
	step := stepBadge(p, props.StepIdx, props.StepCount)
	title := Style(p.Accent, true).Render(titleTxt) + "  " + step

	crumb := renderBreadcrumb(p, props.CmdName, props.Selections)
	baseLine := Style(p.FgDim, false).Render("in ") +
		Style(p.Cyan, false).Render(props.BaseDir)
	if props.Filter != "" {
		baseLine += Style(p.FgDim, false).Render(" · filter ") +
			Style(p.Yellow, false).Render(props.Filter)
	}

	searchStr := props.Search
	if searchStr == "" {
		searchStr = Style(p.FgMuted, false).Render("type to filter…")
	} else {
		searchStr = Style(p.Fg, false).Render(searchStr) +
			Style(p.Accent, true).Render("▌")
	}
	searchRow := Style(p.FgDim, false).Render("  ") + searchStr

	listH := h - 8
	if listH < 3 {
		listH = 3
	}
	list := renderPickerList(p, props.Entries, props.Cursor, w-6, listH)

	hint := Style(p.FgMuted, false).
		Render("[↑/↓] move  [enter] select  [←/backspace] back  [esc] cancel")

	inner := title + "\n" +
		crumb + "\n" +
		baseLine + "\n" +
		searchRow + "\n" +
		SepLine(p, w-6) + "\n" +
		list + "\n" +
		hint

	return lipgloss.NewStyle().
		Background(p.BgPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Accent).
		Padding(1, 2).
		Width(w).
		Render(inner)
}

func stepBadge(p Palette, idx, total int) string {
	if total <= 0 {
		return ""
	}
	label := ""
	for i := 0; i < total; i++ {
		dot := "○"
		if i == idx {
			dot = "●"
		} else if i < idx {
			dot = "✓"
		}
		label += dot + " "
	}
	return lipgloss.NewStyle().
		Foreground(p.Accent).
		Background(p.BgSelected).
		Padding(0, 1).
		Render(strings.TrimSpace(label))
}

func renderBreadcrumb(p Palette, cmdName string, selections []string) string {
	parts := []string{Style(p.Accent, true).Render(cmdName)}
	for _, s := range selections {
		parts = append(parts, Style(p.Green, false).Render(s))
	}
	sep := Style(p.FgDim, false).Render(" › ")
	return strings.Join(parts, sep)
}

func renderPickerList(p Palette, entries []PickerEntry, cursor, w, h int) string {
	if len(entries) == 0 {
		return Style(p.FgMuted, false).Render("  (no matches)")
	}
	// Window the list around the cursor so it stays visible.
	start := 0
	if cursor >= h {
		start = cursor - h + 1
	}
	end := start + h
	if end > len(entries) {
		end = len(entries)
	}
	var rows []string
	for i := start; i < end; i++ {
		e := entries[i]
		line := e.Icon + " " + e.Name
		if i == cursor {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(p.BgDeep).
				Background(p.Accent).
				Bold(true).
				Width(w).
				Render(" ▸ "+line))
		} else {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(p.Fg).
				Width(w).
				Render("   "+line))
		}
	}
	return strings.Join(rows, "\n")
}
