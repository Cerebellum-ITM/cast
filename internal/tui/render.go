package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui/views"
)

// Layout constants — all in terminal columns / rows.
const (
	headerH = 4 // 3 content rows (pill height) + 1 separator row
	statusH = 1
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
	sidebarW := m.sidebarPanelW()
	outputW := m.outputPanelW()
	centerW := m.width - sidebarW - outputW
	if !m.showCenter {
		centerW = 0
	} else if centerW < 10 {
		centerW = 10
	}

	hdr := m.renderHeader(p)
	bdy := m.renderBody(p, bodyH, centerW)
	sts := views.StatusBar(p, len(m.commands), m.makefilePath, m.width)
	// No explicit frame Background: the cast UI is intentionally
	// "transparent" over the terminal so structural cells (panel
	// interiors, dividers, header gaps, hint rows) inherit the user's
	// terminal background. Explicit bg is reserved for purposeful
	// emphasis only — selected items (BgSelected), shortcut badges,
	// and tag chips. Anything else MUST stay bg-less so the theme
	// composes cleanly with whatever terminal palette the user runs.
	//
	// Each band gets a hard fit to its budgeted slot. Without this, a
	// header that grows past `headerH` rows (because pills overflow on a
	// narrow terminal) or a body line that exceeds `m.width` (because the
	// terminal wraps it) would shove the status bar off-screen. The
	// status bar is the most important persistent surface — it must
	// remain visible at every terminal size.
	hdr = fitFrame(hdr, m.width, headerH)
	bdy = fitFrame(bdy, m.width, bodyH)
	sts = fitFrame(sts, m.width, statusH)
	full := hdr + "\n" + bdy + "\n" + sts

	if m.showPicker {
		popupW := m.width - 12
		if popupW < 50 {
			popupW = 50
		}
		if popupW > 90 {
			popupW = 90
		}
		popupH := m.height - 6
		if popupH < 14 {
			popupH = 14
		}
		var filter string
		if m.pickerStep < len(m.pickerCmd.Picks) {
			filter = m.pickerCmd.Picks[m.pickerStep].Filter
		}
		box := views.Picker(p, views.PickerProps{
			CmdName:    m.pickerCmd.Name,
			StepIdx:    m.pickerStep,
			StepCount:  len(m.pickerCmd.Picks),
			BaseDir:    m.pickerBase,
			Filter:     filter,
			Search:     m.pickerSearch,
			Entries:    m.pickerEntries,
			Cursor:     m.pickerCursor,
			Selections: m.pickerSelections,
			IconStyle:  m.iconStyle,
			Width:      popupW,
			Height:     popupH,
		})
		return views.OverlayCenter(full, box)
	}

	if m.showConfirm && len(m.filtered) > 0 {
		box := views.Modal(p, m.filtered[m.selected].Name, m.env.String(), m.confirmModalSel)
		return views.OverlayCenter(full, box)
	}

	if m.showTagsPopup {
		box := views.TagsPopup(p, views.TagsPopupProps{
			CmdName:    m.tagsPopupName,
			State:      m.tagsPopupState,
			Selected:   m.tagsPopupSel,
			Editing:    m.tagsEditing,
			EditBuffer: m.tagsEditBuffer,
		})
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
	modePill := m.renderModePill(p)
	noticePill := m.renderNoticePill(p)

	rightW := lipgloss.Width(pill) + lipgloss.Width(modePill)
	if rightW > 0 {
		rightW += 1 // gap between mode and env pills
	}
	if noticePill != "" {
		rightW += lipgloss.Width(noticePill) + 1 // pill + gap
	}

	logo := views.Style(p.Accent, true).Render("⬡ cast")
	div := views.Style(p.Border, false).Render(" │ ")
	tabs := m.renderTabs(p)
	leftContent := logo + div + tabs

	leftW := m.width - rightW
	if leftW < 0 {
		leftW = 0
	}
	rowStyle := lipgloss.NewStyle().Width(leftW).Padding(0, 1)
	leftBlock := rowStyle.Render("") + "\n" +
		rowStyle.Render(leftContent) + "\n" +
		rowStyle.Render("")

	// Order on the right edge (left → right): notice · mode · env. The
	// notice sits closest to the tabs so updates appear "near" the action,
	// while the env pill stays anchored to the far right where the user
	// expects it.
	var rightBlock string
	if noticePill != "" {
		rightBlock = lipgloss.JoinHorizontal(lipgloss.Top,
			noticePill, " ", modePill, " ", pill)
	} else {
		rightBlock = lipgloss.JoinHorizontal(lipgloss.Top, modePill, " ", pill)
	}
	rows123 := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, rightBlock)
	sep := views.Style(p.Border, false).Render(strings.Repeat("─", m.width))
	return rows123 + "\n" + sep
}

