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
			Bg: lipgloss.Color("#1E1F29"), BgPanel: lipgloss.Color("#21222C"), BgDeep: lipgloss.Color("#1a1b26"),
			BgSelected: lipgloss.Color("#44475A"), BgHover: lipgloss.Color("#383A4A"),
			Fg: lipgloss.Color("#F8F8F2"), FgDim: lipgloss.Color("#6272A4"), FgMuted: lipgloss.Color("#44475A"),
			Border: lipgloss.Color("#44475A"), Accent: lipgloss.Color("#BD93F9"),
			Cyan: lipgloss.Color("#8BE9FD"), Green: lipgloss.Color("#50FA7B"), Yellow: lipgloss.Color("#F1FA8C"),
			Orange: lipgloss.Color("#FFB86C"), Red: lipgloss.Color("#FF5555"),
		}
	case config.ThemeNord:
		return views.Palette{
			Bg: lipgloss.Color("#242933"), BgPanel: lipgloss.Color("#2E3440"), BgDeep: lipgloss.Color("#1a1f2e"),
			BgSelected: lipgloss.Color("#3B4252"), BgHover: lipgloss.Color("#353C4A"),
			Fg: lipgloss.Color("#ECEFF4"), FgDim: lipgloss.Color("#4C566A"), FgMuted: lipgloss.Color("#3B4252"),
			Border: lipgloss.Color("#3B4252"), Accent: lipgloss.Color("#88C0D0"),
			Cyan: lipgloss.Color("#8FBCBB"), Green: lipgloss.Color("#A3BE8C"), Yellow: lipgloss.Color("#EBCB8B"),
			Orange: lipgloss.Color("#D08770"), Red: lipgloss.Color("#BF616A"),
		}
	default: // Catppuccin Mocha
		return views.Palette{
			Bg: lipgloss.Color("#11111B"), BgPanel: lipgloss.Color("#1E1E2E"), BgDeep: lipgloss.Color("#181825"),
			BgSelected: lipgloss.Color("#313244"), BgHover: lipgloss.Color("#292A3E"),
			Fg: lipgloss.Color("#CDD6F4"), FgDim: lipgloss.Color("#6C7086"), FgMuted: lipgloss.Color("#313244"),
			Border: lipgloss.Color("#313244"), Accent: lipgloss.Color("#CBA6F7"),
			Cyan: lipgloss.Color("#89DCEB"), Green: lipgloss.Color("#A6E3A1"), Yellow: lipgloss.Color("#F9E2AF"),
			Orange: lipgloss.Color("#FAB387"), Red: lipgloss.Color("#F38BA8"),
		}
	}
}

// accentStyle returns a Lipgloss style in the active accent color.
func accentStyle(theme config.Theme, env config.Env) lipgloss.Style {
	p := paletteFor(theme, env)
	return lipgloss.NewStyle().Foreground(p.Accent)
}
