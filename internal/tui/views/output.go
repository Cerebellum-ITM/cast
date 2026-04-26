package views

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/Cerebellum-ITM/cast/internal/db"
)

// OutputProps holds all data needed to render the right output panel.
type OutputProps struct {
	Lines       []string
	History     []db.Run
	Running     bool
	Streaming   bool // running command is a long-lived log stream
	LivePulse   bool // flip each tick to animate the LIVE dot
	HasLastRun  bool
	LastRunOK   bool
	LastRunCmd  string
	SpinnerView string // pre-rendered spinner string from the bubbles spinner
	RunProgress float64
	Width       int
	Height      int
}

// Output renders the right panel: live command output and recent run history.
func Output(p Palette, props OutputProps) string {
	w, h := props.Width, props.Height

	label := "OUTPUT"
	if props.Streaming {
		label = "LIVE"
	}
	labelColor := p.Fg
	if props.Streaming {
		labelColor = p.StreamAccent
	}
	outputLabel := lipgloss.NewStyle().Foreground(labelColor).Bold(true).Render(label)
	var statusStr string
	switch {
	case props.Streaming:
		dot := "●"
		dotColor := p.StreamAccent
		if !props.LivePulse {
			dotColor = p.Red
		}
		statusStr = Style(dotColor, true).Render(dot) + " " +
			Style(p.FgDim, false).Render("ctrl+c stop · "+Truncate(props.LastRunCmd, 14))
	case props.Running:
		statusStr = lipgloss.NewStyle().Foreground(p.Yellow).
			Render(props.SpinnerView + " " + Truncate(props.LastRunCmd, 10))
	case props.HasLastRun && props.LastRunOK:
		statusStr = Style(p.Green, false).Render("✓ success")
	case props.HasLastRun:
		statusStr = Style(p.Red, false).Render("✗ error")
	}

	hGap := w - VisWidth(outputLabel) - VisWidth(statusStr) - 2
	if hGap < 0 {
		hGap = 0
	}
	headerRow := lipgloss.NewStyle().Width(w).Padding(0, 1).
		Render(outputLabel + strings.Repeat(" ", hGap) + statusStr)
	sepColor := p.Border
	if props.Streaming {
		sepColor = p.StreamAccent
	}
	sep := Style(sepColor, false).Render(strings.Repeat("─", w))

	var progressRow string
	progH := 0
	if props.Running || props.HasLastRun {
		progH = 2 // bar row + blank spacer
		var fillColor color.Color
		switch {
		case props.Streaming:
			fillColor = p.StreamAccent
		case props.Running:
			fillColor = p.Accent
		case props.LastRunOK:
			fillColor = p.Green
		default:
			fillColor = p.Red
		}
		progressRow = RenderProgressBar(p, w, props.RunProgress, fillColor)
	}

	const recentMax = 4
	recentRows := renderRecentRows(p, props.History, w, recentMax)
	recentH := len(recentRows)

	hintSep := SepLine(p, w)
	hintRow := renderOutputHintsRow(p, w)
	hintH := 2 // sep + hint row

	termH := h - 2 - progH - recentH - hintH // 2 = header + sep
	if termH < 1 {
		termH = 1
	}
	termRows := renderTermRows(p, props.Lines, w, termH)

	emptyRow := lipgloss.NewStyle().Width(w).Render("")
	allRows := []string{headerRow, sep}
	if props.Running || props.HasLastRun {
		allRows = append(allRows, progressRow, emptyRow)
	}
	allRows = append(allRows, termRows...)
	allRows = append(allRows, hintSep, hintRow, hintSep)
	allRows = append(allRows, recentRows...)

	return lipgloss.NewStyle().Width(w).Height(h).
		Render(strings.Join(allRows, "\n"))
}

// renderOutputHintsRow mirrors the sidebar hint row visually: accent-colored
// key glyphs followed by a dim label, laid out on a single line capped to w.
func renderOutputHintsRow(p Palette, w int) string {
	hints := [][2]string{{"ctrl+e", "expand"}, {"ctrl+c", "stop"}}
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1)
	avail := w - 2

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

func renderTermRows(p Palette, output []string, w, h int) []string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = lipgloss.NewStyle().Width(w).Render("")
	}

	if len(output) == 0 {
		rows[0] = lipgloss.NewStyle().Width(w).Padding(0, 1).
			Render(Style(p.FgDim, false).Render("run a command to see output…"))
		return rows
	}

	start := 0
	if len(output) > h {
		start = len(output) - h
	}
	contentW := w - 2 // subtract horizontal padding
	for i, l := range output[start:] {
		if i >= h {
			break
		}
		line := ansi.Truncate(colorizeLogLine(p, l), contentW, "")
		rows[i] = lipgloss.NewStyle().Width(w).Padding(0, 1).
			Render(line)
	}
	return rows
}

// renderRecentRows draws a compact, non-interactive table mirroring the
// History tab's columns (status dot, command, duration, started) but
// trimmed to fit in the output panel: only a header underline, no outer
// borders, no row striping. The slot is fixed-height so the live output
// area above stays stable: label (1) + header (1) + separator (1) +
// `max` data lines.
func renderRecentRows(p Palette, history []db.Run, w, max int) []string {
	label := lipgloss.NewStyle().Width(w).Padding(0, 1).
		Foreground(p.Fg).Bold(true).Render("RECENT")

	slotH := 1 + 1 + 1 + max // label + header + separator + data rows

	if len(history) == 0 {
		empty := lipgloss.NewStyle().Width(w).Padding(0, 1).
			Foreground(p.FgDim).Render("no runs yet")
		rows := []string{label, empty}
		for len(rows) < slotH {
			rows = append(rows, lipgloss.NewStyle().Width(w).Render(""))
		}
		return rows
	}

	headers := []string{"", "COMMAND", "DUR", "TIME"}
	count := len(history)
	if count > max {
		count = max
	}
	data := make([][]string, 0, count)
	// Compact command column width: total minus dot col, dur (~6), time (~8),
	// padding (~6) → leave the rest for the name. Lipgloss handles overflow,
	// but Truncate keeps the row visually predictable.
	nameW := w - 3 - 6 - 8 - 6
	if nameW < 6 {
		nameW = 6
	}
	for i := 0; i < count; i++ {
		r := history[i]
		data = append(data, []string{
			StatusDot(p, r.Status),
			Truncate(r.Command, nameW),
			r.DurationStr(),
			r.TimeStr(),
		})
	}

	borderStyle := lipgloss.NewStyle().Foreground(p.Border)
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(true).
		BorderRow(false).
		BorderColumn(false).
		Headers(headers...).
		Rows(data...).
		Width(w - 2).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				if col == 0 {
					return s.Width(3).Align(lipgloss.Center).Foreground(p.FgDim).Bold(true)
				}
				return s.Foreground(p.FgDim).Bold(true)
			}
			switch col {
			case 0:
				return s.Width(3).Align(lipgloss.Center)
			case 1:
				return s.Foreground(p.Fg)
			default:
				return s.Foreground(p.FgMuted)
			}
		})

	body := lipgloss.NewStyle().Padding(0, 1).Render(tbl.Render())
	rows := []string{label}
	rows = append(rows, strings.Split(body, "\n")...)
	for len(rows) < slotH {
		rows = append(rows, lipgloss.NewStyle().Width(w).Render(""))
	}
	if len(rows) > slotH {
		rows = rows[:slotH]
	}
	return rows
}
