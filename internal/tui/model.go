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

// RunRecord is a single entry in the run history.
type RunRecord struct {
	Command  string
	Duration string
	Status   RunStatus
	Time     string
}

// RunStatus represents the outcome of a command execution.
type RunStatus int

const (
	StatusSuccess RunStatus = iota
	StatusError
	StatusRunning
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
	Status RunStatus
	Record RunRecord
}

// tickMsg drives the progress bar animation.
type tickMsg struct{}

// --- Model -------------------------------------------------------------------

// Model is the root Bubble Tea model for cast.
// It owns the splash sub-model until the splash is done, then drives the main
// three-panel layout.
type Model struct {
	// App state
	state AppState

	// Sub-models
	splashModel splash.Model

	// Data
	commands []source.Command
	history  []RunRecord
	filtered []source.Command // search-filtered view of commands

	// Makefile viewer
	makefileLines  []string
	makefilePath   string
	makefileOffset int // scroll offset in lines

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
	running      bool
	runProgress  float64 // 0.0–1.0
	output       []string
	showConfirm  bool
	lastRunCmd   string
	lastRunOK    bool // true = success, false = error
	hasLastRun   bool // whether a run has completed

	// Layout
	width  int
	height int

	// Bubbles sub-models
	searchInput textinput.Model
	viewport    viewport.Model // unused — kept for future
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

	mfLines := loadFileLines(cfg.SourcePath)

	return Model{
		state:         StateSplash,
		splashModel:   splash.New(cfg.Theme),
		commands:      commands,
		filtered:      commands,
		env:           cfg.Env,
		theme:         cfg.Theme,
		searchInput:   si,
		spinner:       sp,
		progressBar:   pb,
		makefileLines: mfLines,
		makefilePath:  cfg.SourcePath,
	}
}

// Init satisfies tea.Model. The splash model drives its own tick loop.
func (m Model) Init() tea.Cmd {
	return m.splashModel.Init()
}

// Update handles all incoming messages and delegates to the appropriate
// sub-model or handler based on the current AppState.
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
			// Any key press skips the splash immediately.
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

	case RunOutputMsg:
		m.output = append(m.output, msg.Line)
		m.outputView.GotoBottom()
		return m, nil

	case RunDoneMsg:
		m.running = false
		m.runProgress = 1.0
		m.hasLastRun = true
		m.lastRunOK = msg.Status == StatusSuccess
		m.history = append([]RunRecord{msg.Record}, m.history...)
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

	// Delegate remaining messages to the splash while in splash state.
	if m.state == StateSplash {
		var cmd tea.Cmd
		m.splashModel, cmd = m.splashModel.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the entire UI for the current frame.
// In bubbletea v2, View() returns tea.View (not string).
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

// --- internal helpers --------------------------------------------------------

// handleKey processes keyboard input in the main TUI state.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Modal intercepts all keys when visible.
	if m.showConfirm {
		return m.handleConfirmModal(msg)
	}

	// Search input captures keys when focused.
	if m.searchInput.Focused() {
		return m.handleSearchKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		m.activeTab = (m.activeTab + 1) % 4
		return m, nil

	case "shift+tab":
		m.activeTab = (m.activeTab + 3) % 4
		return m, nil

	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil

	case "down", "j":
		if m.selected < len(m.filtered)-1 {
			m.selected++
		}
		return m, nil

	case "g":
		m.selected = 0
		return m, nil

	case "G":
		if len(m.filtered) > 0 {
			m.selected = len(m.filtered) - 1
		}
		return m, nil

	case "/":
		m.searchInput.Focus()
		return m, textinput.Blink

	case "enter", "r":
		return m.triggerRun()

	case "ctrl+r":
		if len(m.history) > 0 {
			// re-run last — to be implemented with runner package
		}
		return m, nil

	case "s":
		if m.activeTab == TabEnv {
			m.showSecrets = !m.showSecrets
		}
		return m, nil

	// Quick shortcuts
	case "b":
		return m.runByName("build")
	case "B":
		return m.runByName("build_release")
	case "t":
		return m.runByName("test")
	case "l":
		return m.runByName("lint")
	case "c":
		return m.runByName("clean")
	case "d":
		return m.runByName("deploy")
	}

	return m, nil
}

// handleSearchKey handles keys while the search input is focused.
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

// handleConfirmModal handles keys inside the production confirm dialog.
func (m Model) handleConfirmModal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.showConfirm = false
		return m, nil
	case "enter", "y":
		m.showConfirm = false
		return m.dispatchRun()
	}
	return m, nil
}

// triggerRun either opens the confirm modal (prod) or dispatches immediately.
func (m Model) triggerRun() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.running {
		return m, nil
	}
	if m.env == config.EnvProd {
		m.showConfirm = true
		return m, nil
	}
	return m.dispatchRun()
}

// dispatchRun sends the RunStartMsg for the currently selected command.
func (m Model) dispatchRun() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	cmd := m.filtered[m.selected]
	return m, func() tea.Msg { return RunStartMsg{Command: cmd.Name} }
}

// runByName selects a command by name and triggers it.
func (m Model) runByName(name string) (tea.Model, tea.Cmd) {
	for i, c := range m.filtered {
		if c.Name == name {
			m.selected = i
			return m.triggerRun()
		}
	}
	return m, nil
}

// recalcLayout recomputes viewport sizes after a window resize.
func (m *Model) recalcLayout() {
	const sidebarW = 22
	const outputW = 30
	const borders = 2

	centerW := m.width - sidebarW - outputW - borders
	if centerW < 10 {
		centerW = 10
	}
	contentH := m.height - 2 // subtract header + status bar
	if contentH < 1 {
		contentH = 1
	}

	m.viewport.SetWidth(centerW)
	m.viewport.SetHeight(contentH)
	m.outputView.SetWidth(outputW)
	m.outputView.SetHeight(contentH)
}

// filterCommands returns commands whose name or description contains query.
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
	return foldContains(s, sub)
}

func foldContains(s, sub string) bool {
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

// loadFileLines reads a text file into a slice of lines.
func loadFileLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}
