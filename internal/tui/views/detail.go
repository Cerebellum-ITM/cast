package views

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/db"
	"github.com/Cerebellum-ITM/cast/internal/source"
)

// CommandsProps holds data for the center commands-detail panel.
type CommandsProps struct {
	Cmd             *source.Command // nil if no commands loaded
	MakefileLines   []string
	MakefilePath    string
	MakefileOffset  int
	Running         bool
	RunProgress     float64
	Env             int // 0=local, 1=staging, 2=prod
	ShortcutEditing bool
	Width           int
	Height          int
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
			Padding(1, 2).Foreground(p.FgDim).Render("no commands")
		return noCmd + "\n" + SepLine(p, w), lipgloss.Height(noCmd) + 1
	}

	cmd := props.Cmd

	nameStr := Style(p.Accent, true).Render(cmd.Name)
	nameBadge := RenderKeyBadge(p, cmd.Shortcut)

	var envWarn string
	switch props.Env {
	case 1:
		envWarn = Style(p.Orange, false).Render("⚠ staging")
	case 2:
		envWarn = Style(p.Red, false).Render("⚠ production")
	}

	// Collect all chips (category + explicit tags + runtime flag chips). They
	// will be packed onto as many rows as needed so nothing overflows when the
	// center panel is narrow.
	var chips []string
	if cmd.Category != "" {
		chips = append(chips, RenderInlineTag(p, cmd.Category))
	}
	for _, t := range cmd.Tags {
		chips = append(chips, RenderInlineTag(p, t))
	}
	chips = append(chips, renderFlagChips(p, cmd)...)

	row1 := nameStr + "  " + nameBadge
	if envWarn != "" {
		row1 += "  " + envWarn
	}
	// avail is the max row width inside the center panel's 2-col indent.
	avail := w - 4
	if avail < 1 {
		avail = 1
	}
	chipRows := packChips(chips, avail)
	// Inline the first chip row into row1 when it still fits, so single-row
	// chip sets stay on the identity line (nameStr + badge + chips).
	if len(chipRows) > 0 && VisWidth(row1)+2+VisWidth(chipRows[0]) <= avail {
		row1 = row1 + "  " + chipRows[0]
		chipRows = chipRows[1:]
	}
	rows := []string{row1}
	// Each wrapped chip row gets a leading blank spacer so the colored chip
	// backgrounds don't visually touch the row above.
	for _, cr := range chipRows {
		rows = append(rows, "", cr)
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

	lines := []string{""}
	for _, r := range rows {
		lines = append(lines, Pad(2)+r)
	}
	lines = append(lines,
		Pad(2)+descRow,
		"",
		Pad(2)+cmdRow,
	)

	if props.ShortcutEditing {
		prompt := Style(p.Accent, true).Render("⌨ press a key to bind as shortcut") +
			Style(p.FgDim, false).Render("  ·  backspace clears · esc cancels")
		current := Style(p.FgDim, false).Render("current: ") + RenderKeyBadge(p, cmd.Shortcut)
		lines = append(lines, "", Pad(2)+prompt, Pad(2)+current)
	}

	if props.Running {
		bar := RenderProgressBar(p, w-4, props.RunProgress, p.Accent)
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
			Padding(1, 2).Foreground(p.FgDim).
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
func History(p Palette, records []db.Run, w, h int) string {
	titleRow := lipgloss.NewStyle().Width(w).Padding(0, 2).
		Background(p.BgPanel).Foreground(p.Fg).Bold(true).
		Render("HISTORY")
	sep := SepLine(p, w)

	var entries []string
	for _, r := range records {
		dot := StatusDot(p, r.Status)
		row := dot + " " + Style(p.Fg, true).Render(r.Command) +
			"  " + Style(p.FgDim, false).Render(r.DurationStr()) +
			"  " + Style(p.FgMuted, false).Render(r.TimeStr())
		entries = append(entries, lipgloss.NewStyle().Width(w).Padding(0, 2).
			Background(p.BgPanel).Render(row))
	}
	if len(entries) == 0 {
		entries = append(entries, lipgloss.NewStyle().Width(w).Padding(1, 2).
			Background(p.BgPanel).Foreground(p.FgDim).Render("no history yet"))
	}

	content := titleRow + "\n" + sep + "\n" + strings.Join(entries, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgPanel).Render(content)
}

// packChips lays out chips on as many rows as needed so no single row exceeds
// avail visible columns. Chips are joined with two spaces. An oversized chip
// takes its own row (it would overflow anywhere, so at least don't cascade).
func packChips(chips []string, avail int) []string {
	if len(chips) == 0 {
		return nil
	}
	var rows []string
	var cur []string
	curW := 0
	for _, c := range chips {
		cw := VisWidth(c)
		gap := 0
		if len(cur) > 0 {
			gap = 2
		}
		if len(cur) > 0 && curW+gap+cw > avail {
			rows = append(rows, strings.Join(cur, "  "))
			cur = cur[:0]
			curW = 0
			gap = 0
		}
		cur = append(cur, c)
		curW += gap + cw
	}
	if len(cur) > 0 {
		rows = append(rows, strings.Join(cur, "  "))
	}
	return rows
}

// renderFlagChips returns one inline chip per active Makefile flag tag on cmd.
// Colors are chosen to match the semantic already used elsewhere in the TUI:
// confirm/yellow mirrors the confirmation modal, no-confirm/green signals
// "skips the guard", stream/cyan matches the LIVE badge accent.
func renderFlagChips(p Palette, cmd *source.Command) []string {
	var chips []string
	chip := func(fg color.Color, text string) string {
		return lipgloss.NewStyle().
			Foreground(fg).Background(p.BgSelected).
			Padding(0, 1).Render(text)
	}
	if cmd.Confirm {
		chips = append(chips, chip(p.Yellow, "confirm"))
	}
	if cmd.NoConfirm {
		chips = append(chips, chip(p.Green, "no-confirm"))
	}
	if cmd.Stream {
		chips = append(chips, chip(p.StreamAccent, "stream"))
	}
	return chips
}

