package views

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/db"
	"github.com/Cerebellum-ITM/cast/internal/source"
)

const masked = "••••••••"

// EnvSidebarProps holds data for the left panel of the .env tab.
type EnvSidebarProps struct {
	Vars          []source.EnvVar
	Selected      int
	Search        string
	SearchFocused bool
	ShowSecrets   bool
	Focused       bool
	Width         int
	Height        int
}

// EnvDetailProps holds data for the center-top panel (variable detail / edit).
type EnvDetailProps struct {
	Var            *source.EnvVar
	ShowSecrets    bool
	EditMode       bool
	EditBuffer     string
	NewMode        bool   // true when adding a brand-new variable
	NewKeyMode     bool   // true during key-name entry step of new-var flow
	NewKeyBuffer   string // key name typed so far
	NewSensitive   bool   // sensitive toggle state during new-var flow
	EnvName        string // e.g. "local", "staging", "prod"
	VarCount       int
	SensitiveCount int
	Filename       string
	Width          int
	Height         int
}

// EnvFilePreviewProps holds data for the center-bottom panel (.env file view).
type EnvFilePreviewProps struct {
	Lines    []string
	Filename string
	Width    int
	Height   int
}

// EnvHistoryProps holds data for the right panel (change history).
type EnvHistoryProps struct {
	Changes     []db.EnvChange
	Selected    int
	ShowSecrets bool
	Focused     bool
	Width       int
	Height      int
}

// ── Sidebar ───────────────────────────────────────────────────────────────────

// EnvSidebar renders the left panel: searchable list of env vars.
func EnvSidebar(p Palette, props EnvSidebarProps) string {
	w, h := props.Width, props.Height

	const (
		searchRows = 1
		sepRows    = 1
		hintSepR   = 1
		hintRows   = 2
	)
	listH := h - searchRows - sepRows - hintSepR - hintRows
	if listH < 0 {
		listH = 0
	}

	rows := []string{
		renderEnvSearch(p, props, w),
		SepLine(p, w),
	}
	rows = append(rows, renderEnvVarList(p, props, w, listH)...)
	rows = append(rows,
		SepLine(p, w),
		renderEnvSidebarHints(p, w, props.Focused),
	)

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgPanel).Render(content)
}

func renderEnvSearch(p Palette, props EnvSidebarProps, w int) string {
	icon := Style(p.FgDim, false).Render("⌕ ")
	available := w - VisWidth(icon) - 2

	var inputStr string
	if props.Search == "" && !props.SearchFocused {
		inputStr = Style(p.FgDim, false).Render("filter vars…")
	} else {
		inputStr = Style(p.Fg, false).Render(Truncate(props.Search, available))
		if props.SearchFocused {
			inputStr += Style(p.Accent, false).Render("▌")
		}
	}

	return lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgPanel).
		Render(icon + inputStr)
}

func renderEnvVarList(p Palette, props EnvSidebarProps, w, listH int) []string {
	const rowsPerItem = 2
	maxItems := listH / rowsPerItem

	rows := make([]string, listH)
	for i := range rows {
		rows[i] = lipgloss.NewStyle().Width(w).Background(p.BgPanel).Render("")
	}

	start := 0
	if props.Selected >= maxItems {
		start = props.Selected - maxItems + 1
	}

	slot := 0
	for i := start; i < len(props.Vars) && slot < maxItems; i++ {
		r1, r2 := renderEnvVarCard(p, props.Vars[i], i == props.Selected, props.Focused, w)
		rows[slot*rowsPerItem] = r1
		rows[slot*rowsPerItem+1] = r2
		slot++
	}
	return rows
}

