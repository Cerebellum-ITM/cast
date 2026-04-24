package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/source"
)

// Layout constants — all in terminal columns / rows.
const (
	sidebarW = 24 // includes right divider char
	outputW  = 28 // includes left divider char
	headerH  = 4  // 3 content rows (pill height) + 1 separator row
	statusH  = 1
)

// renderMain is the root renderer.
func (m Model) renderMain() string {
	if m.width == 0 || m.height == 0 {
		return "initializing…"
	}
	p := paletteFor(m.theme, m.env)

	bodyH := m.height - headerH - statusH
	if bodyH < 1 {
		bodyH = 1
	}
	centerW := m.width - sidebarW - outputW
	if centerW < 10 {
		centerW = 10
	}

	hdr := m.renderHeader(p)
	bdy := clampToHeight(m.renderBody(p, bodyH, centerW), bodyH)
	sts := m.renderStatusBar(p)

	return hdr + "\n" + bdy + "\n" + sts
}

// ── Header (2 rows) ───────────────────────────────────────────────────────────

func (m Model) renderHeader(p palette) string {
	// Right: env pill — 3 lines tall due to rounded border.
	pill := m.renderEnvPill(p)
	pillW := lipgloss.Width(pill)

	// Left: logo + divider + tabs, rendered as 3 rows to match pill height.
	// Content sits on the middle row so it visually centers next to the pill.
	logo := style(p.accent, true).Render("⬡ cast")
	div := style(p.border, false).Render(" │ ")
	tabs := m.renderTabs(p)
	leftContent := logo + div + tabs

	leftW := m.width - pillW
	if leftW < 0 {
		leftW = 0
	}
	rowStyle := lipgloss.NewStyle().Width(leftW).Background(p.bgPanel).Padding(0, 1)
	leftBlock := rowStyle.Render("") + "\n" +
		rowStyle.Render(leftContent) + "\n" +
		rowStyle.Render("")

	// Rows 1-3: left block beside pill.
	rows123 := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, pill)

	// Row 4: separator.
	sep := style(p.border, false).Render(strings.Repeat("─", m.width))

	return rows123 + "\n" + sep
}

func (m Model) renderTabs(p palette) string {
	names := []string{"commands", "history", ".env", "theme"}
	var parts []string
	for i, n := range names {
		if Tab(i) == m.activeTab {
			parts = append(parts,
				lipgloss.NewStyle().
					Foreground(p.fg).Bold(true).
					Underline(true).UnderlineColor(p.accent).
					Padding(0, 1).Render(n))
		} else {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(p.fgDim).
					Padding(0, 1).Render(n))
		}
	}
	return strings.Join(parts, " ")
}

// renderEnvPill renders the 3-button env indicator as a single inline row.
func (m Model) renderEnvPill(p palette) string {
	type envBtn struct {
		label string
		color color.Color
	}
	btns := []envBtn{
		{"DEV", lipgloss.Color("#A6E3A1")},
		{"STG", lipgloss.Color("#FAB387")},
		{"PRD", lipgloss.Color("#F38BA8")},
	}

	var parts []string
	for i, b := range btns {
		if int(m.env) == i {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(b.color).Bold(true).
					Background(p.bgSelected).Padding(0, 1).
					Render("● "+b.label))
		} else {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(p.fgDim).Padding(0, 1).
					Render("● "+b.label))
		}
	}

	inner := strings.Join(parts, "")
	return lipgloss.NewStyle().
		Background(p.bgDeep).
		Border(lipgloss.RoundedBorder()).BorderForeground(p.border).
		Padding(0, 1).
		Render(inner)
}

// ── Body ─────────────────────────────────────────────────────────────────────

// renderBody renders the three-panel row. Each panel is exactly bodyH rows tall.
func (m Model) renderBody(p palette, bodyH, centerW int) string {
	sbInner := sidebarW - 1 // content cols; divider takes the last col
	outInner := outputW - 1

	sidebar := m.renderSidebar(p, sbInner, bodyH)
	center := m.renderCenter(p, centerW, bodyH)
	output := m.renderOutput(p, outInner, bodyH)

	divStyle := style(p.border, false)
	divCol := divStyle.Render(strings.Repeat("│\n", bodyH-1) + "│")

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divCol, center, divCol, output)
}

