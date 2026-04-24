package splash

import (
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/version"
)

const logoASCII = `  ██████╗  █████╗  ███████╗ ████████╗
 ██╔════╝ ██╔══██╗ ██╔════╝ ╚══██╔══╝
 ██║      ███████║ ███████╗    ██║
 ██║      ██╔══██║ ╚════██║    ██║
 ╚██████╗ ██║  ██║ ███████║    ██║
  ╚═════╝ ╚═╝  ╚═╝ ╚══════╝   ╚═╝   `

var initMessages = []string{
	"✓ loading config  ~/.config/cast/config.toml",
	"✓ found Makefile  10 targets loaded",
	"✓ env             local",
	"→ launching cast…",
}

type phase int

const (
	phaseLogo phase = iota
	phaseTagline
	phaseInit
	phaseDone
)

type tickMsg time.Time

// DoneMsg is sent to the parent model when the splash completes.
type DoneMsg struct{}

// Model drives the splash animation.
type Model struct {
	ph         phase
	logoLines  int // how many logo lines are visible
	taglineVis bool
	initLines  int // how many init messages are visible
	typedChars int // chars typed in the current init message
	ticks      int

	accent lipgloss.Style
	green  lipgloss.Style
	dim    lipgloss.Style
}

// New creates a new splash Model. The env parameter overrides the theme's
// base accent so the logo reflects the active environment — orange for
// staging, red for prod — matching the header pill and status bar.
func New(theme config.Theme, env config.Env) Model {
	return Model{
		accent: lipgloss.NewStyle().Foreground(accentFor(theme, env)).Bold(true),
		green:  lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")),
		dim:    lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")),
	}
}

func (m Model) Init() tea.Cmd {
	return tick()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if _, ok := msg.(tickMsg); ok {
		return m.advance()
	}
	return m, nil
}

// View returns the splash screen as a plain string.
// The parent Model wraps it in tea.NewView().
func (m Model) View() string {
	var b strings.Builder

	lines := strings.Split(logoASCII, "\n")
	for i, l := range lines {
		if i < m.logoLines {
			b.WriteString(m.accent.Render(l) + "\n")
		}
	}

	if m.taglineVis {
		b.WriteString("\n")
		b.WriteString(m.accent.Render("run spells, not commands") + "  ")
		b.WriteString(m.dim.Render("v"+version.Current) + "\n\n")
	}

	for i, msg := range initMessages {
		if i >= m.initLines {
			break
		}
		visible := msg
		if i == m.initLines-1 && m.typedChars < len(msg) {
			visible = msg[:m.typedChars]
		}
		b.WriteString(m.colorLine(visible) + "\n")
	}

	return b.String()
}

func (m Model) advance() (Model, tea.Cmd) {
	m.ticks++

	const logoTotal = 6

	switch m.ph {
	case phaseLogo:
		// One logo line every ~200ms (7 ticks × 30ms ≈ 210ms).
		m.logoLines = m.ticks / 7
		if m.logoLines >= logoTotal {
			m.logoLines = logoTotal
			m.ph = phaseTagline
			m.ticks = 0
		}

	case phaseTagline:
		if m.ticks > 10 {
			m.taglineVis = true
			m.ph = phaseInit
			m.ticks = 0
			m.initLines = 1
		}

	case phaseInit:
		if m.initLines > len(initMessages) {
			m.ph = phaseDone
			return m, func() tea.Msg { return DoneMsg{} }
		}
		currentMsg := initMessages[m.initLines-1]
		if m.typedChars < len(currentMsg) {
			m.typedChars++
		} else if m.ticks%20 == 0 {
			m.initLines++
			m.typedChars = 0
		}

	case phaseDone:
		return m, func() tea.Msg { return DoneMsg{} }
	}

	return m, tick()
}

func (m Model) colorLine(line string) string {
	if strings.HasPrefix(line, "✓") {
		return m.green.Render(line)
	}
	if strings.HasPrefix(line, "→") {
		return m.accent.Render(line)
	}
	return m.dim.Render(line)
}

func tick() tea.Cmd {
	return tea.Tick(30*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func accentFor(theme config.Theme, env config.Env) color.Color {
	// Env override takes precedence so the splash warns about
	// non-local environments at a glance.
	switch env {
	case config.EnvStaging:
		return lipgloss.Color("#FAB387") // orange
	case config.EnvProd:
		return lipgloss.Color("#F38BA8") // red
	}
	switch theme {
	case config.ThemeDracula:
		return lipgloss.Color("#BD93F9")
	case config.ThemeNord:
		return lipgloss.Color("#88C0D0")
	default:
		return lipgloss.Color("#CBA6F7")
	}
}
