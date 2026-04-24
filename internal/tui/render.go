package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui/views"
)

// Layout constants — all in terminal columns / rows.
const (
	sidebarW = 24 // includes right divider char
	outputW  = 28 // includes left divider char
	headerH  = 4  // 3 content rows (pill height) + 1 separator row
	statusH  = 1
)

// renderMain is the root renderer. When the confirm modal is active it renders
// the modal overlay instead of the normal three-panel layout.
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
	sts := views.StatusBar(p, len(m.commands), m.env.String(), m.width)
	full := hdr + "\n" + bdy + "\n" + sts

	if m.showConfirm && len(m.filtered) > 0 {
		box := views.Modal(p, m.filtered[m.selected].Name, m.env.String())
		return views.OverlayCenter(full, box)
	}

	if m.showOutputExpand {
		popupW := m.width - 8
		if popupW < 40 {
			popupW = 40
		}
		popupH := m.height - 4
		if popupH < 10 {
			popupH = 10
		}
		box := views.ExpandedOutput(p, m.output, m.outputExpandOff, popupW, popupH, m.lastRunCmd)
		return views.OverlayCenter(full, box)
	}

	if m.showMakefileExpand {
		popupW := m.width - 8
		if popupW < 40 {
			popupW = 40
		}
		popupH := m.height - 4
		if popupH < 10 {
			popupH = 10
		}
		var cmdName string
		if len(m.filtered) > 0 {
			cmdName = m.filtered[m.selected].Name
		}
		box := views.ExpandedMakefile(p, m.makefileExpandLines, m.makefileExpandOff, popupW, popupH, cmdName)
		return views.OverlayCenter(full, box)
	}

	return full
}

// ── Header ────────────────────────────────────────────────────────────────────

func (m Model) renderHeader(p views.Palette) string {
	pill := m.renderEnvPill(p)
	pillW := lipgloss.Width(pill)

	logo := views.Style(p.Accent, true).Render("⬡ cast")
	div := views.Style(p.Border, false).Render(" │ ")
	tabs := m.renderTabs(p)
	leftContent := logo + div + tabs

	leftW := m.width - pillW
	if leftW < 0 {
		leftW = 0
	}
	rowStyle := lipgloss.NewStyle().Width(leftW).Background(p.BgPanel).Padding(0, 1)
	leftBlock := rowStyle.Render("") + "\n" +
		rowStyle.Render(leftContent) + "\n" +
		rowStyle.Render("")

	rows123 := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, pill)
	sep := views.Style(p.Border, false).Render(strings.Repeat("─", m.width))
	return rows123 + "\n" + sep
}

func (m Model) renderTabs(p views.Palette) string {
	names := []string{"commands", "history", ".env", "theme"}
	var parts []string
	for i, n := range names {
		if Tab(i) == m.activeTab {
			parts = append(parts,
				lipgloss.NewStyle().
					Foreground(p.Accent).Bold(true).
					Padding(0, 1).Render(n))
		} else {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(p.FgDim).Padding(0, 1).Render(n))
		}
	}
	return strings.Join(parts, " ")
}

func (m Model) renderEnvPill(p views.Palette) string {
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
					Background(p.BgSelected).Padding(0, 1).
					Render("● "+b.label))
		} else {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(b.color).Padding(0, 1).
					Render("○ "+b.label))
		}
	}

	inner := strings.Join(parts, "")
	return lipgloss.NewStyle().
		Background(p.BgDeep).
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).
		Padding(0, 1).
		Render(inner)
}

// ── Body ──────────────────────────────────────────────────────────────────────

func (m Model) renderBody(p views.Palette, bodyH, centerW int) string {
	if m.activeTab == TabEnv {
		return m.renderEnvBody(p, bodyH, centerW)
	}

	sbInner := sidebarW - 1
	outInner := outputW - 1

	sidebar := views.Sidebar(p, views.SidebarProps{
		Commands:      m.filtered,
		Selected:      m.selected,
		Search:        m.searchInput.Value(),
		SearchFocused: m.searchInput.Focused(),
		Width:         sbInner,
		Height:        bodyH,
	})

	center := m.renderCenter(p, centerW, bodyH)

	output := views.Output(p, views.OutputProps{
		Lines:       m.output,
		History:     m.history,
		Running:     m.running,
		HasLastRun:  m.hasLastRun,
		LastRunOK:   m.lastRunOK,
		LastRunCmd:  m.lastRunCmd,
		SpinnerView: m.spinner.View(),
		RunProgress: m.runProgress,
		Width:       outInner,
		Height:      bodyH,
	})

	divStyle := views.Style(p.Border, false)
	divCol := divStyle.Render(strings.Repeat("│\n", bodyH-1) + "│")

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divCol, center, divCol, output)
}

