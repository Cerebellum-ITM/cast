package views

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/db"
	"github.com/Cerebellum-ITM/cast/internal/source"
)

// SidebarProps holds all data needed to render the left command-list panel.
// In single mode the list shows individual commands; in chain mode it shows
// the deduplicated auto-saved chains.
type SidebarProps struct {
	Commands      []source.Command
	Selected      int
	Search        string
	SearchFocused bool
	Width         int
	Height        int

	Mode     int // 0 = single, 1 = chain
	Chains   []db.SequenceSummary
	ChainSel int

	// Chain builder overlay on single mode: slim accent bar + order number
	// for checked rows. Empty when builder is off.
	ChainBuilder bool
	ChainChecked []string

	// Active chain/queue: rendered as a "QUEUE" section at the top while a
	// chain (len >= 2) is running. CurrentStep is the 0-indexed position of
	// the step currently executing.
	QueueCommands []string
	CurrentStep   int
	// QueueQuit signals that the user has queued an exit after the chain
	// drains. Rendered as a dimmed `⏻ quit` chip below the step list and
	// excluded from the (N) count on the CHAIN header.
	QueueQuit bool
	IconStyle IconStyle

	// LastRunCmds is the persisted "rerun" target rendered as a card pinned
	// at the top of the command list. Single-element slice => single
	// command; ≥2 elements => chain joined with " › ". Empty disables the
	// card. Persists across CLI restarts via internal/state.
	LastRunCmds    []string
	LastRunIsPick  bool // shows a "(pick)" hint when extras are cached
	LastRunFocused bool // selection is on the rerun card
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

	var queueBlock []string
	if len(props.QueueCommands) > 1 || (props.QueueQuit && len(props.QueueCommands) >= 1) {
		queueBlock = renderQueueBlock(p, props.QueueCommands, props.CurrentStep, w, props.QueueQuit, props.IconStyle)
	}

	var rerunBlock []string
	if len(props.LastRunCmds) > 0 && props.Mode == 0 {
		rerunBlock = renderRerunCard(p, props.LastRunCmds, props.LastRunIsPick, props.LastRunFocused, w)
	}

	listH := h - searchRows - sepRows - hintRows - hintSepR - len(queueBlock) - len(rerunBlock)
	if len(queueBlock) > 0 {
		listH -= 1 // extra sep under queue
	}
	if len(rerunBlock) > 0 {
		listH -= 1 // extra sep under rerun
	}
	if listH < 0 {
		listH = 0
	}

	rows := make([]string, 0, h)
	rows = append(rows, renderSearchRow(p, props, w))
	rows = append(rows, SepLine(p, w))
	if len(queueBlock) > 0 {
		rows = append(rows, queueBlock...)
		rows = append(rows, SepLine(p, w))
	}
	if len(rerunBlock) > 0 {
		rows = append(rows, rerunBlock...)
		rows = append(rows, SepLine(p, w))
	}
	if props.Mode == 1 {
		rows = append(rows, renderChainList(p, props, w, listH)...)
	} else {
		rows = append(rows, renderCommandList(p, props, w, listH)...)
	}
	rows = append(rows, SepLine(p, w))
	rows = append(rows, renderHintsRow(p, w, props.Mode, props.ChainBuilder))

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Render(content)
}

// renderRerunCard renders the persistent "last command" card pinned above
// the regular command list. Uses the warning/temporary accent (yellow) to
// signal that this is a synthetic, ephemeral entry and not a Makefile
// target. Two-row layout matches renderCommandCard so the rest of the
// sidebar stays vertically aligned.
func renderRerunCard(p Palette, cmds []string, isPick, focused bool, w int) []string {
	contentW := w - 1
	accent := p.Yellow

	badge := lipgloss.NewStyle().
		Foreground(p.BgDeep).Bold(true).
		Background(accent).
		Padding(0, 1).
		Render("↻")

	avail := contentW - VisWidth(badge) - 3
	if avail < 4 {
		avail = 4
	}
	display := strings.Join(cmds, " › ")
	nameStr := lipgloss.NewStyle().
		Foreground(p.Fg).Bold(focused).
		Render(Truncate(display, avail))

	row1 := badge + " " + nameStr

	label := "last command"
	if len(cmds) > 1 {
		label = "last chain (" + itoa(len(cmds)) + ")"
	} else if isPick {
		label = "last command (pick)"
	}
	row2Inner := lipgloss.NewStyle().Foreground(accent).Italic(true).
		Render(Truncate(label, contentW-2))
	row2 := "  " + row2Inner

	var rowStyle lipgloss.Style
	if focused {
		rowStyle = lipgloss.NewStyle().Width(contentW).
			Background(p.BgSelected).
			BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(accent).BorderBackground(p.BgSelected)
	} else {
		rowStyle = lipgloss.NewStyle().Width(contentW).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(accent)
	}
	return []string{rowStyle.Render(row1), rowStyle.Render(row2)}
}