// ── Sidebar (sidebarW-1 cols, bodyH rows) ────────────────────────────────────

func (m Model) renderSidebar(p palette, w, h int) string {
	// Fixed regions
	const (
		searchRows = 1
		sepRows    = 1 // separator below search
		hintRows   = 3 // 2 hint rows + 1 blank spacer at the bottom
		hintSepR   = 1 // separator above hints
	)
	listH := h - searchRows - sepRows - hintRows - hintSepR
	if listH < 0 {
		listH = 0
	}

	rows := make([]string, 0, h)

	// Search row
	rows = append(rows, m.renderSearchRow(p, w))
	// Separator
	rows = append(rows, sepLine(p, w))
	// Command list
	rows = append(rows, m.renderCommandList(p, w, listH)...)
	// Separator
	rows = append(rows, sepLine(p, w))
	// Hints row
	rows = append(rows, m.renderHintsRow(p, w))

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(w).Height(h).
		Background(p.bgPanel).
		Render(content)
}

func (m Model) renderSearchRow(p palette, w int) string {
	icon := style(p.fgDim, false).Render("⌕ ")
	input := style(p.fg, false).Render(m.searchInput.View())
	available := w - visWidth(icon) - 2
	inputStr := truncate(m.searchInput.Value(), available)
	if inputStr == "" && !m.searchInput.Focused() {
		inputStr = style(p.fgDim, false).Render("search…")
	} else {
		inputStr = style(p.fg, false).Render(inputStr)
		if m.searchInput.Focused() {
			inputStr += style(p.accent, false).Render("▌")
		}
	}
	_ = input
	return lipgloss.NewStyle().Width(w).Padding(0, 1).
		Background(p.bgPanel).
		Render(icon + inputStr)
}

func (m Model) renderCommandList(p palette, w, listH int) []string {
	// Each command card is 2 rows: name row + detail row.
	const rowsPerItem = 2
	maxItems := listH / rowsPerItem

	// Scroll the visible window so selected item is always visible.
	start := 0
	if m.selected >= maxItems {
		start = m.selected - maxItems + 1
	}

	rows := make([]string, listH)
	for i := range rows {
		rows[i] = lipgloss.NewStyle().Width(w).Background(p.bgPanel).Render("")
	}

	slot := 0
	for i := start; i < len(m.filtered) && slot < maxItems; i++ {
		selected := i == m.selected
		r1, r2 := m.renderCommandCard(p, m.filtered[i], selected, w)
		rows[slot*rowsPerItem] = r1
		rows[slot*rowsPerItem+1] = r2
		slot++
	}
	return rows
}

// renderCommandCard renders a single command as a 2-row card block.
func (m Model) renderCommandCard(
	p palette,
	cmd source.Command,
	selected bool,
	w int,
) (string, string) {
	bg := p.bgPanel
	fg := p.fgDim
	if selected {
		bg = p.bgSelected
		fg = p.fg
	}

	// Key badge
	badge := renderKeyBadge(p, cmd.Shortcut)
	badgeW := visWidth(badge)

	// Tag chip (first tag) — right-aligned
	var tagChip string
	tagChipW := 0
	if len(cmd.Tags) > 0 {
		tagChip = renderTagChip(p, cmd.Tags[0])
		tagChipW = visWidth(tagChip) + 1
	}

	// Border takes 1 column; content fills the rest.
	contentW := w - 1

	// Row 1: badge + name (right-aligns tag) within contentW
	nameAvail := contentW - badgeW - 1 - tagChipW
	if nameAvail < 1 {
		nameAvail = 1
	}
	name := truncate(cmd.Name, nameAvail)
	nameStr := lipgloss.NewStyle().Foreground(fg).Bold(selected).Render(name)

	namePad := nameAvail - visWidth(name)
	if namePad < 0 {
		namePad = 0
	}

	var row1Content string
	if tagChip != "" {
		row1Content = badge + " " + nameStr + strings.Repeat(" ", namePad) + " " + tagChip
	} else {
		row1Content = badge + " " + nameStr
	}

	// Row 2: description indented to align under name
	indent := badgeW + 1 // badge + space
	descAvail := contentW - indent - 1
	if descAvail < 1 {
		descAvail = 1
	}
	desc := truncate(cmd.Desc, descAvail)
	descStr := lipgloss.NewStyle().Foreground(p.fgMuted).Render(desc)
	row2Content := strings.Repeat(" ", indent) + descStr

	// Left border: thick accent for selected, normal dim for unselected.
	var rowStyle lipgloss.Style
	if selected {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(bg).
			BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(p.accent).BorderBackground(bg)
	} else {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(bg).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(p.border).BorderBackground(bg)
	}

	row1 := rowStyle.Render(row1Content)
	row2 := rowStyle.Render(row2Content)
	return row1, row2
}

