package views

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/source"
)

// SidebarProps holds all data needed to render the left command-list panel.
type SidebarProps struct {
	Commands      []source.Command
	Selected      int
	Search        string
	SearchFocused bool
	Width         int
	Height        int
}

// Sidebar renders the left panel: search, command list, and keyboard hints.
func Sidebar(p Palette, props SidebarProps) string {
	w, h := props.Width, props.Height

	const (
		searchRows = 1
		sepRows    = 1
		hintRows   = 3 // 2 hint rows + 1 blank spacer
		hintSepR   = 1
	)
	listH := h - searchRows - sepRows - hintRows - hintSepR
	if listH < 0 {
		listH = 0
	}

	rows := make([]string, 0, h)
	rows = append(rows, renderSearchRow(p, props, w))
	rows = append(rows, SepLine(p, w))
	rows = append(rows, renderCommandList(p, props, w, listH)...)
	rows = append(rows, SepLine(p, w))
	rows = append(rows, renderHintsRow(p, w))

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgPanel).Render(content)
}

func renderSearchRow(p Palette, props SidebarProps, w int) string {
	icon := Style(p.FgDim, false).Render("⌕ ")
	available := w - VisWidth(icon) - 2

	var inputStr string
	if props.Search == "" && !props.SearchFocused {
		inputStr = Style(p.FgDim, false).Render("search…")
	} else {
		inputStr = Style(p.Fg, false).Render(Truncate(props.Search, available))
		if props.SearchFocused {
			inputStr += Style(p.Accent, false).Render("▌")
		}
	}

	return lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgPanel).
		Render(icon + inputStr)
}

func renderCommandList(p Palette, props SidebarProps, w, listH int) []string {
	const rowsPerItem = 2
	maxItems := listH / rowsPerItem

	start := 0
	if props.Selected >= maxItems {
		start = props.Selected - maxItems + 1
	}

	rows := make([]string, listH)
	for i := range rows {
		rows[i] = lipgloss.NewStyle().Width(w).Background(p.BgPanel).Render("")
	}

	slot := 0
	for i := start; i < len(props.Commands) && slot < maxItems; i++ {
		r1, r2 := renderCommandCard(p, props.Commands[i], i == props.Selected, w)
		rows[slot*rowsPerItem] = r1
		rows[slot*rowsPerItem+1] = r2
		slot++
	}
	return rows
}

func renderCommandCard(p Palette, cmd source.Command, selected bool, w int) (string, string) {
	bg, fg := p.BgPanel, p.FgDim
	if selected {
		bg, fg = p.BgSelected, p.Fg
	}

	badge := RenderKeyBadge(p, cmd.Shortcut)
	badgeW := VisWidth(badge)

	var tagChip string
	tagChipW := 0
	if len(cmd.Tags) > 0 {
		tagChip = RenderTagChip(p, cmd.Tags[0])
		tagChipW = VisWidth(tagChip) + 1
	}

	contentW := w - 1
	nameAvail := contentW - badgeW - 1 - tagChipW
	if nameAvail < 1 {
		nameAvail = 1
	}

	name := Truncate(cmd.Name, nameAvail)
	nameStr := lipgloss.NewStyle().Foreground(fg).Bold(selected).Render(name)
	namePad := nameAvail - VisWidth(name)
	if namePad < 0 {
		namePad = 0
	}

	var row1Content string
	if tagChip != "" {
		row1Content = badge + " " + nameStr + strings.Repeat(" ", namePad) + " " + tagChip
	} else {
		row1Content = badge + " " + nameStr
	}

	indent := badgeW + 1
	descAvail := contentW - indent - 1
	if descAvail < 1 {
		descAvail = 1
	}
	descStr := lipgloss.NewStyle().Foreground(p.FgDim).Render(Truncate(cmd.Desc, descAvail))
	row2Content := strings.Repeat(" ", indent) + descStr

	var rowStyle lipgloss.Style
	if selected {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(bg).
			BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(p.Accent).BorderBackground(bg)
	} else {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(bg).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(p.Border).BorderBackground(bg)
	}

	return rowStyle.Render(row1Content), rowStyle.Render(row2Content)
}

func renderHintsRow(p Palette, w int) string {
	hints := [][2]string{{"↑↓", "nav"}, {"⏎", "run"}, {"/", "search"}, {"ctrl+o", "source"}, {"q", "quit"}}
	avail := w - 2
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgPanel)

	var lines []string
	var rowParts []string
	rowUsed := 0

	for _, h := range hints {
		key := Style(p.Accent, true).Render("[" + h[0] + "]")
		lbl := Style(p.FgDim, false).Render(h[1])
		part := key + " " + lbl
		partW := VisWidth(part)
		gap := 0
		if len(rowParts) > 0 {
			gap = 1
		}
		if rowUsed+gap+partW > avail && len(rowParts) > 0 {
			lines = append(lines, rowStyle.Render(strings.Join(rowParts, " ")))
			rowParts = nil
			rowUsed = 0
			gap = 0
		}
		rowParts = append(rowParts, part)
		rowUsed += gap + partW
	}
	if len(rowParts) > 0 {
		lines = append(lines, rowStyle.Render(strings.Join(rowParts, " ")))
	}
	lines = append(lines, rowStyle.Render(""))
	return strings.Join(lines, "\n")
}