// renderNoticePill renders the transient toast as a rounded-border pill in
// the header. Empty notice → empty string so callers can skip layout work.
// Color matches the kind: success=green, error=red, info=accent.
func (m Model) renderNoticePill(p views.Palette) string {
	if m.notice == "" {
		return ""
	}
	fg := p.Accent
	glyph := "·"
	switch views.NoticeKind(m.noticeKind) {
	case views.NoticeSuccess:
		fg = p.Green
		glyph = "✓"
	case views.NoticeError:
		fg = p.Red
		glyph = "⚠"
	}
	// Cap width so a long notice doesn't shove the env/mode pills off-screen.
	const maxNoticeW = 48
	label := m.notice
	if lipgloss.Width(label) > maxNoticeW {
		label = views.Truncate(label, maxNoticeW)
	}
	inner := lipgloss.NewStyle().Foreground(fg).Bold(true).
		Padding(0, 1).Render(glyph + " " + label)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).
		Padding(0, 1).
		Render(inner)
}

// renderModePill renders a two-state badge (SINGLE vs CHAIN) next to the env
// pill. Accent color distinguishes chain mode from single.
func (m Model) renderModePill(p views.Palette) string {
	label := "SINGLE"
	fg := p.Cyan
	if m.mode == ModeChain {
		label = "CHAIN"
		fg = p.Orange
	}
	inner := lipgloss.NewStyle().Foreground(fg).Bold(true).
		Padding(0, 1).Render("⛓ " + label)
	// No bg on the pill — only the rounded border + colored label define
	// it visually, so the pill stays consistent with the terminal's bg.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).
		Padding(0, 1).
		Render(inner)
}

func (m Model) renderTabs(p views.Palette) string {
	names := []string{"commands", "history", ".env", "theme", "library"}
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
	// Pull from the active palette so each theme's env pill matches the rest
	// of the UI (Nord uses muted polar tones, Catppuccin uses pastel mocha,
	// Dracula uses neon). Hardcoded hex would force one theme's flavour
	// onto the other two.
	btns := []envBtn{
		{"DEV", p.Green},
		{"STG", p.Orange},
		{"PRD", p.Red},
	}

	var parts []string
	for i, b := range btns {
		if int(m.env) == i {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(b.color).Bold(true).
					Padding(0, 1).
					Render("● "+b.label))
		} else {
			parts = append(parts,
				lipgloss.NewStyle().Foreground(b.color).Padding(0, 1).
					Render("○ "+b.label))
		}
	}

	inner := strings.Join(parts, "")
	// Same as renderModePill: no bg, just rounded border + colored
	// indicators inside.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).
		Padding(0, 1).
		Render(inner)
}

// ── Body ──────────────────────────────────────────────────────────────────────