func renderKeyBadge(p palette, key string) string {
	if key == "" {
		key = " "
	}
	return lipgloss.NewStyle().
		Foreground(p.bgDeep).Bold(true).
		Background(p.accent).
		Padding(0, 1).
		Render(key)
}

// renderTagChip renders a tag with the tag color as text on a dark background.
func renderTagChip(p palette, text string) string {
	c := tagColor(p, text)
	return lipgloss.NewStyle().
		Foreground(c).
		Background(p.bgSelected).
		Padding(0, 1).
		Render(text)
}

func (m Model) renderHintsRow(p palette, w int) string {
	hints := [][2]string{{"↑↓", "nav"}, {"⏎", "run"}, {"/", "search"}, {"q", "quit"}}
	avail := w - 2 // subtract Padding(0,1) left+right
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.bgPanel)

	// Pack hints into rows: each row holds as many hints as fit in avail cols.
	var lines []string
	var rowParts []string
	rowUsed := 0

	for _, h := range hints {
		key := style(p.accent, true).Render("[" + h[0] + "]")
		lbl := style(p.fgDim, false).Render(h[1])
		part := key + " " + lbl
		partW := visWidth(part)
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

	// Blank spacer row for visual breathing room below hints.
	lines = append(lines, rowStyle.Render(""))
	return strings.Join(lines, "\n")
}

// ── Center panel ──────────────────────────────────────────────────────────────

func (m Model) renderCenter(p palette, w, h int) string {
	switch m.activeTab {
	case TabHistory:
		return m.renderHistory(p, w, h)
	case TabEnv:
		return m.renderEnvPane(p, w, h)
	default:
		return m.renderCommands(p, w, h)
	}
}

func (m Model) renderCommands(p palette, w, h int) string {
	cmdHeader, cmdHeaderH := m.renderCommandHeader(p, w)
	previewH := h - cmdHeaderH
	if previewH < 1 {
		previewH = 1
	}
	preview := m.renderMakefilePreview(p, w, previewH)
	return lipgloss.NewStyle().Width(w).Height(h).
		Background(p.bgDeep).
		Render(cmdHeader + "\n" + preview)
}

// renderCommandHeader returns the rendered header and its exact row count.
func (m Model) renderCommandHeader(p palette, w int) (string, int) {
	if len(m.filtered) == 0 {
		noCmd := lipgloss.NewStyle().Width(w).Background(p.bgPanel).
			Padding(1, 2).Foreground(p.fgMuted).Render("no commands")
		return noCmd + "\n" + sepLine(p, w), lipgloss.Height(noCmd) + 1
	}

	cmd := m.filtered[m.selected]

	// Row 1: name (accent bold) + category badge + tag badges + env warning
	nameStr := style(p.accent, true).Render(cmd.Name)
	// "Build" badge — capitalized command name in bgSelected style
	nameBadge := lipgloss.NewStyle().
		Background(p.bgSelected).Foreground(p.fg).Bold(true).
		Padding(0, 1).Render(strings.Title(cmd.Name))
	row1 := nameStr + "  " + nameBadge
	if cmd.Category != "" {
		row1 += "  " + renderInlineTag(p, cmd.Category)
	}
	for _, t := range cmd.Tags {
		row1 += "  " + renderInlineTag(p, t)
	}
	if m.env == 1 {
		row1 += "  " + style(p.orange, false).Render("⚠ staging")
	} else if m.env == 2 {
		row1 += "  " + style(p.red, false).Render("⚠ production")
	}

	// Description row
	descRow := style(p.fgDim, false).Render(cmd.Desc)

	// Command row: $ make <name> ────── ⏎ Run (button right-aligned)
	dollar := style(p.fgDim, false).Render("$ ")
	makeCmd := style(p.cyan, false).Render("make " + cmd.Name)
	cmdBox := lipgloss.NewStyle().
		Background(p.bgDeep).
		Padding(0, 1).
		Render(dollar + makeCmd)

	var runBtn string
	if m.running {
		runBtn = lipgloss.NewStyle().
			Background(p.bgSelected).Foreground(p.fgDim).Bold(true).
			Padding(0, 2).Render("…")
	} else {
		runBtn = lipgloss.NewStyle().
			Background(p.accent).Foreground(p.bgDeep).Bold(true).
			Padding(0, 2).Render("⏎ Run")
	}
	// Push Run button to the far right of the panel.
	cmdBoxW := visWidth(cmdBox)
	btnW := visWidth(runBtn)
	innerW := w - 4 // account for 2-space left pad
	gapW := innerW - cmdBoxW - btnW
	if gapW < 1 {
		gapW = 1
	}
	cmdRow := cmdBox + strings.Repeat(" ", gapW) + runBtn

	lines := []string{
		"",
		pad(2) + row1,
		pad(2) + descRow,
		"",
		pad(2) + cmdRow,
	}

	if m.running {
		pct := fmt.Sprintf("%.0f%%", m.runProgress*100)
		bar := renderProgressBar(p, w-4, m.runProgress)
		lines = append(lines, pad(2)+bar)
		lines = append(lines, pad(2)+style(p.fgDim, false).Render(pct))
	}

	lines = append(lines, "")
	lines = append(lines, sepLine(p, w))

	content := strings.Join(lines, "\n")
	rendered := lipgloss.NewStyle().Width(w).Background(p.bgPanel).Render(content)
	return rendered, len(lines)
}

