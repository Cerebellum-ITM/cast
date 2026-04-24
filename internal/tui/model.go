package tui

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/runner"
	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui/splash"
)

// Tab identifies the active top-level tab.
type Tab int

const (
	TabCommands Tab = iota
	TabHistory
	TabEnv
	TabTheme
)

// AppState tracks whether we are in the splash or the main TUI.
type AppState int

const (
	StateSplash AppState = iota
	StateMain
)

// --- Messages ----------------------------------------------------------------

// SplashDoneMsg is emitted by the splash model when the animation completes.
type SplashDoneMsg struct{}

// RunStartMsg signals that command execution has begun.
type RunStartMsg struct{ Command string }

// RunOutputMsg carries a single streamed output line.
type RunOutputMsg struct{ Line string }

// RunDoneMsg signals execution completion.
type RunDoneMsg struct {
	Status runner.RunStatus
	Record runner.RunRecord
}

// tickMsg drives the progress bar animation.
type tickMsg struct{}

// --- Model -------------------------------------------------------------------

// Model is the root Bubble Tea model for cast.
type Model struct {
	// App state
	state AppState
	keys  KeyMap

	// Sub-models
	splashModel splash.Model

	// Data
	commands []source.Command
	history  []runner.RunRecord
	filtered []source.Command

	// Makefile viewer
	makefileLines  []string
	makefilePath   string
	makefileOffset int

	// .env tab state
	envFile        *source.EnvFile
	selectedEnvKey int
	showSecrets    bool

	// Navigation
	selected  int
	search    string
	activeTab Tab
	env       config.Env
	theme     config.Theme

	// Execution state
	running     bool
	runProgress float64
	output      []string
	showConfirm bool
	lastRunCmd  string
	lastRunOK   bool
	hasLastRun  bool
	streamCh    <-chan tea.Msg

	// Layout
	width  int
	height int

	// Bubbles sub-models
	searchInput textinput.Model
	viewport    viewport.Model
	outputView  viewport.Model
	spinner     spinner.Model
	progressBar progress.Model
}

// New creates a fully initialized Model ready to be passed to tea.NewProgram.
func New(cfg *config.Config, commands []source.Command) Model {
	si := textinput.New()
	si.Placeholder = "search commands…"
	si.CharLimit = 64

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = accentStyle(cfg.Theme, cfg.Env)

	pb := progress.New(
		progress.WithDefaultBlend(),
		progress.WithoutPercentage(),
	)

	return Model{
		state:         StateSplash,
		keys:          DefaultKeyMap,
		splashModel:   splash.New(cfg.Theme),
		commands:      commands,
		filtered:      commands,
		env:           cfg.Env,
		theme:         cfg.Theme,
		searchInput:   si,
		spinner:       sp,
		progressBar:   pb,
		makefileLines: loadFileLines(cfg.SourcePath),
		makefilePath:  cfg.SourcePath,
	}
}

