package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/tui/views"
)

func paletteFor(theme config.Theme, env config.Env) views.Palette {
	p := basePalette(theme)
	switch env {
	case config.EnvStaging:
		p.Accent = p.Orange
	case config.EnvProd:
		p.Accent = p.Red
	}
	return p
}

func basePalette(theme config.Theme) views.Palette {
	switch theme {
	case config.ThemeDracula:
		return views.Palette{
			// Surfaces
			Bg:         lipgloss.Color("#282A36"),
			BgPanel:    lipgloss.Color("#2D2F3F"),
			BgDeep:     lipgloss.Color("#1E1F29"),
			BgHover:    lipgloss.Color("#383A4A"),
			BgSelected: lipgloss.Color("#44475A"),

			// Foreground
			Fg:      lipgloss.Color("#F8F8F2"),
			FgDim:   lipgloss.Color("#7A88B8"),
			FgMuted: lipgloss.Color("#5A6388"),

			// Border
			Border: lipgloss.Color("#3B3D4D"),

			Accent: lipgloss.Color("#BD93F9"),
			Cyan:   lipgloss.Color("#8BE9FD"),
			Green:  lipgloss.Color("#50FA7B"),
			Yellow: lipgloss.Color("#F1FA8C"),
			Orange: lipgloss.Color("#FFB86C"),
			Red: lipgloss.Color(
				"#FF6E6E",
			),
			StreamAccent: lipgloss.Color("#FF79C6"),
		}

	case config.ThemeNord:
		return views.Palette{
			Bg:         lipgloss.Color("#2E3440"),
			BgPanel:    lipgloss.Color("#3B4252"),
			BgDeep:     lipgloss.Color("#242933"),
			BgHover:    lipgloss.Color("#434C5E"),
			BgSelected: lipgloss.Color("#4C566A"),

			Fg:      lipgloss.Color("#ECEFF4"),
			FgDim:   lipgloss.Color("#8893A8"),
			FgMuted: lipgloss.Color("#677084"),

			Border: lipgloss.Color("#3B4252"),

			Accent:       lipgloss.Color("#88C0D0"),
			Cyan:         lipgloss.Color("#8FBCBB"),
			Green:        lipgloss.Color("#A3BE8C"),
			Yellow:       lipgloss.Color("#EBCB8B"),
			Orange:       lipgloss.Color("#D08770"),
			Red:          lipgloss.Color("#BF616A"),
			StreamAccent: lipgloss.Color("#B48EAD"),
		}

	default: // Catppuccin Mocha
		return views.Palette{
			// Pila oficial: crust → mantle → base → surface0 → surface1 → surface2
			Bg:         lipgloss.Color("#1E1E2E"), // base
			BgPanel:    lipgloss.Color("#181825"), // mantle
			BgDeep:     lipgloss.Color("#11111B"), // crust
			BgHover:    lipgloss.Color("#313244"), // surface0
			BgSelected: lipgloss.Color("#45475A"), // surface1

			Fg:      lipgloss.Color("#CDD6F4"), // text
			FgDim:   lipgloss.Color("#9399B2"), // overlay2 — antes 6C7086, sube contraste
			FgMuted: lipgloss.Color("#6C7086"), // overlay0 — separado de BgSelected y Border

			Border: lipgloss.Color("#313244"), // surface0

			Accent:       lipgloss.Color("#CBA6F7"), // mauve
			Cyan:         lipgloss.Color("#89DCEB"), // sky
			Green:        lipgloss.Color("#A6E3A1"),
			Yellow:       lipgloss.Color("#F9E2AF"),
			Orange:       lipgloss.Color("#FAB387"), // peach
			Red:          lipgloss.Color("#EBA0AC"),
			StreamAccent: lipgloss.Color("#F5C2E7"), // pink
		}
	}
}

// accentStyle returns a Lipgloss style in the active accent color.
func accentStyle(theme config.Theme, env config.Env) lipgloss.Style {
	p := paletteFor(theme, env)
	return lipgloss.NewStyle().Foreground(p.Accent)
}