func (m Model) renderMakefilePreview(p palette, w, h int) string {
	if len(m.makefileLines) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Background(p.bgDeep).
			Padding(1, 2).Foreground(p.fgMuted).
			Render("no makefile loaded")
	}

	// Path header (1 row)
	pathRow := lipgloss.NewStyle().Width(w).Padding(0, 2).Background(p.bgDeep).
		Render(style(p.fgDim, false).Render(m.makefilePath) + "  " +
			style(p.fgMuted, false).Render(fmt.Sprintf("%d lines", len(m.makefileLines))))

	codeH := h - 1
	if codeH < 0 {
		codeH = 0
	}

	start := m.makefileOffset
	if start >= len(m.makefileLines) {
		start = 0
	}
	end := start + codeH
	if end > len(m.makefileLines) {
		end = len(m.makefileLines)
	}

	codeLines := make([]string, codeH)
	for i := start; i < end; i++ {
		lineNum := lipgloss.NewStyle().Foreground(p.fgMuted).Width(3).
			Render(fmt.Sprintf("%3d", i+1))
		hl := highlightMakefileLine(p, m.makefileLines[i])
		codeLines[i-start] = "  " + lineNum + "  " + hl
	}

	code := strings.Join(codeLines, "\n")
	preview := lipgloss.NewStyle().Width(w).Height(codeH).Background(p.bgDeep).
		Render(code)

	return pathRow + "\n" + preview
}

func (m Model) renderHistory(p palette, w, h int) string {
	titleRow := lipgloss.NewStyle().Width(w).Padding(0, 2).
		Background(p.bgPanel).Foreground(p.fgDim).Bold(true).
		Render("HISTORY")
	sep := sepLine(p, w)

	var entries []string
	for _, r := range m.history {
		dot := statusDot(p, r.Status)
		row := dot + " " + style(p.fg, true).Render(r.Command) +
			"  " + style(p.fgDim, false).Render(r.Duration) +
			"  " + style(p.fgMuted, false).Render(r.Time)
		entries = append(entries, lipgloss.NewStyle().Width(w).Padding(0, 2).
			Background(p.bgPanel).Render(row))
	}
	if len(entries) == 0 {
		entries = append(entries, lipgloss.NewStyle().Width(w).Padding(1, 2).
			Background(p.bgPanel).Foreground(p.fgMuted).Render("no history yet"))
	}

	content := titleRow + "\n" + sep + "\n" + strings.Join(entries, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.bgPanel).Render(content)
}

func (m Model) renderEnvPane(p palette, w, h int) string {
	title := style(p.accent, true).Render(".env")
	note := style(p.fgDim, false).Render("env file viewer — coming soon")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.bgDeep).
		Padding(1, 2).Render(title + "\n" + note)
}