// renderQueueBlock renders a compact list of the in-flight chain steps with
// the current step highlighted. Steps already done are dimmed, pending ones
// are faint, the running one is bold with a ▶ marker.
func renderQueueBlock(p Palette, steps []string, current, w int, quitQueued bool, iconStyle IconStyle) []string {
	title := lipgloss.NewStyle().Width(w).Padding(0, 1).
		Foreground(p.Accent).Bold(true).
		Render("CHAIN (" + itoa(len(steps)) + ")")
	rows := []string{title}
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1)
	avail := w - 2 - 4
	if avail < 4 {
		avail = 4
	}
	for i, s := range steps {
		marker := "  "
		numFg := p.FgMuted
		nameFg := p.FgDim
		bold := false
		switch {
		case i < current:
			marker = "✓ "
			numFg = p.Green
			nameFg = p.FgDim
		case i == current:
			marker = "▶ "
			numFg = p.Accent
			nameFg = p.Fg
			bold = true
		default:
			marker = "· "
			numFg = p.FgMuted
			nameFg = p.FgMuted
		}
		num := lipgloss.NewStyle().Foreground(numFg).Render(itoa(i + 1) + ".")
		name := lipgloss.NewStyle().Foreground(nameFg).Bold(bold).
			Render(Truncate(s, avail))
		rows = append(rows, rowStyle.Render(marker+num+" "+name))
	}
	if quitQueued {
		icon := Icons(iconStyle).Quit
		marker := lipgloss.NewStyle().Foreground(p.Accent).Render(icon + " ")
		label := lipgloss.NewStyle().Foreground(p.FgDim).Italic(true).
			Render(Truncate("quit", avail))
		rows = append(rows, rowStyle.Render(marker+label))
	}
	return rows
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
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

	return lipgloss.NewStyle().Width(w).Padding(0, 1).
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
		rows[i] = lipgloss.NewStyle().Width(w).Render("")
	}

	slot := 0
	for i := start; i < len(props.Commands) && slot < maxItems; i++ {
		var mark string
		if props.ChainBuilder {
			mark = chainCheckboxFor(p, props.Commands[i].Name, props.ChainChecked)
		}
		r1, r2 := renderCommandCard(p, props.Commands[i], i == props.Selected, w, mark)
		rows[slot*rowsPerItem] = r1
		rows[slot*rowsPerItem+1] = r2
		slot++
	}
	return rows
}

// chainCheckboxFor renders a 2-col prefix: accent bar + order number when
// selected, a dim hairline when not, so selected items stand out without
// shifting the row layout.
func chainCheckboxFor(p Palette, name string, checked []string) string {
	for i, n := range checked {
		if n == name {
			bar := lipgloss.NewStyle().Foreground(p.Accent).Bold(true).Render("▌")
			num := lipgloss.NewStyle().Foreground(p.Accent).Bold(true).Render(itoa(i + 1))
			return bar + num
		}
	}
	return lipgloss.NewStyle().Foreground(p.Border).Render("│ ")
}

// renderChainList renders saved chains in the sidebar when the app is in
// chain mode. Each card is a two-row block similar to commands: the first
// shows a slim accent bar + the chain's command joinline, the second the
// last-run timestamp and status.
func renderChainList(p Palette, props SidebarProps, w, listH int) []string {
	const rowsPerItem = 2
	maxItems := listH / rowsPerItem
	rows := make([]string, listH)
	for i := range rows {
		rows[i] = lipgloss.NewStyle().Width(w).Render("")
	}
	if len(props.Chains) == 0 {
		empty := lipgloss.NewStyle().Width(w-2).Padding(1, 1).
			Foreground(p.FgDim).
			Render("no chains yet · queue targets while one runs to create one")
		rows[0] = empty
		return rows
	}
	start := 0
	if props.ChainSel >= maxItems {
		start = props.ChainSel - maxItems + 1
	}
	slot := 0
	for i := start; i < len(props.Chains) && slot < maxItems; i++ {
		r1, r2 := renderChainCard(p, props.Chains[i], i == props.ChainSel, w)
		rows[slot*rowsPerItem] = r1
		rows[slot*rowsPerItem+1] = r2
		slot++
	}
	return rows
}

