package views

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/runner"
)

// OutputProps holds all data needed to render the right output panel.
type OutputProps struct {
	Lines       []string
	History     []runner.RunRecord
	Running     bool
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

	outputLabel := lipgloss.NewStyle().Foreground(p.Fg).Bold(true).Render("OUTPUT")
	var statusStr string
	switch {
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
	headerRow := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgPanel).
		Render(outputLabel + strings.Repeat(" ", hGap) + statusStr)
	sep := SepLine(p, w)

	var progressRow string
	progH := 0
	if props.Running {
		progressRow = RenderProgressBar(p, w, props.RunProgress)
		progH = 1
	}

	const recentMax = 4
	recentRows := renderRecentRows(p, props.History, w, recentMax)
	recentH := len(recentRows)

	termH := h - 2 - progH - recentH // 2 = header + sep
	if termH < 1 {
		termH = 1
	}
	termRows := renderTermRows(p, props.Lines, w, termH)

	allRows := []string{headerRow, sep}
	if props.Running {
		allRows = append(allRows, progressRow)
	}
	allRows = append(allRows, termRows...)
	allRows = append(allRows, recentRows...)

	return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgDeep).
		Render(strings.Join(allRows, "\n"))
}

func renderTermRows(p Palette, output []string, w, h int) []string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = lipgloss.NewStyle().Width(w).Background(p.BgDeep).Render("")
	}

	if len(output) == 0 {
		rows[0] = lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgDeep).
			Render(Style(p.FgDim, false).Render("run a command to see output…"))
		return rows
	}

	start := 0
	if len(output) > h {
		start = len(output) - h
	}
	for i, l := range output[start:] {
		if i >= h {
			break
		}
		rows[i] = lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgDeep).
			Render(ColorOutputLine(p, l))
	}
	return rows
}

func renderRecentRows(p Palette, history []runner.RunRecord, w, max int) []string {
	label := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgPanel).
		Foreground(p.Fg).Bold(true).Render("RECENT")
	sep := SepLine(p, w)

	rows := []string{label, sep}

	for i, r := range history {
		if i >= max {
			break
		}
		dot := StatusDot(p, r.Status)
		name := Style(p.FgDim, false).Render(Truncate(r.Command, 10))
		dur := Style(p.FgDim, false).Render(r.Duration)
		ts := Style(p.FgDim, false).Render(r.Time)
		rows = append(rows, lipgloss.NewStyle().Width(w).Padding(0, 1).
			Background(p.BgPanel).Render(dot+" "+name+"  "+dur+"  "+ts))
	}
	for len(rows) < 2+max {
		rows = append(rows, lipgloss.NewStyle().Width(w).Background(p.BgPanel).Render(""))
	}
	return rows
}
