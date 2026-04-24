package views

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/source"
)

// TagsPopupProps holds the data to render the tag-editor popup.
type TagsPopupProps struct {
	CmdName  string
	State    source.DocTagState
	Selected int // index into the flag list
}

// TagFlagItems is the canonical ordered list of flags the popup exposes.
// Kept exported so the handler and the view agree on indexes.
var TagFlagItems = []struct {
	Flag  string // tag name as it appears in the Makefile, e.g. "stream"
	Label string // display label
	Desc  string // short description shown next to the label
}{
	{"stream", "stream", "follow log output (tail -f, docker logs -f…)"},
	{"no-stream", "no-stream", "force non-stream even if auto-detected"},
	{"confirm", "confirm", "always ask before running (any env)"},
	{"no-confirm", "no-confirm", "never ask — even in staging/prod"},
}

// TagsPopup renders the popup contents.
func TagsPopup(p Palette, props TagsPopupProps) string {
	title := Style(p.Accent, true).Render("⌨  Tags")
	name := ""
	if props.CmdName != "" {
		name = ": " + props.CmdName
	}
	header := title + Style(p.FgDim, false).Render(name)

	rows := make([]string, 0, len(TagFlagItems)+4)
	rows = append(rows, header, "")

	for i, item := range TagFlagItems {
		checked := isFlagOn(props.State, item.Flag)
		var box string
		if checked {
			box = Style(p.Green, true).Render("[x]")
		} else {
			box = Style(p.FgDim, false).Render("[ ]")
		}
		labelFg := p.Fg
		if !checked {
			labelFg = p.FgDim
		}
		label := lipgloss.NewStyle().Foreground(labelFg).Bold(i == props.Selected).
			Render(padRight(item.Label, 12))
		desc := Style(p.FgMuted, false).Render(item.Desc)

		marker := " "
		if i == props.Selected {
			marker = Style(p.Accent, true).Render("›")
		}
		rows = append(rows, marker+" "+box+"  "+label+" "+desc)
	}

	rows = append(rows, "")
	shortcutBadge := RenderKeyBadge(p, props.State.Shortcut)
	shortcutRow := Style(p.FgDim, false).Render("shortcut: ") + shortcutBadge +
		Style(p.FgMuted, false).Render("   press ctrl+k to edit")
	rows = append(rows, shortcutRow)

	rows = append(rows, "", Style(p.FgMuted, false).Render("↑↓ nav  space/⏎ toggle  ctrl+k shortcut  esc close"))

	inner := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Background(p.BgPanel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Accent).
		Padding(1, 3).
		Render(inner)
}

func isFlagOn(s source.DocTagState, flag string) bool {
	switch flag {
	case "stream":
		return s.StreamSet && s.Stream
	case "no-stream":
		return s.StreamSet && !s.Stream
	case "confirm":
		return s.Confirm
	case "no-confirm":
		return s.NoConfirm
	}
	return false
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
