package tui

import "charm.land/lipgloss/v2"

// renderMain composes the full three-panel layout into a single string.
func (m Model) renderMain() string {
	p := paletteFor(m.theme, m.env)

	header := m.renderHeader(p)
	body := m.renderBody(p)
	status := m.renderStatusBar(p)

	return lipgloss.JoinVertical(lipgloss.Top, header, body, status)
}

func (m Model) renderHeader(p palette) string {
	style := lipgloss.NewStyle().
		Background(p.bgPanel).
		Foreground(p.fg).
		Width(m.width).
		Padding(0, 1)

	logo := lipgloss.NewStyle().Foreground(p.accent).Bold(true).Render("⬡ cast")
	tabs := m.renderTabs(p)
	envPill := lipgloss.NewStyle().Foreground(p.accent).Render("● " + m.env.String())

	gap := lipgloss.NewStyle().
		Width(m.width - lipgloss.Width(logo) - lipgloss.Width(tabs) - lipgloss.Width(envPill) - 4).
		Render("")

	return style.Render(lipgloss.JoinHorizontal(lipgloss.Center, logo, "  ", tabs, gap, envPill))
}

func (m Model) renderTabs(p palette) string {
	names := []string{"commands", "history", ".env", "theme"}
	rendered := make([]string, len(names))
	for i, name := range names {
		if Tab(i) == m.activeTab {
			rendered[i] = lipgloss.NewStyle().
				Foreground(p.fg).
				BorderBottom(true).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(p.accent).
				Render(name)
		} else {
			rendered[i] = lipgloss.NewStyle().Foreground(p.fgDim).Render(name)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		rendered[0], "  ", rendered[1], "  ", rendered[2], "  ", rendered[3],
	)
}

func (m Model) renderBody(p palette) string {
	h := m.height - 2
	if h < 1 {
		h = 1
	}
	sidebar := m.renderSidebar(p, h)
	center := m.renderCenter(p, h)
	output := m.renderOutput(p, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, center, output)
}

func (m Model) renderSidebar(p palette, h int) string {
	const w = 22
	style := lipgloss.NewStyle().
		Width(w).Height(h).
		Background(p.bgPanel).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(p.border)

	search := lipgloss.NewStyle().Foreground(p.fgDim).Render("⌕ " + m.searchInput.View())

	rows := []string{search}
	for i, cmd := range m.filtered {
		name := cmd.Name
		if len(name) > w-4 {
			name = name[:w-4] + "…"
		}
		var row string
		if i == m.selected {
			row = lipgloss.NewStyle().
				Foreground(p.fg).
				Background(p.bgSelected).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(p.accent).
				Width(w - 3).
				Render(name)
		} else {
			row = lipgloss.NewStyle().Foreground(p.fgDim).Width(w - 2).Render(name)
		}
		rows = append(rows, row)
	}

	hints := lipgloss.NewStyle().Foreground(p.fgDim).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(p.border).
		Width(w - 2).
		Render("↑↓ nav  ⏎ run  / search  q quit")

	rows = append(rows, hints)
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) renderCenter(p palette, h int) string {
	w := m.viewport.Width()
	if w < 10 {
		w = 10
	}
	style := lipgloss.NewStyle().Width(w).Height(h).Background(p.bgDeep)

	if len(m.filtered) == 0 {
		placeholder := lipgloss.NewStyle().Foreground(p.fgMuted).Render("no commands found")
		return style.Render(placeholder)
	}

	cmd := m.filtered[m.selected]
	name := lipgloss.NewStyle().Foreground(p.accent).Bold(true).Render(cmd.Name)
	desc := lipgloss.NewStyle().Foreground(p.fgDim).Render(cmd.Desc)
	preview := lipgloss.NewStyle().Foreground(p.fgDim).Render(m.viewport.View())

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, name, desc, preview))
}

func (m Model) renderOutput(p palette, h int) string {
	const w = 30
	style := lipgloss.NewStyle().
		Width(w).Height(h).
		Background(p.bgDeep).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(p.border)

	label := lipgloss.NewStyle().Foreground(p.fgDim).Bold(true).Render("OUTPUT")

	lines := []string{label}
	for _, l := range m.output {
		lines = append(lines, lipgloss.NewStyle().Foreground(p.fgDim).Render(l))
	}
	if len(m.output) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(p.fgMuted).Render("run a command to see output…"))
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m Model) renderStatusBar(p palette) string {
	style := lipgloss.NewStyle().
		Background(p.accent).
		Foreground(p.bgDeep).
		Bold(true).
		Width(m.width).
		Padding(0, 1)

	left := "⬡ cast · " + envCount(m) + " · ● " + m.env.String()
	right := "v0.1.0"
	gap := lipgloss.NewStyle().
		Width(m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2).
		Render("")

	return style.Render(lipgloss.JoinHorizontal(lipgloss.Left, left, gap, right))
}

func envCount(m Model) string {
	n := len(m.commands)
	if n == 1 {
		return "1 command"
	}
	return itoa(n) + " commands"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
