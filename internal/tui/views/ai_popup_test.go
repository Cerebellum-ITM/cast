package views

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/ai"
)

func testPalette() Palette {
	c := lipgloss.Color("#ffffff")
	return Palette{
		Bg: c, BgPanel: c, BgDeep: c, BgSelected: c, BgHover: c,
		Fg: c, FgDim: c, FgMuted: c, Border: c, Accent: c,
		Cyan: c, Green: c, Yellow: c, Orange: c, Red: c, StreamAccent: c,
	}
}

func TestAIAnnotatePopup_AllModes(t *testing.T) {
	p := testPalette()
	plan := ai.Plan{
		Annotations: []ai.Annotation{{Name: "build", Desc: "Compile", Tags: []string{"build"}}},
		Skipped:     []ai.SkipReason{{Name: "x", Reason: "no recipe"}},
	}
	cases := []struct {
		name  string
		props AIAnnotatePopupProps
		want  string
	}{
		{"menu", AIAnnotatePopupProps{Mode: AIPopupMenu, Target: "build"}, "Annotate this target"},
		{"loading", AIAnnotatePopupProps{Mode: AIPopupLoading, Spinner: "*"}, "asking the model"},
		{"diff", AIAnnotatePopupProps{Mode: AIPopupDiff, Plan: plan, DiffViewport: "+## build:"}, "1 annotation(s)"},
		{"error", AIAnnotatePopupProps{Mode: AIPopupError, Error: "boom"}, "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := AIAnnotatePopup(p, tc.props)
			if !strings.Contains(out, tc.want) {
				t.Errorf("mode %s: output missing %q\n%s", tc.name, tc.want, out)
			}
		})
	}
}
