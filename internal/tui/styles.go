package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
)

// palette holds all resolved color tokens for a theme + env combination.
type palette struct {
	bg         color.Color
	bgPanel    color.Color
	bgDeep     color.Color
	bgSelected color.Color
	bgHover    color.Color
	fg         color.Color
	fgDim      color.Color
	fgMuted    color.Color
	border     color.Color
	accent     color.Color
	cyan       color.Color
	green      color.Color
	yellow     color.Color
	orange     color.Color
	red        color.Color
}

func paletteFor(theme config.Theme, env config.Env) palette {
	p := basePalette(theme)
	switch env {
	case config.EnvStaging:
		p.accent = p.orange
	case config.EnvProd:
		p.accent = p.red
	}
	return p
}

func basePalette(theme config.Theme) palette {
	switch theme {
	case config.ThemeDracula:
		return palette{
			bg: lipgloss.Color("#1E1F29"), bgPanel: lipgloss.Color("#21222C"), bgDeep: lipgloss.Color("#1a1b26"),
			bgSelected: lipgloss.Color("#44475A"), bgHover: lipgloss.Color("#383A4A"),
			fg: lipgloss.Color("#F8F8F2"), fgDim: lipgloss.Color("#6272A4"), fgMuted: lipgloss.Color("#44475A"),
			border: lipgloss.Color("#44475A"), accent: lipgloss.Color("#BD93F9"),
			cyan: lipgloss.Color("#8BE9FD"), green: lipgloss.Color("#50FA7B"), yellow: lipgloss.Color("#F1FA8C"),
			orange: lipgloss.Color("#FFB86C"), red: lipgloss.Color("#FF5555"),
		}
	case config.ThemeNord:
		return palette{
			bg: lipgloss.Color("#242933"), bgPanel: lipgloss.Color("#2E3440"), bgDeep: lipgloss.Color("#1a1f2e"),
			bgSelected: lipgloss.Color("#3B4252"), bgHover: lipgloss.Color("#353C4A"),
			fg: lipgloss.Color("#ECEFF4"), fgDim: lipgloss.Color("#4C566A"), fgMuted: lipgloss.Color("#3B4252"),
			border: lipgloss.Color("#3B4252"), accent: lipgloss.Color("#88C0D0"),
			cyan: lipgloss.Color("#8FBCBB"), green: lipgloss.Color("#A3BE8C"), yellow: lipgloss.Color("#EBCB8B"),
			orange: lipgloss.Color("#D08770"), red: lipgloss.Color("#BF616A"),
		}
	default: // Catppuccin Mocha
		return palette{
			bg: lipgloss.Color("#11111B"), bgPanel: lipgloss.Color("#1E1E2E"), bgDeep: lipgloss.Color("#181825"),
			bgSelected: lipgloss.Color("#313244"), bgHover: lipgloss.Color("#292A3E"),
			fg: lipgloss.Color("#CDD6F4"), fgDim: lipgloss.Color("#6C7086"), fgMuted: lipgloss.Color("#313244"),
			border: lipgloss.Color("#313244"), accent: lipgloss.Color("#CBA6F7"),
			cyan: lipgloss.Color("#89DCEB"), green: lipgloss.Color("#A6E3A1"), yellow: lipgloss.Color("#F9E2AF"),
			orange: lipgloss.Color("#FAB387"), red: lipgloss.Color("#F38BA8"),
		}
	}
}

// accentStyle returns a Lipgloss style in the active accent color.
func accentStyle(theme config.Theme, env config.Env) lipgloss.Style {
	p := paletteFor(theme, env)
	return lipgloss.NewStyle().Foreground(p.accent)
}