func renderChainCard(p Palette, s db.SequenceSummary, selected bool, w int) (string, string) {
	fg := p.FgDim
	if selected {
		fg = p.Fg
	}
	contentW := w - 1
	avail := contentW - 2
	if avail < 8 {
		avail = 8
	}

	label := Truncate(strings.Join(s.Commands, " › "), avail-2)
	name := lipgloss.NewStyle().Foreground(fg).Bold(selected).Render(label)

	var subtitle string
	if s.LastRunAt.Valid {
		subtitle = s.LastRunAt.Time.Local().Format("15:04:05") + " · "
	}
	subtitle += "runs " + itoa(s.RunCount)
	if s.LastStatus.Valid {
		st := db.RunStatus(s.LastStatus.Int64)
		c := p.FgMuted
		switch st {
		case db.StatusSuccess:
			c = p.Green
		case db.StatusError:
			c = p.Red
		case db.StatusInterrupted:
			c = p.Orange
		}
		subtitle += " · " + lipgloss.NewStyle().Foreground(c).Render(st.String())
	}
	sub := lipgloss.NewStyle().Foreground(p.FgDim).Render(Truncate(subtitle, avail-2))

	var rowStyle lipgloss.Style
	if selected {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(p.BgSelected).
			BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(p.Accent).BorderBackground(p.BgSelected)
	} else {
		rowStyle = lipgloss.NewStyle().Width(contentW).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(p.Border)
	}
	return rowStyle.Render(" " + name), rowStyle.Render(" " + sub)
}

func renderCommandCard(p Palette, cmd source.Command, selected bool, w int, chainMark string) (string, string) {
	fg := p.FgDim
	if selected {
		fg = p.Fg
	}

	badge := RenderKeyBadge(p, cmd.Shortcut)
	if chainMark != "" {
		badge = chainMark + " " + badge
	}
	badgeW := VisWidth(badge)

	contentW := w - 1
	// minNameW keeps at least a few characters of the command name visible
	// before we spend width on tag chips.
	const minNameW = 6
	// trailBuf is an unstyled space appended after the last chip. Without it,
	// lipgloss v2 collapses the chip's own right-padding (a styled trailing
	// space) when the row fills Width() exactly, leaving the chip looking cut.
	trailBuf := 1

	// Greedy fit: take tag chips in order until adding another would leave
	// fewer than minNameW columns for the name.
	var chips []string
	chipsW := 0
	budgetBase := contentW - badgeW - 1 // everything to the right of "badge "
	for _, t := range cmd.Tags {
		chip := RenderTagChip(p, t)
		need := chipsW + VisWidth(chip) + 1 // +1 leading separator
		if budgetBase-need-trailBuf < minNameW {
			break
		}
		chips = append(chips, chip)
		chipsW = need
	}
	if len(chips) == 0 {
		trailBuf = 0
	}

	nameAvail := budgetBase - chipsW - trailBuf
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
	if len(chips) > 0 {
		row1Content = badge + " " + nameStr + strings.Repeat(" ", namePad) + " " +
			strings.Join(chips, " ") + " "
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
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(p.BgSelected).
			BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(p.Accent).BorderBackground(p.BgSelected)
	} else {
		rowStyle = lipgloss.NewStyle().Width(contentW).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(p.Border)
	}

	return rowStyle.Render(row1Content), rowStyle.Render(row2Content)
}

func renderHintsRow(p Palette, w int, mode int, builder bool) string {
	var hints [][2]string
	switch {
	case builder:
		hints = [][2]string{{"↑↓", "nav"}, {"space", "toggle"}, {"⏎", "run chain"}, {"esc", "cancel"}}
	case mode == 1:
		hints = [][2]string{{"↑↓", "nav"}, {"⏎", "re-run"}, {"ctrl+s", "single"}, {"q", "quit"}}
	default:
		hints = [][2]string{{"↑↓", "nav"}, {"⏎", "run"}, {"/", "search"}, {"ctrl+s", "mode"}, {"ctrl+a", "chain"}, {"q", "quit"}}
	}
	avail := w - 2
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1)

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