func renderEnvVarCard(p Palette, v source.EnvVar, selected, focused bool, w int) (string, string) {
	bg := p.BgPanel
	if selected {
		if focused {
			bg = p.BgSelected
		} else {
			bg = p.BgDeep
		}
	}

	// Sensitive badge: compact [⚿] or alignment indent.
	const badgeW = 4
	var badge string
	if v.Sensitive {
		badge = Style(p.Yellow, false).Render("[⚿]") + " "
	} else {
		badge = strings.Repeat(" ", badgeW)
	}

	// Key name: sensitive items use Yellow fg; normal items use Fg.
	var keyStr string
	switch {
	case selected && focused:
		if v.Sensitive {
			keyStr = lipgloss.NewStyle().Foreground(p.Yellow).Bold(true).Render(v.Key)
		} else {
			keyStr = Style(p.Cyan, true).Render(v.Key)
		}
	case selected:
		keyStr = Style(p.FgDim, false).Render(v.Key)
	default:
		if v.Sensitive {
			keyStr = Style(p.Yellow, false).Render(v.Key)
		} else {
			keyStr = Style(p.Fg, false).Render(v.Key)
		}
	}

	contentW := w - 1
	var rowStyle lipgloss.Style
	if selected && focused {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(bg).
			BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(p.Accent).BorderBackground(bg)
	} else if selected {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(bg).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(p.FgMuted).BorderBackground(bg)
	} else {
		rowStyle = lipgloss.NewStyle().Width(contentW).Background(bg).
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(p.Border).BorderBackground(bg)
	}

	r1 := rowStyle.Render(badge + keyStr)
	r2 := rowStyle.Render("")
	return r1, r2
}

func renderEnvSidebarHints(p Palette, w int, focused bool) string {
	accentColor := p.Accent
	if !focused {
		accentColor = p.FgMuted
	}

	hints := [][2]string{
		{"↑↓", "nav"}, {"⏎", "edit"}, {"ctrl+a", "new"}, {"ctrl+s", "sensitive"}, {"s", "secrets"},
	}
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgPanel)

	var lines []string
	var rowParts []string
	rowUsed := 0
	avail := w - 2

	for _, h := range hints {
		key := lipgloss.NewStyle().Foreground(accentColor).Bold(focused).Render("[" + h[0] + "]")
		lbl := lipgloss.NewStyle().Foreground(p.FgMuted).Render(h[1])
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
	for len(lines) < 2 {
		lines = append(lines, rowStyle.Render(""))
	}
	return strings.Join(lines, "\n")
}

// ── Detail (center top) ───────────────────────────────────────────────────────

// envBadgeColor returns the badge background color matching the environment name.
func envBadgeColor(p Palette, name string) color.Color {
	switch name {
	case "staging":
		return p.Orange
	case "prod":
		return p.Red
	default:
		return p.Green
	}
}

// envDetailHeader builds the two-line header block (title row + path row + separator).
func envDetailHeader(p Palette, props EnvDetailProps) []string {
	w := props.Width

	// Line 1: ".env  [local]            12 variables"
	dotEnv := Style(p.Accent, true).Render(".env")
	badgeBg := envBadgeColor(p, props.EnvName)
	envBadge := lipgloss.NewStyle().
		Foreground(p.BgDeep).Background(badgeBg).
		Padding(0, 1).Render(props.EnvName)
	varCountStr := Style(p.FgDim, false).Render(fmt.Sprintf("%d variables", props.VarCount))

	hLeft := dotEnv + "  " + envBadge
	hGap := w - 4 - VisWidth(hLeft) - VisWidth(varCountStr)
	if hGap < 1 {
		hGap = 1
	}
	titleLine := Pad(2) + hLeft + strings.Repeat(" ", hGap) + varCountStr

	// Line 2: "~/path/.env    [N show secrets]"
	pathStr := Style(p.FgDim, false).Render(props.Filename)
	var secretsLabel string
	if props.ShowSecrets {
		secretsLabel = "hide secrets"
	} else {
		secretsLabel = fmt.Sprintf("%d show secrets", props.SensitiveCount)
	}
	secretsBtn := lipgloss.NewStyle().
		Background(p.BgSelected).Foreground(p.FgDim).
		Padding(0, 1).Render(secretsLabel)
	pGap := w - 4 - VisWidth(pathStr) - VisWidth(secretsBtn)
	if pGap < 1 {
		pGap = 1
	}
	pathLine := Pad(2) + pathStr + strings.Repeat(" ", pGap) + secretsBtn

	return []string{titleLine, pathLine, SepLine(p, w)}
}

