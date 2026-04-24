package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/runner"
	"github.com/Cerebellum-ITM/cast/internal/source"
)

// CommandsProps holds data for the center commands-detail panel.
type CommandsProps struct {
	Cmd            *source.Command // nil if no commands loaded
	MakefileLines  []string
	MakefilePath   string
	MakefileOffset int
	Running        bool
	RunProgress    float64
	Env            int // 0=local, 1=staging, 2=prod
	Width          int
	Height         int
}

// Commands renders the center panel when the commands tab is active.
func Commands(p Palette, props CommandsProps) string {
	hdr, hdrH := renderCommandHeader(p, props)
	previewH := props.Height - hdrH
	if previewH < 1 {
		previewH = 1
	}
	preview := renderMakefilePreview(p, props, previewH)
	return lipgloss.NewStyle().Width(props.Width).Height(props.Height).
		Background(p.BgDeep).
		Render(hdr + "\n" + preview)
}

func renderCommandHeader(p Palette, props CommandsProps) (string, int) {
	w := props.Width
	if props.Cmd == nil {
		noCmd := lipgloss.NewStyle().Width(w).Background(p.BgPanel).
			Padding(1, 2).Foreground(p.FgMuted).Render("no commands")
		return noCmd + "\n" + SepLine(p, w), lipgloss.Height(noCmd) + 1
	}

	cmd := props.Cmd

	nameStr := Style(p.Accent, true).Render(cmd.Name)
	nameBadge := lipgloss.NewStyle().
		Background(p.BgSelected).Foreground(p.Fg).Bold(true).
		Padding(0, 1).Render(capitalize(cmd.Name))
	row1 := nameStr + "  " + nameBadge
	if cmd.Category != "" {
		row1 += "  " + RenderInlineTag(p, cmd.Category)
	}
	for _, t := range cmd.Tags {
		row1 += "  " + RenderInlineTag(p, t)
	}
	switch props.Env {
	case 1:
		row1 += "  " + Style(p.Orange, false).Render("⚠ staging")
	case 2:
		row1 += "  " + Style(p.Red, false).Render("⚠ production")
	}

	descRow := Style(p.FgDim, false).Render(cmd.Desc)

	dollar := Style(p.FgDim, false).Render("$ ")
	makeCmd := Style(p.Cyan, false).Render("make " + cmd.Name)
	cmdBox := lipgloss.NewStyle().Background(p.BgDeep).Padding(0, 1).Render(dollar + makeCmd)

	var runBtn string
	if props.Running {
		runBtn = lipgloss.NewStyle().
			Background(p.BgSelected).Foreground(p.FgDim).Bold(true).
			Padding(0, 2).Render("…")
	} else {
		runBtn = lipgloss.NewStyle().
			Background(p.Accent).Foreground(p.BgDeep).Bold(true).
			Padding(0, 2).Render("⏎ Run")
	}

	cmdBoxW := VisWidth(cmdBox)
	btnW := VisWidth(runBtn)
	gapW := w - 4 - cmdBoxW - btnW
	if gapW < 1 {
		gapW = 1
	}
	cmdRow := cmdBox + strings.Repeat(" ", gapW) + runBtn

	lines := []string{
		"",
		Pad(2) + row1,
		Pad(2) + descRow,
		"",
		Pad(2) + cmdRow,
	}

	if props.Running {
		bar := RenderProgressBar(p, w-4, props.RunProgress)
		pct := fmt.Sprintf("%.0f%%", props.RunProgress*100)
		lines = append(lines, Pad(2)+bar)
		lines = append(lines, Pad(2)+Style(p.FgDim, false).Render(pct))
	}

	lines = append(lines, "")
	lines = append(lines, SepLine(p, w))

	content := strings.Join(lines, "\n")
	rendered := lipgloss.NewStyle().Width(w).Background(p.BgPanel).Render(content)
	return rendered, len(lines)
}

func renderMakefilePreview(p Palette, props CommandsProps, h int) string {
	w := props.Width
	if len(props.MakefileLines) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgDeep).
			Padding(1, 2).Foreground(p.FgMuted).
			Render("no makefile loaded")
	}

	pathRow := lipgloss.NewStyle().Width(w).Padding(0, 2).Background(p.BgDeep).
		Render(Style(p.FgDim, false).Render(props.MakefilePath) + "  " +
			Style(p.FgMuted, false).Render(fmt.Sprintf("%d lines", len(props.MakefileLines))))

	codeH := h - 1
	if codeH < 0 {
		codeH = 0
	}

	start := props.MakefileOffset
	if start >= len(props.MakefileLines) {
		start = 0
	}
	end := start + codeH
	if end > len(props.MakefileLines) {
		end = len(props.MakefileLines)
	}

	codeLines := make([]string, codeH)
	for i := start; i < end; i++ {
		lineNum := lipgloss.NewStyle().Foreground(p.FgMuted).Width(3).
			Render(fmt.Sprintf("%3d", i+1))
		codeLines[i-start] = "  " + lineNum + "  " + HighlightMakefileLine(p, props.MakefileLines[i])
	}

	code := strings.Join(codeLines, "\n")
	preview := lipgloss.NewStyle().Width(w).Height(codeH).Background(p.BgDeep).Render(code)
	return pathRow + "\n" + preview
}

// History renders the center panel when the history tab is active.
func History(p Palette, records []runner.RunRecord, w, h int) string {
	titleRow := lipgloss.NewStyle().Width(w).Padding(0, 2).
		Background(p.BgPanel).Foreground(p.FgDim).Bold(true).
		Render("HISTORY")
	sep := SepLine(p, w)

	var entries []string
	for _, r := range records {
		dot := StatusDot(p, r.Status)
		row := dot + " " + Style(p.Fg, true).Render(r.Command) +
			"  " + Style(p.FgDim, false).Render(r.Duration) +
			"  " + Style(p.FgMuted, false).Render(r.Time)
		entries = append(entries, lipgloss.NewStyle().Width(w).Padding(0, 2).
			Background(p.BgPanel).Render(row))
	}
	if len(entries) == 0 {
		entries = append(entries, lipgloss.NewStyle().Width(w).Padding(1, 2).
			Background(p.BgPanel).Foreground(p.FgMuted).Render("no history yet"))
	}

	content := titleRow + "\n" + sep + "\n" + strings.Join(entries, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgPanel).Render(content)
}

// EnvPane renders the center panel when the .env tab is active.
func EnvPane(p Palette, w, h int) string {
	title := Style(p.Accent, true).Render(".env")
	note := Style(p.FgDim, false).Render("env file viewer — coming soon")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgDeep).
		Padding(1, 2).Render(title + "\n" + note)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