// ── Output panel (outputW-1 cols, bodyH rows) ─────────────────────────────────

func (m Model) renderOutput(p palette, w, h int) string {
	// Header row
	outputLabel := lipgloss.NewStyle().Foreground(p.fgDim).Bold(true).Render("OUTPUT")
	var statusStr string
	switch {
	case m.running:
		statusStr = lipgloss.NewStyle().Foreground(p.yellow).
			Render(m.spinner.View() + " " + truncate(m.lastRunCmd, 10))
	case m.hasLastRun && m.lastRunOK:
		statusStr = style(p.green, false).Render("✓ success")
	case m.hasLastRun:
		statusStr = style(p.red, false).Render("✗ error")
	}
	hGap := w - visWidth(outputLabel) - visWidth(statusStr) - 2
	if hGap < 0 {
		hGap = 0
	}
	headerRow := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.bgPanel).
		Render(outputLabel + strings.Repeat(" ", hGap) + statusStr)
	sep := sepLine(p, w)

	// Progress bar (1 row, only when running)
	var progressRow string
	progH := 0
	if m.running {
		progressRow = renderProgressBar(p, w, m.runProgress)
		progH = 1
	}

	// RECENT section at bottom — fixed 6 rows (label + sep + 4 entries)
	const recentMax = 4
	recentRows := m.renderRecentRows(p, w, recentMax)
	recentH := len(recentRows)

	// Terminal output fills the gap
	termH := h - 2 - progH - recentH // 2 = header + sep
	if termH < 1 {
		termH = 1
	}
	termRows := m.renderTermRows(p, w, termH)

	allRows := []string{headerRow, sep}
	if m.running {
		allRows = append(allRows, progressRow)
	}
	allRows = append(allRows, termRows...)
	allRows = append(allRows, recentRows...)

	content := strings.Join(allRows, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.bgDeep).Render(content)
}

func (m Model) renderTermRows(p palette, w, h int) []string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = lipgloss.NewStyle().Width(w).Background(p.bgDeep).Render("")
	}

	if len(m.output) == 0 {
		placeholder := style(p.fgMuted, false).Render("run a command to see output…")
		rows[0] = lipgloss.NewStyle().
			Width(w).
			Padding(0, 1).
			Background(p.bgDeep).
			Render(placeholder)
		return rows
	}

	start := 0
	if len(m.output) > h {
		start = len(m.output) - h
	}
	for i, l := range m.output[start:] {
		if i >= h {
			break
		}
		rows[i] = lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.bgDeep).
			Render(colorOutputLine(p, l))
	}
	return rows
}

func (m Model) renderRecentRows(p palette, w, max int) []string {
	label := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.bgPanel).
		Foreground(p.fgDim).Bold(true).Render("RECENT")
	sep := sepLine(p, w)

	rows := []string{label, sep}

	for i, r := range m.history {
		if i >= max {
			break
		}
		dot := statusDot(p, r.Status)
		name := style(p.fgDim, false).Render(truncate(r.Command, 10))
		dur := style(p.fgMuted, false).Render(r.Duration)
		ts := style(p.fgMuted, false).Render(r.Time)
		row := dot + " " + name + "  " + dur + "  " + ts
		rows = append(rows, lipgloss.NewStyle().Width(w).Padding(0, 1).
			Background(p.bgPanel).Render(row))
	}
	// Pad to max entries for stable height
	for len(rows) < 2+max {
		rows = append(rows, lipgloss.NewStyle().Width(w).Background(p.bgPanel).Render(""))
	}
	return rows
}

// ── Status bar (1 row) ────────────────────────────────────────────────────────

func (m Model) renderStatusBar(p palette) string {
	left := fmt.Sprintf("⬡ cast  %d commands  ● %s", len(m.commands), m.env.String())
	right := "~/projects/myapp  Makefile  v0.1.0"

	// Use visWidth (visual columns) not len (bytes) — multi-byte Unicode chars
	// like ⬡ and ● have len > 1 but visual width 1.
	usedW := visWidth(left) + visWidth(right)
	gap := m.width - usedW - 2 // -2 for the two Padding(0,1) spaces
	if gap < 0 {
		gap = 0
	}
	content := left + strings.Repeat(" ", gap) + right

	return lipgloss.NewStyle().
		Background(p.accent).
		Foreground(p.bgDeep).
		Bold(true).
		Width(m.width).
		Padding(0, 1).
		Render(content)
}

