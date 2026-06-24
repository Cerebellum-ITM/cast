package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/ai"
)

// AIPopupMode is the visual state of the AI-annotate popup. The model owns the
// transitions; the view is a pure function of the current mode.
type AIPopupMode int

const (
	AIPopupMenu AIPopupMode = iota // three-option menu (t / a / A)
	AIPopupLoading                 // spinner while the LLM is queried
	AIPopupDiff                    // proposed diff, awaiting apply/cancel
	AIPopupError                   // error or "no annotations" message
)

// AIAnnotatePopupProps is the explicit input for AIAnnotatePopup. DiffViewport
// is the already-windowed, already-coloured diff string the model slices to
// fit the popup height — the view never re-renders it.
type AIAnnotatePopupProps struct {
	Mode         AIPopupMode
	Target       string // selected target name, labelled in the menu
	Plan         ai.Plan
	Error        string
	Spinner      string
	DiffViewport string
}

// AIAnnotatePopup renders the ctrl+i popup contents.
func AIAnnotatePopup(p Palette, props AIAnnotatePopupProps) string {
	rows := []string{Style(p.Accent, true).Render("✦ AI annotate"), ""}

	switch props.Mode {
	case AIPopupMenu:
		rows = append(rows, aiMenuRows(p, props.Target)...)
	case AIPopupLoading:
		rows = append(rows,
			props.Spinner+" "+Style(p.Fg, false).Render("asking the model…"),
			"",
			Style(p.FgMuted, false).Render("esc  cancel"))
	case AIPopupDiff:
		summary := fmt.Sprintf("%d annotation(s)", len(props.Plan.Annotations))
		if len(props.Plan.Skipped) > 0 {
			summary += fmt.Sprintf(" · %d skipped", len(props.Plan.Skipped))
		}
		rows = append(rows, Style(p.FgDim, false).Render(summary), "")
		rows = append(rows, props.DiffViewport, "")
		rows = append(rows, Style(p.FgMuted, false).
			Render("↑↓ pgup pgdn scroll   ⏎ apply   esc cancel"))
	case AIPopupError:
		rows = append(rows,
			Style(p.Red, false).Render(props.Error),
			"",
			Style(p.FgMuted, false).Render("esc  close"))
	}

	return lipgloss.NewStyle().
		Background(p.BgPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Accent).
		Padding(1, 3).
		Render(strings.Join(rows, "\n"))
}

func aiMenuRows(p Palette, target string) []string {
	type opt struct{ key, label string }
	tLabel := "Annotate this target"
	if target != "" {
		tLabel += " (" + target + ")"
	}
	opts := []opt{
		{"t", tLabel},
		{"a", "Annotate targets without a doc-line"},
		{"A", "Annotate all (overwrite existing doc-lines)"},
	}
	rows := make([]string, 0, len(opts)+2)
	for _, o := range opts {
		badge := RenderKeyBadge(p, o.key)
		rows = append(rows, badge+"  "+Style(p.Fg, false).Render(o.label))
	}
	rows = append(rows, "", Style(p.FgMuted, false).Render("esc  cancel"))
	return rows
}