// centeredHints renders a hint row centered within width w.
func centeredHints(p Palette, w int, editMode bool) string {
	var content string
	if editMode {
		content = Style(p.Accent, true).Render("[⏎]") +
			Style(p.FgMuted, false).Render(" save  ") +
			Style(p.FgMuted, false).Render("[esc]") +
			Style(p.FgMuted, false).Render(" cancel  ") +
			Style(p.FgMuted, false).Render("[ctrl+s]") +
			Style(p.FgMuted, false).Render(" sensitive")
	} else {
		content = Style(p.Accent, true).Render("[⏎]") +
			Style(p.FgMuted, false).Render(" edit  ") +
			Style(p.FgMuted, false).Render("[s]") +
			Style(p.FgMuted, false).Render(" secrets  ") +
			Style(p.FgMuted, false).Render("[ctrl+s]") +
			Style(p.FgMuted, false).Render(" sensitive")
	}
	return lipgloss.NewStyle().Width(w).Background(p.BgPanel).Align(lipgloss.Center).Render(content)
}

// EnvDetail renders the top portion of the center panel.
func EnvDetail(p Palette, props EnvDetailProps) string {
	w, h := props.Width, props.Height
	base := lipgloss.NewStyle().Width(w).Background(p.BgPanel)

	hdr := envDetailHeader(p, props)

	// New-variable flow: step 1 — enter key name.
	if props.NewKeyMode {
		header := Style(p.Accent, true).Render("NEW VARIABLE")
		keyLabel := Style(p.FgDim, false).Render("KEY    ")
		cursor := Style(p.Accent, false).Render("▌")
		keyInput := Style(p.Fg, false).Render(props.NewKeyBuffer) + cursor

		var sensRow string
		if props.NewSensitive {
			sensRow = Style(p.Yellow, false).Render("[⚿]") + " " +
				Style(p.Yellow, true).Render("sensitive") + "  " +
				Style(p.FgMuted, false).Render("ctrl+s to toggle")
		} else {
			sensRow = Style(p.FgDim, false).Render("[ ]") + " " +
				Style(p.FgDim, false).Render("not sensitive") + "  " +
				Style(p.FgMuted, false).Render("ctrl+s to toggle")
		}

		lines := append(hdr,
			"",
			Pad(2)+header,
			"",
			Pad(2)+keyLabel+keyInput,
			Pad(2)+sensRow,
			"",
			centeredHints(p, w, false),
		)
		return base.Height(h).Render(strings.Join(lines, "\n"))
	}

	// New-variable flow: step 2 — enter value (reuses EditMode).
	if props.NewMode && props.EditMode && props.Var != nil {
		header := Style(p.Accent, true).Render("NEW VARIABLE")
		keyLabel := Style(p.FgDim, false).Render("KEY    ")
		keyStr := Style(p.Cyan, true).Render(props.Var.Key)
		valLabel := Style(p.FgDim, false).Render("VALUE  ")
		cursor := Style(p.Accent, false).Render("▌")
		valInput := Style(p.Fg, false).Render(props.EditBuffer) + cursor

		var sensRow string
		if props.NewSensitive {
			sensRow = Style(p.Yellow, false).Render("[⚿]") + " " +
				Style(p.Yellow, true).Render("sensitive") + "  " +
				Style(p.FgMuted, false).Render("ctrl+s to toggle")
		} else {
			sensRow = Style(p.FgDim, false).Render("[ ]") + " " +
				Style(p.FgDim, false).Render("not sensitive") + "  " +
				Style(p.FgMuted, false).Render("ctrl+s to toggle")
		}

		lines := append(hdr,
			"",
			Pad(2)+header,
			"",
			Pad(2)+keyLabel+keyStr,
			Pad(2)+valLabel+valInput,
			Pad(2)+sensRow,
			"",
			centeredHints(p, w, true),
		)
		return base.Height(h).Render(strings.Join(lines, "\n"))
	}

	if props.Var == nil {
		lines := append(hdr, "")
		return base.Height(h).Render(strings.Join(lines, "\n"))
	}

	v := props.Var

	// Key + comment on the same line (matches image: "APP_ENV  Application environment").
	keyStr := Style(p.Accent, true).Render(v.Key)
	varLine := keyStr
	if v.Comment != "" {
		varLine += "  " + Style(p.FgDim, false).Render(v.Comment)
	} else if v.Sensitive {
		badge := lipgloss.NewStyle().
			Background(p.Yellow).Foreground(p.BgDeep).
			Padding(0, 1).Render("⚿ sensitive")
		varLine += "  " + badge
	}

	// Value in a rounded box.
	var displayVal string
	if props.EditMode {
		cursor := Style(p.Accent, false).Render("▌")
		displayVal = Style(p.Fg, false).Render(props.EditBuffer) + cursor
	} else if v.Sensitive && !props.ShowSecrets {
		displayVal = Style(p.FgDim, false).Render(masked)
	} else {
		displayVal = Style(p.Green, false).Render(v.Value)
	}
	// boxInnerW = panel width - 2(left margin) - border(1) - inner pad(1+1) - border(1)
	boxInnerW := w - 6
	if boxInnerW < 4 {
		boxInnerW = 4
	}
	valueBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).
		Background(p.BgDeep).Padding(0, 1).
		MarginLeft(2).
		Width(boxInnerW).
		Render(displayVal)

	lines := append(hdr,
		"",
		Pad(2)+varLine,
		valueBox,
		"",
		centeredHints(p, w, props.EditMode),
	)

	return base.Height(h).Render(strings.Join(lines, "\n"))
}