// ── Primitives ────────────────────────────────────────────────────────────────

// style returns a simple fg lipgloss style.
func style(c color.Color, bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(c)
	if bold {
		s = s.Bold(true)
	}
	return s
}

// renderInlineTag renders a tag with the tag color as text on a dark background.
func renderInlineTag(p palette, text string) string {
	c := tagColor(p, text)
	return lipgloss.NewStyle().
		Foreground(c).
		Background(p.bgSelected).
		Padding(0, 1).
		Render(text)
}

// tagColor maps a tag string to a palette color.
func tagColor(p palette, tag string) color.Color {
	switch strings.ToLower(tag) {
	case "go":
		return p.cyan
	case "ci", "golangci":
		return p.orange
	case "prod", "production":
		return p.red
	case "staging":
		return p.orange
	case "local":
		return p.green
	default:
		return p.accent
	}
}

// renderProgressBar draws a 1-row progress bar with block fill characters.
func renderProgressBar(p palette, w int, progress float64) string {
	if w < 2 {
		return ""
	}
	filled := int(float64(w) * progress)
	if filled > w {
		filled = w
	}
	return lipgloss.NewStyle().Foreground(p.accent).Render(strings.Repeat("▓", filled)) +
		lipgloss.NewStyle().Foreground(p.fgMuted).Render(strings.Repeat("░", w-filled))
}

func statusDot(p palette, status RunStatus) string {
	switch status {
	case StatusSuccess:
		return style(p.green, false).Render("●")
	case StatusError:
		return style(p.red, false).Render("●")
	case StatusRunning:
		return style(p.yellow, false).Render("●")
	default:
		return style(p.fgDim, false).Render("●")
	}
}

func colorOutputLine(p palette, line string) string {
	switch {
	case strings.HasPrefix(line, "✓"):
		return style(p.green, false).Render(line)
	case strings.HasPrefix(line, "✗"):
		return style(p.red, false).Render(line)
	case strings.HasPrefix(line, "$"):
		return style(p.cyan, false).Render(line)
	case strings.HasPrefix(line, "--- PASS"):
		return style(p.green, false).Render(line)
	case strings.HasPrefix(line, "--- FAIL"):
		return style(p.red, false).Render(line)
	default:
		return style(p.fgDim, false).Render(line)
	}
}

func highlightMakefileLine(p palette, line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "##"), strings.HasPrefix(trimmed, "#"),
		strings.HasPrefix(trimmed, ".PHONY"):
		return style(p.fgDim, false).Render(line)
	case strings.Contains(line, " = ") || strings.Contains(line, " := "):
		idx := strings.Index(line, "=")
		if idx > 0 {
			return style(p.cyan, false).Render(line[:idx]) +
				style(p.fgDim, false).Render("=") +
				style(p.yellow, false).Render(line[idx+1:])
		}
	case !strings.HasPrefix(line, "\t") && strings.HasSuffix(trimmed, ":"):
		return style(p.accent, false).Render(line)
	case strings.HasPrefix(line, "\t@echo"):
		return style(p.fgDim, false).Render("\t@") +
			style(p.green, false).Render(strings.TrimPrefix(line, "\t@"))
	case strings.HasPrefix(line, "\t@"):
		return style(p.fgDim, false).Render("\t@") +
			style(p.fg, false).Render(strings.TrimPrefix(line, "\t@"))
	}
	return style(p.fgDim, false).Render(line)
}

// sepLine returns a full-width separator using ─ characters.
func sepLine(p palette, w int) string {
	return style(p.border, false).Render(strings.Repeat("─", w))
}

// pad returns n spaces for manual indentation.
func pad(n int) string { return strings.Repeat(" ", n) }

// visWidth returns the visible character width of a styled string.
func visWidth(s string) int { return lipgloss.Width(s) }

// dimColor returns a slightly darkened variant of a color for tag backgrounds.
// Terminals don't support alpha; we approximate by returning the same color
// (lipgloss will use default terminal blending).
func dimColor(c color.Color) color.Color { return c }

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

// clampToHeight trims a multi-line string to at most h rows.
// Prevents any over-tall section from pushing the status bar off screen.
func clampToHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= h {
		return s
	}
	return strings.Join(lines[:h], "\n")
}