func (m Model) renderBody(p views.Palette, bodyH, centerW int) string {
	if m.activeTab == TabEnv {
		return m.renderEnvBody(p, bodyH, centerW)
	}
	if m.activeTab == TabLibrary {
		return m.renderLibraryBody(p, bodyH)
	}
	if m.activeTab == TabHistory {
		return m.renderHistoryBody(p, bodyH)
	}

	sbInner := m.sidebarPanelW() - 1
	outInner := m.outputPanelW() - 1

	var queueCmds []string
	curStep := 0
	if len(m.chainCommands) > 1 {
		queueCmds = m.chainCommands
		curStep = m.chainStepIdx
	}
	var lastRunCmds []string
	var lastRunIsPick bool
	if m.hasRerunCard() {
		lastRunCmds = m.lastRunCommands
		// Mark as pick when we have cached extras (only pick commands produce them).
		lastRunIsPick = len(m.lastRunExtraVars) > 0
	}
	sidebar := views.Sidebar(p, views.SidebarProps{
		Commands:       m.filtered,
		Selected:       m.selected,
		Search:         m.searchInput.Value(),
		SearchFocused:  m.searchInput.Focused(),
		Width:          sbInner,
		Height:         bodyH,
		Mode:           int(m.mode),
		Chains:         m.chains,
		ChainSel:       m.chainSel,
		ChainBuilder:   m.chainBuilder,
		ChainChecked:   m.chainChecked,
		QueueCommands:  queueCmds,
		CurrentStep:    curStep,
		LastRunCmds:    lastRunCmds,
		LastRunIsPick:  lastRunIsPick,
		LastRunFocused: m.rerunFocused,
	})

	var center string
	if m.showCenter {
		center = m.renderCenter(p, centerW, bodyH)
	}

	output := views.Output(p, views.OutputProps{
		Lines:       m.output,
		History:     m.history,
		Running:     m.running,
		Streaming:   m.streaming,
		LivePulse:   m.livePulse,
		HasLastRun:  m.hasLastRun,
		LastRunOK:   m.lastRunOK,
		LastRunCmd:  m.lastRunCmd,
		SpinnerView: m.spinner.View(),
		RunProgress: m.runProgress,
		Width:       outInner,
		Height:      bodyH,
	})

	// Dividers render bg-less so they sit directly on the terminal's
	// background — same as every other structural cell.
	divStyle := lipgloss.NewStyle().Foreground(p.Border)
	divCol := divStyle.Render(strings.Repeat("│\n", bodyH-1) + "│")

	if !m.showCenter {
		return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divCol, output)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divCol, center, divCol, output)
}

// envSidebarW is the wider sidebar used by the .env tab (includes right divider).
const envSidebarW = 37

// renderLibraryBody draws the library tab full-width: no sidebar, no
// output panel. Browsing snippets benefits from extra horizontal space
// for the preview pane, and the commands sidebar / output stream are
// irrelevant while the user is curating their snippet collection.
func (m Model) renderLibraryBody(p views.Palette, bodyH int) string {
	snippets := make([]views.LibrarySnippet, 0, len(m.libraryFiltered))
	for _, s := range m.libraryFiltered {
		snippets = append(snippets, views.LibrarySnippet{
			Name: s.Name,
			Desc: s.Desc,
			Tags: s.Tags,
			Body: s.Body,
		})
	}
	return views.Library(p, views.LibraryProps{
		Snippets:      snippets,
		Selected:      m.librarySel,
		Search:        m.librarySearchInput.Value(),
		SearchFocused: m.librarySearchInput.Focused(),
		Error:         m.libraryError,
		Feedback:      m.libraryFeedback,
		ConfirmDelete: m.libraryConfirmDelete,
		IconStyle:     m.iconStyle,
		Width:         m.width,
		Height:        bodyH,
	})
}

// renderHistoryBody draws the history tab without a sidebar: the runs/chains
// table is the primary surface and takes the freed horizontal space, while
// the live output panel stays on the right. Up/Down moves the row cursor;
// Enter on a row re-runs the corresponding command (single mode) or replays
// the chain (chain mode) — see handleKey's TabHistory branch.
func (m Model) renderHistoryBody(p views.Palette, bodyH int) string {
	outW := m.outputPanelW()
	if !m.showCenter {
		outW = 0
	}
	tableW := m.width - outW - 1 // -1 for the divider column
	if outW == 0 {
		tableW = m.width
	}
	if tableW < 10 {
		tableW = 10
	}

	selected := m.historySel
	if m.mode == ModeChain {
		selected = m.chainHistorySel
	}
	tbl := views.History(p, views.HistoryProps{
		Records:   m.history,
		Cmds:      m.commands,
		Mode:      int(m.mode),
		ChainRuns: m.chainRuns,
		Selected:  selected,
		Width:     tableW,
		Height:    bodyH,
	})

	if outW == 0 {
		return tbl
	}

	output := views.Output(p, views.OutputProps{
		Lines:       m.output,
		History:     m.history,
		Running:     m.running,
		Streaming:   m.streaming,
		LivePulse:   m.livePulse,
		HasLastRun:  m.hasLastRun,
		LastRunOK:   m.lastRunOK,
		LastRunCmd:  m.lastRunCmd,
		SpinnerView: m.spinner.View(),
		RunProgress: m.runProgress,
		Width:       outW - 1,
		Height:      bodyH,
	})
	divStyle := lipgloss.NewStyle().Foreground(p.Border)
	divCol := divStyle.Render(strings.Repeat("│\n", bodyH-1) + "│")
	return lipgloss.JoinHorizontal(lipgloss.Top, tbl, divCol, output)
}