// ── File preview (center bottom) ──────────────────────────────────────────────

// EnvFilePreview renders the bottom portion of the center panel with line-numbered,
// syntax-highlighted .env file content.
func EnvFilePreview(p Palette, props EnvFilePreviewProps) string {
	w, h := props.Width, props.Height
	base := lipgloss.NewStyle().Width(w).Background(p.BgDeep)

	if len(props.Lines) == 0 {
		return base.Height(h).Padding(1, 2).Foreground(p.FgDim).Render("no .env file loaded")
	}

	pathRow := lipgloss.NewStyle().Width(w).Padding(0, 2).Background(p.BgDeep).
		Render(Style(p.FgDim, false).Render(props.Filename) + "  " +
			Style(p.FgMuted, false).Render(fmt.Sprintf("%d lines", len(props.Lines))))

	codeH := h - 1
	if codeH < 0 {
		codeH = 0
	}

	end := codeH
	if end > len(props.Lines) {
		end = len(props.Lines)
	}

	codeLines := make([]string, codeH)
	for i := 0; i < end; i++ {
		lineNum := lipgloss.NewStyle().Foreground(p.FgMuted).Width(3).
			Render(fmt.Sprintf("%3d", i+1))
		codeLines[i] = "  " + lineNum + "  " + HighlightEnvLine(p, props.Lines[i])
	}

	code := strings.Join(codeLines, "\n")
	preview := base.Height(codeH).Render(code)
	return pathRow + "\n" + preview
}

// ── History panel (right) ────────────────────────────────────────────────────