// Init satisfies tea.Model. The splash model drives its own tick loop.
func (m Model) Init() tea.Cmd {
	return m.splashModel.Init()
}

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil

	case SplashDoneMsg:
		m.state = StateMain
		return m, nil

	case tea.KeyPressMsg:
		if m.state == StateSplash {
			m.state = StateMain
			return m, nil
		}
		return m.handleKey(msg)

	case RunStartMsg:
		m.running = true
		m.runProgress = 0
		m.output = nil
		m.lastRunCmd = msg.Command
		return m, tea.Batch(m.spinner.Tick, tickCmd())

	case runner.OutputMsg:
		m.output = append(m.output, msg.Line)
		m.outputView.GotoBottom()
		return m, waitNext(m.streamCh)

	case runner.DoneMsg:
		rec := runner.NewRecord(m.lastRunCmd, msg.Duration, msg.Err)
		status := runner.StatusSuccess
		if msg.Err != nil {
			status = runner.StatusError
		}
		m.streamCh = nil
		return m, func() tea.Msg { return RunDoneMsg{Status: status, Record: rec} }

	case RunDoneMsg:
		m.running = false
		m.runProgress = 1.0
		m.hasLastRun = true
		m.lastRunOK = msg.Status == runner.StatusSuccess
		m.history = append([]runner.RunRecord{msg.Record}, m.history...)
		return m, nil

	case tickMsg:
		if !m.running {
			return m, nil
		}
		m.runProgress = clampProgress(m.runProgress + 0.02)
		return m, tickCmd()

	case spinner.TickMsg:
		if m.state == StateSplash {
			var splashCmd tea.Cmd
			m.splashModel, splashCmd = m.splashModel.Update(msg)
			return m, splashCmd
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.state == StateSplash {
		var cmd tea.Cmd
		m.splashModel, cmd = m.splashModel.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the entire UI for the current frame.
func (m Model) View() tea.View {
	var content string
	if m.state == StateSplash {
		content = m.splashModel.View()
	} else {
		content = m.renderMain()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// --- key handling ------------------------------------------------------------

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.showConfirm {
		return m.handleConfirmModal(msg)
	}
	if m.searchInput.Focused() {
		return m.handleSearchKey(msg)
	}

	k := msg.String()
	switch {
	case k == m.keys.Quit, k == "ctrl+c":
		return m, tea.Quit
	case k == m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % 4
	case k == m.keys.TabPrev:
		m.activeTab = (m.activeTab + 3) % 4
	case k == m.keys.Up, k == m.keys.UpVim:
		if m.selected > 0 {
			m.selected--
		}
	case k == m.keys.Down, k == m.keys.DownVim:
		if m.selected < len(m.filtered)-1 {
			m.selected++
		}
	case k == m.keys.Top:
		m.selected = 0
	case k == m.keys.Bottom:
		if len(m.filtered) > 0 {
			m.selected = len(m.filtered) - 1
		}
	case k == m.keys.Search:
		m.searchInput.Focus()
		return m, textinput.Blink
	case k == m.keys.Run, k == m.keys.RunAlt:
		return m.triggerRun()
	case k == m.keys.RerunLast:
		if len(m.history) > 0 {
			// TODO: re-run last via runner.StreamRun
		}
	case k == m.keys.ToggleSecrets:
		if m.activeTab == TabEnv {
			m.showSecrets = !m.showSecrets
		}
	// Quick shortcuts
	case k == "b":
		return m.runByName("build")
	case k == "B":
		return m.runByName("build_release")
	case k == "t":
		return m.runByName("test")
	case k == "l":
		return m.runByName("lint")
	case k == "c":
		return m.runByName("clean")
	case k == "d":
		return m.runByName("deploy")
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.search = ""
		m.filtered = m.commands
		m.selected = 0
		return m, nil
	case "enter":
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.search = m.searchInput.Value()
	m.filtered = filterCommands(m.commands, m.search)
	m.selected = 0
	return m, cmd
}

func (m Model) handleConfirmModal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.showConfirm = false
	case "enter", "y":
		m.showConfirm = false
		return m.dispatchRun()
	}
	return m, nil
}

// --- run dispatch ------------------------------------------------------------

func (m Model) triggerRun() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.running {
		return m, nil
	}
	cmd := m.filtered[m.selected]
	if m.env != config.EnvLocal || cmd.Confirm {
		m.showConfirm = true
		return m, nil
	}
	return m.dispatchRun()
}

func (m Model) dispatchRun() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	name := m.filtered[m.selected].Name
	ch := runner.StreamRun(name)
	m.streamCh = ch
	startCmd := func() tea.Msg { return RunStartMsg{Command: name} }
	return m, tea.Batch(startCmd, waitNext(ch))
}

func waitNext(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m Model) runByName(name string) (tea.Model, tea.Cmd) {
	for i, c := range m.filtered {
		if c.Name == name {
			m.selected = i
			return m.triggerRun()
		}
	}
	return m, nil
}

// --- layout ------------------------------------------------------------------

func (m *Model) recalcLayout() {
	const borders = 2
	centerW := m.width - sidebarW - outputW - borders
	if centerW < 10 {
		centerW = 10
	}
	contentH := m.height - 2
	if contentH < 1 {
		contentH = 1
	}
	m.viewport.SetWidth(centerW)
	m.viewport.SetHeight(contentH)
	m.outputView.SetWidth(outputW)
	m.outputView.SetHeight(contentH)
}

// --- helpers -----------------------------------------------------------------

func filterCommands(cmds []source.Command, query string) []source.Command {
	if query == "" {
		return cmds
	}
	var out []source.Command
	for _, c := range cmds {
		if containsFold(c.Name, query) || containsFold(c.Desc, query) {
			out = append(out, c)
		}
	}
	return out
}

func containsFold(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if equalFold(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func clampProgress(v float64) float64 {
	if v > 0.95 {
		return 0.95
	}
	return v
}

func tickCmd() tea.Cmd {
	return func() tea.Msg { return tickMsg{} }
}

func loadFileLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}