// envSidebarW is the wider sidebar used by the .env tab (includes right divider).
const envSidebarW = 37

func (m Model) renderEnvBody(p views.Palette, bodyH, totalW int) string {
	// Recompute center width using the wider env sidebar.
	envCenterW := totalW + (sidebarW - envSidebarW)
	if envCenterW < 10 {
		envCenterW = 10
	}
	sbInner := envSidebarW - 1
	outInner := outputW - 1

	vars := filterEnvVars(m.envFile, m.envSearchInput.Value())

	sidebar := views.EnvSidebar(p, views.EnvSidebarProps{
		Vars:          vars,
		Selected:      m.selectedEnvKey,
		Search:        m.envSearchInput.Value(),
		SearchFocused: m.envSearchInput.Focused(),
		ShowSecrets:   m.showSecrets,
		Focused:       m.envFocus == 0,
		Width:         sbInner,
		Height:        bodyH,
	})

	center := m.renderEnvCenter(p, envCenterW, bodyH, vars)

	history := views.EnvHistoryPanel(p, views.EnvHistoryProps{
		Changes:     m.envHistoryItems,
		Selected:    m.envHistorySel,
		ShowSecrets: m.showSecrets,
		Focused:     m.envFocus == 1,
		Width:       outInner,
		Height:      bodyH,
	})

	divStyle := views.Style(p.Border, false)
	divCol := divStyle.Render(strings.Repeat("│\n", bodyH-1) + "│")

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divCol, center, divCol, history)
}

func (m Model) renderEnvCenter(p views.Palette, w, h int, vars []source.EnvVar) string {
	detailH := h * 2 / 5
	if detailH < 4 {
		detailH = 4
	}
	previewH := h - detailH - 1
	if previewH < 2 {
		previewH = 2
	}

	var selectedVar *source.EnvVar
	if len(vars) > 0 && m.selectedEnvKey < len(vars) {
		v := vars[m.selectedEnvKey]
		selectedVar = &v
	}

	detail := views.EnvDetail(p, views.EnvDetailProps{
		Var:          selectedVar,
		ShowSecrets:  m.showSecrets,
		EditMode:     m.envEditMode,
		EditBuffer:   m.envEditBuffer,
		NewMode:      m.envNewMode,
		NewKeyMode:   m.envNewKeyMode,
		NewKeyBuffer: m.envNewKeyBuffer,
		NewSensitive: m.envNewSensitive,
		Width:        w,
		Height:       detailH,
	})

	sep := views.SepLine(p, w)

	var rawLines []string
	var filename string
	if m.envFile != nil {
		rawLines = m.envFile.RawLines
		filename = m.envFile.Filename
	}
	preview := views.EnvFilePreview(p, views.EnvFilePreviewProps{
		Lines:    rawLines,
		Filename: filename,
		Width:    w,
		Height:   previewH,
	})

	return detail + "\n" + sep + "\n" + preview
}

func (m Model) renderCenter(p views.Palette, w, h int) string {
	switch m.activeTab {
	case TabHistory:
		return views.History(p, m.history, w, h)
	default:
		var cmd *source.Command
		if len(m.filtered) > 0 {
			c := m.filtered[m.selected]
			cmd = &c
		}
		return views.Commands(p, views.CommandsProps{
			Cmd:            cmd,
			MakefileLines:  m.makefileLines,
			MakefilePath:   m.makefilePath,
			MakefileOffset: m.makefileOffset,
			Running:        m.running,
			RunProgress:    m.runProgress,
			Env:            int(m.env),
			Width:          w,
			Height:         h,
		})
	}
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// clampToHeight trims a multi-line string to at most h rows, preventing any
// over-tall panel from pushing the status bar off screen.
func clampToHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= h {
		return s
	}
	return strings.Join(lines[:h], "\n")
}