// EnvHistoryPanel renders the right panel: timeline of env var changes.
func EnvHistoryPanel(p Palette, props EnvHistoryProps) string {
	w, h := props.Width, props.Height

	titleRow := lipgloss.NewStyle().Width(w).Padding(0, 2).
		Background(p.BgPanel).Foreground(p.Fg).Bold(true).
		Render("ENV HISTORY")
	sep := SepLine(p, w)
	hint := renderEnvHistoryHints(p, w, props.Focused)

	// Reserve: title(1) + sep(1) + sep(1) + hint(1) = 4 rows
	entryH := h - 4
	if entryH < 0 {
		entryH = 0
	}
	const rowsPerEntry = 2
	maxEntries := entryH / rowsPerEntry

	var entriesStr string
	if len(props.Changes) == 0 {
		entriesStr = lipgloss.NewStyle().Width(w).Padding(1, 2).
			Background(p.BgPanel).Foreground(p.FgDim).
			Render("no changes recorded yet")
	} else {
		start := 0
		if props.Selected >= maxEntries {
			start = props.Selected - maxEntries + 1
		}

		var entries []string
		for i := start; i < len(props.Changes) && (i-start) < maxEntries; i++ {
			c := props.Changes[i]
			selected := i == props.Selected
			bg := p.BgPanel
			if selected {
				if props.Focused {
					bg = p.BgSelected
				} else {
					bg = p.BgDeep
				}
			}
			rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 2).Background(bg)

			// Row 1: key + timestamp
			var keyStr string
			switch {
			case selected && props.Focused:
				keyStr = Style(p.Cyan, true).Render(c.Key)
			case selected:
				keyStr = Style(p.FgDim, false).Render(c.Key)
			default:
				keyStr = Style(p.Fg, false).Render(c.Key)
			}
			timeStr := Style(p.FgMuted, false).Render(c.TimeStr())
			keyTimeW := VisWidth(keyStr) + VisWidth(timeStr) + 2
			pad := w - 4 - keyTimeW
			if pad < 1 {
				pad = 1
			}
			row1 := keyStr + strings.Repeat(" ", pad) + timeStr

			// Row 2: old → new
			var oldStr string
			if c.OldValue.Valid {
				val := c.OldValue.String
				if c.Sensitive && !props.ShowSecrets {
					val = masked
				}
				oldStr = Style(p.FgDim, false).Render(Truncate(val, (w-4)/2-2))
			} else {
				oldStr = Style(p.FgMuted, false).Render("(new)")
			}
			newVal := c.NewValue
			if c.Sensitive && !props.ShowSecrets {
				newVal = masked
			}
			newStr := Style(p.Green, false).Render(Truncate(newVal, (w-4)/2-2))
			arrow := Style(p.FgDim, false).Render(" → ")
			row2 := oldStr + arrow + newStr

			entries = append(entries, rowStyle.Render(row1), rowStyle.Render(row2))
		}
		entriesStr = strings.Join(entries, "\n")
	}

	content := titleRow + "\n" + sep + "\n" +
		entriesStr + "\n" +
		SepLine(p, w) + "\n" + hint
	return lipgloss.NewStyle().Width(w).Height(h).Background(p.BgPanel).Render(content)
}

func renderEnvHistoryHints(p Palette, w int, focused bool) string {
	accentColor := p.Accent
	if !focused {
		accentColor = p.FgMuted
	}

	hints := [][2]string{{"↑↓", "nav"}, {"r", "restore"}, {"tab", "switch"}}
	rowStyle := lipgloss.NewStyle().Width(w).Padding(0, 1).Background(p.BgPanel)

	var parts []string
	for _, h := range hints {
		key := lipgloss.NewStyle().Foreground(accentColor).Bold(focused).Render("[" + h[0] + "]")
		lbl := lipgloss.NewStyle().Foreground(p.FgMuted).Render(h[1])
		parts = append(parts, key+" "+lbl)
	}
	return rowStyle.Render(strings.Join(parts, "  "))
}