func (m Model) renderEnvBody(p views.Palette, bodyH, totalW int) string {
	_ = totalW
	envCenterW := m.width - envSidebarW - m.outputPanelW()
	if !m.showCenter {
		envCenterW = 0
	} else if envCenterW < 10 {
		envCenterW = 10
	}
	sbInner := envSidebarW - 1
	outInner := m.outputPanelW() - 1

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

	var center string
	if m.showCenter {
		center = m.renderEnvCenter(p, envCenterW, bodyH, vars)
	}

	history := views.EnvHistoryPanel(p, views.EnvHistoryProps{
		Changes:     m.envHistoryItems,
		Selected:    m.envHistorySel,
		ShowSecrets: m.showSecrets,
		Focused:     m.envFocus == 1,
		Width:       outInner,
		Height:      bodyH,
	})

	// Dividers render bg-less so they sit directly on the terminal's
	// background — same as every other structural cell.
	divStyle := lipgloss.NewStyle().Foreground(p.Border)
	divCol := divStyle.Render(strings.Repeat("│\n", bodyH-1) + "│")

	if !m.showCenter {
		return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divCol, history)
	}
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

	var totalVarCount, sensitiveCount int
	var envFilename string
	if m.envFile != nil {
		totalVarCount = len(m.envFile.Vars)
		envFilename = m.envFile.Filename
		for _, v := range m.envFile.Vars {
			if v.Sensitive {
				sensitiveCount++
			}
		}
	}

	detail := views.EnvDetail(p, views.EnvDetailProps{
		Var:            selectedVar,
		ShowSecrets:    m.showSecrets,
		EditMode:       m.envEditMode,
		EditBuffer:     m.envEditBuffer,
		NewMode:        m.envNewMode,
		NewKeyMode:     m.envNewKeyMode,
		NewKeyBuffer:   m.envNewKeyBuffer,
		NewSensitive:   m.envNewSensitive,
		EnvName:        m.env.String(),
		VarCount:       totalVarCount,
		SensitiveCount: sensitiveCount,
		Filename:       envFilename,
		Width:          w,
		Height:         detailH,
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
	case TabTheme:
		opts := make([]views.ThemeOption, 0, len(themeOrder))
		for _, t := range themeOrder {
			opts = append(opts, views.ThemeOption{
				Key:      string(t),
				Label:    themeLabel(t),
				Preview:  paletteFor(t, m.env),
				IsActive: t == m.theme,
				Saved:    t == m.savedTheme,
			})
		}
		return views.Theme(p, views.ThemeProps{
			Options:    opts,
			Selected:   m.themeTabSel,
			LocalPath:  config.LocalPath(),
			WriteError: m.themeError,
			Width:      w,
			Height:     h,
		})
	case TabHistory:
		return views.History(p, views.HistoryProps{
			Records:   m.history,
			Cmds:      m.commands,
			Mode:      int(m.mode),
			ChainRuns: m.chainRuns,
			Width:     w,
			Height:    h,
		})
	default:
		var cmd *source.Command
		if len(m.filtered) > 0 {
			c := m.filtered[m.selected]
			cmd = &c
		}
		return views.Commands(p, views.CommandsProps{
			Cmd:             cmd,
			MakefileLines:   m.makefileLines,
			MakefilePath:    m.makefilePath,
			MakefileOffset:  m.makefileOffset,
			Running:         m.running,
			RunProgress:     m.runProgress,
			Env:             int(m.env),
			ShortcutEditing: m.shortcutEditMode,
			Width:           w,
			Height:          h,
		})
	}
}

// themeLabel returns the display name shown in the Theme tab. Lives here
// (and not in the views package) so views stay decoupled from theme IDs.
func themeLabel(t config.Theme) string {
	switch t {
	case config.ThemeCatppuccin:
		return "Catppuccin"
	case config.ThemeDracula:
		return "Dracula"
	case config.ThemeNord:
		return "Nord"
	}
	return string(t)
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

// fitFrame forces s to render in exactly w columns by h rows. Lines longer
// than w are ANSI-truncated (preserving SGR sequences); shorter outputs
// gain blank padding rows so downstream concatenation aligns. Fewer
// columns than the natural content width clip the right edge instead of
// wrapping — wrapping would cascade into more vertical rows and push the
// status bar off-screen on narrow terminals.
func fitFrame(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		if lipgloss.Width(line) > w {
			lines[i] = ansi.Truncate(line, w, "")
		}
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
