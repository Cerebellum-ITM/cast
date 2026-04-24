package tui

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Cerebellum-ITM/cast/internal/config"
	"github.com/Cerebellum-ITM/cast/internal/db"
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
	Status db.RunStatus
	Run    db.Run
}

// HistoryLoadedMsg carries runs loaded from the database on startup.
type HistoryLoadedMsg struct{ Runs []db.Run }

// EnvHistoryLoadedMsg carries env change records loaded from the database.
type EnvHistoryLoadedMsg struct{ Changes []db.EnvChange }

// EnvChangedMsg is dispatched after a successful env var write.
type EnvChangedMsg struct{ Key string }

// HistoryErrorMsg reports a non-fatal DB error (load or insert).
type HistoryErrorMsg struct{ Err error }

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
	commands   []source.Command
	history    []db.Run
	historyMax int
	filtered   []source.Command

	// Persistence
	db *db.DB

	// Execution timing
	runStartedAt time.Time

	// Makefile viewer
	makefileLines  []string
	makefilePath   string
	makefileOffset int

	// .env tab state
	envFile        *source.EnvFile
	envFilePath    string
	selectedEnvKey int
	showSecrets    bool
	envEditMode    bool
	envEditBuffer  string
	envSearchInput textinput.Model
	envFocus       int // 0=sidebar, 1=history panel
	envHistoryItems []db.EnvChange
	envHistorySel   int
	envNewMode      bool   // true when adding a new variable
	envNewKeyMode   bool   // true during key-name entry step
	envNewKeyBuffer string // key name typed so far
	envNewSensitive bool   // sensitive toggle during new-var flow

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

	// Output expand popup
	showOutputExpand bool
	outputExpandOff  int

	// Makefile section expand popup
	showMakefileExpand  bool
	makefileExpandOff   int
	makefileExpandLines []string

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
func New(cfg *config.Config, commands []source.Command, database *db.DB) Model {
	si := textinput.New()
	si.Placeholder = "search commands…"
	si.CharLimit = 64

	esi := textinput.New()
	esi.Placeholder = "filter vars…"
	esi.CharLimit = 64

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = accentStyle(cfg.Theme, cfg.Env)

	pb := progress.New(
		progress.WithDefaultBlend(),
		progress.WithoutPercentage(),
	)

	var envFile *source.EnvFile
	if cfg.EnvFilePath != "" {
		if ef, err := source.ParseEnvFile(cfg.EnvFilePath); err == nil {
			envFile = ef
		}
	}

	return Model{
		state:          StateSplash,
		keys:           DefaultKeyMap,
		splashModel:    splash.New(cfg.Theme),
		commands:       commands,
		filtered:       commands,
		historyMax:     cfg.HistoryMax,
		db:             database,
		env:            cfg.Env,
		theme:          cfg.Theme,
		searchInput:    si,
		envSearchInput: esi,
		spinner:        sp,
		progressBar:    pb,
		makefileLines:  loadFileLines(cfg.SourcePath),
		makefilePath:   cfg.SourcePath,
		envFile:        envFile,
		envFilePath:    cfg.EnvFilePath,
	}
}

// NewOnTab creates a Model that starts with the given tab active.
func NewOnTab(cfg *config.Config, commands []source.Command, database *db.DB, tab Tab) Model {
	m := New(cfg, commands, database)
	m.activeTab = tab
	return m
}

// Init satisfies tea.Model. The splash model drives its own tick loop,
// and we kick off DB reads so history tabs are populated on first paint.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.splashModel.Init(), m.loadHistoryCmd(), m.loadEnvHistoryCmd())
}

func (m Model) loadHistoryCmd() tea.Cmd {
	if m.db == nil {
		return nil
	}
	limit := m.historyMax
	if limit <= 0 {
		limit = 100
	}
	database := m.db
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		runs, err := database.RecentRuns(ctx, limit)
		if err != nil {
			return HistoryErrorMsg{Err: fmt.Errorf("load history: %w", err)}
		}
		return HistoryLoadedMsg{Runs: runs}
	}
}

func (m Model) loadEnvHistoryCmd() tea.Cmd {
	if m.db == nil {
		return nil
	}
	database := m.db
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		changes, err := database.RecentEnvChanges(ctx, 100)
		if err != nil {
			return HistoryErrorMsg{Err: fmt.Errorf("load env history: %w", err)}
		}
		return EnvHistoryLoadedMsg{Changes: changes}
	}
}

func (m Model) saveEnvVarCmd(key, oldVal, newVal string, sensitive bool) tea.Cmd {
	database := m.db
	envFile := m.envFile
	envFilePath := m.envFilePath
	return func() tea.Msg {
		if envFile != nil && envFilePath != "" {
			if err := source.WriteEnvFile(envFile, envFilePath); err != nil {
				return HistoryErrorMsg{Err: fmt.Errorf("write env file: %w", err)}
			}
		}
		if database != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			old := sql.NullString{String: oldVal, Valid: oldVal != ""}
			_, _ = database.InsertEnvChange(ctx, db.EnvChange{
				Key:       key,
				OldValue:  old,
				NewValue:  newVal,
				Sensitive: sensitive,
				EnvFile:   envFilePath,
				ChangedAt: time.Now(),
				ChangedBy: "user",
			})
		}
		return EnvChangedMsg{Key: key}
	}
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
		m.runStartedAt = time.Now()
		return m, tea.Batch(m.spinner.Tick, tickCmd())

	case runner.OutputMsg:
		m.output = append(m.output, msg.Line)
		m.outputView.GotoBottom()
		return m, waitNext(m.streamCh)

	case runner.DoneMsg:
		startedAt := m.runStartedAt
		if startedAt.IsZero() {
			startedAt = time.Now().Add(-msg.Duration)
		}
		run := db.NewRun(m.lastRunCmd, m.env.String(), startedAt, msg.Duration, msg.Err)
		m.streamCh = nil
		if m.db != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if id, err := m.db.InsertRun(ctx, run); err == nil {
				run.ID = id
			}
			cancel()
		}
		return m, func() tea.Msg { return RunDoneMsg{Status: run.Status, Run: run} }

	case RunDoneMsg:
		m.running = false
		m.runProgress = 1.0
		m.hasLastRun = true
		m.lastRunOK = msg.Status == db.StatusSuccess
		m.history = append([]db.Run{msg.Run}, m.history...)
		if m.historyMax > 0 && len(m.history) > m.historyMax {
			m.history = m.history[:m.historyMax]
		}
		return m, nil

	case HistoryLoadedMsg:
		m.history = msg.Runs
		return m, nil

	case EnvHistoryLoadedMsg:
		m.envHistoryItems = msg.Changes
		return m, nil

	case EnvChangedMsg:
		if m.envFilePath != "" {
			if ef, err := source.ParseEnvFile(m.envFilePath); err == nil {
				m.envFile = ef
			}
		}
		return m, m.loadEnvHistoryCmd()

	case HistoryErrorMsg:
		// Non-fatal; leave history empty and move on.
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
	if m.showOutputExpand {
		return m.handleExpandedOutput(msg)
	}
	if m.showMakefileExpand {
		return m.handleMakefileExpand(msg)
	}
	if m.activeTab == TabEnv {
		return m.handleEnvKey(msg)
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
		if len(m.history) > 0 && !m.running {
			return m.runByName(m.history[0].Command)
		}
	case k == m.keys.ToggleSecrets:
		if m.activeTab == TabEnv {
			m.showSecrets = !m.showSecrets
		}
	case k == m.keys.ExpandOutput:
		return m.toggleOutputExpand()
	case k == m.keys.ExpandMakefile:
		return m.toggleMakefileExpand()
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

func (m Model) handleExpandedOutput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	visH := m.outputExpandVisH()
	maxOff := len(m.output) - visH
	if maxOff < 0 {
		maxOff = 0
	}
	switch msg.String() {
	case "esc", m.keys.ExpandOutput:
		m.showOutputExpand = false
	case "up", "k":
		if m.outputExpandOff > 0 {
			m.outputExpandOff--
		}
	case "down", "j":
		if m.outputExpandOff < maxOff {
			m.outputExpandOff++
		}
	case "pgup", "ctrl+b":
		m.outputExpandOff -= visH
		if m.outputExpandOff < 0 {
			m.outputExpandOff = 0
		}
	case "pgdown", "ctrl+f", " ":
		m.outputExpandOff += visH
		if m.outputExpandOff > maxOff {
			m.outputExpandOff = maxOff
		}
	case "g":
		m.outputExpandOff = 0
	case "G":
		m.outputExpandOff = maxOff
	}
	return m, nil
}

func (m Model) toggleOutputExpand() (tea.Model, tea.Cmd) {
	if m.showOutputExpand {
		m.showOutputExpand = false
		return m, nil
	}
	m.showOutputExpand = true
	visH := m.outputExpandVisH()
	m.outputExpandOff = len(m.output) - visH
	if m.outputExpandOff < 0 {
		m.outputExpandOff = 0
	}
	return m, nil
}

func (m Model) outputExpandVisH() int {
	popupH := m.height - 4
	if popupH < 10 {
		popupH = 10
	}
	visH := popupH - 6
	if visH < 1 {
		visH = 1
	}
	return visH
}

func (m Model) toggleMakefileExpand() (tea.Model, tea.Cmd) {
	if m.showMakefileExpand {
		m.showMakefileExpand = false
		return m, nil
	}
	if len(m.filtered) == 0 {
		return m, nil
	}
	cmd := m.filtered[m.selected]
	m.makefileExpandLines = commandMakefileSection(m.makefileLines, cmd.Name)
	m.makefileExpandOff = 0
	m.showMakefileExpand = true
	return m, nil
}

func (m Model) handleMakefileExpand(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	visH := m.makefileExpandVisH()
	maxOff := len(m.makefileExpandLines) - visH
	if maxOff < 0 {
		maxOff = 0
	}
	switch msg.String() {
	case "esc", "q", m.keys.ExpandMakefile:
		m.showMakefileExpand = false
	case "up", "k":
		if m.makefileExpandOff > 0 {
			m.makefileExpandOff--
		}
	case "down", "j":
		if m.makefileExpandOff < maxOff {
			m.makefileExpandOff++
		}
	case "pgup", "ctrl+b":
		m.makefileExpandOff -= visH
		if m.makefileExpandOff < 0 {
			m.makefileExpandOff = 0
		}
	case "pgdown", "ctrl+f", " ":
		m.makefileExpandOff += visH
		if m.makefileExpandOff > maxOff {
			m.makefileExpandOff = maxOff
		}
	case "g":
		m.makefileExpandOff = 0
	case "G":
		m.makefileExpandOff = maxOff
	}
	return m, nil
}

func (m Model) makefileExpandVisH() int {
	popupH := m.height - 4
	if popupH < 10 {
		popupH = 10
	}
	visH := popupH - 6
	if visH < 1 {
		visH = 1
	}
	return visH
}

func (m Model) handleEnvKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// New-variable flow: step 1 — typing the key name.
	if m.envNewKeyMode {
		switch k {
		case "esc":
			m.envNewMode = false
			m.envNewKeyMode = false
			m.envNewKeyBuffer = ""
			m.envNewSensitive = false
		case "enter":
			if m.envNewKeyBuffer != "" {
				// Auto-detect sensitive from key name, but respect manual toggle.
				if !m.envNewSensitive {
					m.envNewSensitive = source.IsSensitiveKey(m.envNewKeyBuffer)
				}
				return m.commitNewKey()
			}
		case "ctrl+s":
			m.envNewSensitive = !m.envNewSensitive
		case "backspace":
			runes := []rune(m.envNewKeyBuffer)
			if len(runes) > 0 {
				m.envNewKeyBuffer = string(runes[:len(runes)-1])
			}
		default:
			if len(k) == 1 {
				m.envNewKeyBuffer += k
			}
		}
		return m, nil
	}

	// Edit mode captures all printable input.
	if m.envEditMode {
		switch k {
		case "esc":
			m.envEditMode = false
			m.envEditBuffer = ""
			if m.envNewMode {
				// Cancel the whole new-var flow: remove the placeholder.
				m.envNewMode = false
				m.envNewSensitive = false
				if m.envFile != nil && len(m.envFile.Vars) > 0 {
					m.envFile.Vars = m.envFile.Vars[:len(m.envFile.Vars)-1]
				}
				if m.selectedEnvKey >= len(m.envFile.Vars) && m.selectedEnvKey > 0 {
					m.selectedEnvKey = len(m.envFile.Vars) - 1
				}
			}
		case "enter":
			return m.commitEnvEdit()
		case "ctrl+s":
			if m.envNewMode {
				m.envNewSensitive = !m.envNewSensitive
			}
			// Always toggle the in-memory flag on the selected var.
			m.toggleSelectedSensitive()
		case "backspace":
			runes := []rune(m.envEditBuffer)
			if len(runes) > 0 {
				m.envEditBuffer = string(runes[:len(runes)-1])
			}
		default:
			if len(k) == 1 {
				m.envEditBuffer += k
			}
		}
		return m, nil
	}

	// Env search input captures all input when focused.
	if m.envSearchInput.Focused() {
		switch k {
		case "esc":
			m.envSearchInput.Blur()
			m.envSearchInput.SetValue("")
			m.selectedEnvKey = 0
		case "enter":
			m.envSearchInput.Blur()
		default:
			var cmd tea.Cmd
			m.envSearchInput, cmd = m.envSearchInput.Update(msg)
			m.selectedEnvKey = 0
			return m, cmd
		}
		return m, nil
	}

	switch k {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab", m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % 4
	case m.keys.TabPrev:
		m.activeTab = (m.activeTab + 3) % 4
	case "left", "h":
		m.envFocus = 0
	case "right", "l":
		m.envFocus = 1
	case m.keys.Up, m.keys.UpVim:
		m.envNavUp()
	case m.keys.Down, m.keys.DownVim:
		m.envNavDown()
	case m.keys.Top:
		m.envNavTop()
	case m.keys.Bottom:
		m.envNavBottom()
	case m.keys.Search:
		m.envSearchInput.Focus()
		return m, textinput.Blink
	case m.keys.ToggleSecrets:
		m.showSecrets = !m.showSecrets
	case "ctrl+s":
		m.toggleSelectedSensitive()
	case "ctrl+a":
		return m.startNewVar()
	case "enter":
		if m.envFocus == 0 {
			return m.startEnvEdit()
		}
	case m.keys.EnvRestore:
		if m.envFocus == 1 {
			return m.restoreEnvValue()
		}
	}
	return m, nil
}

func (m *Model) envNavUp() {
	if m.envFocus == 0 {
		if m.selectedEnvKey > 0 {
			m.selectedEnvKey--
		}
	} else {
		if m.envHistorySel > 0 {
			m.envHistorySel--
		}
	}
}

func (m *Model) envNavDown() {
	if m.envFocus == 0 {
		vars := filterEnvVars(m.envFile, m.envSearchInput.Value())
		if m.selectedEnvKey < len(vars)-1 {
			m.selectedEnvKey++
		}
	} else {
		if m.envHistorySel < len(m.envHistoryItems)-1 {
			m.envHistorySel++
		}
	}
}

func (m *Model) envNavTop() {
	if m.envFocus == 0 {
		m.selectedEnvKey = 0
	} else {
		m.envHistorySel = 0
	}
}

func (m *Model) envNavBottom() {
	if m.envFocus == 0 {
		vars := filterEnvVars(m.envFile, m.envSearchInput.Value())
		if len(vars) > 0 {
			m.selectedEnvKey = len(vars) - 1
		}
	} else {
		if len(m.envHistoryItems) > 0 {
			m.envHistorySel = len(m.envHistoryItems) - 1
		}
	}
}

func (m Model) startEnvEdit() (tea.Model, tea.Cmd) {
	vars := filterEnvVars(m.envFile, m.envSearchInput.Value())
	if len(vars) == 0 || m.selectedEnvKey >= len(vars) {
		return m, nil
	}
	m.envEditMode = true
	m.envEditBuffer = vars[m.selectedEnvKey].Value
	return m, nil
}

// toggleSelectedSensitive flips the Sensitive flag on the currently selected var.
// This is a pure in-memory change; .env files have no sensitive metadata column.
func (m *Model) toggleSelectedSensitive() {
	if m.envFile == nil {
		return
	}
	vars := filterEnvVars(m.envFile, m.envSearchInput.Value())
	if len(vars) == 0 || m.selectedEnvKey >= len(vars) {
		return
	}
	targetKey := vars[m.selectedEnvKey].Key
	for i, v := range m.envFile.Vars {
		if v.Key == targetKey {
			m.envFile.Vars[i].Sensitive = !m.envFile.Vars[i].Sensitive
			return
		}
	}
}

func (m Model) startNewVar() (tea.Model, tea.Cmd) {
	m.envFocus = 0
	m.envNewMode = true
	m.envNewKeyMode = true
	m.envNewKeyBuffer = ""
	m.envNewSensitive = false
	m.envSearchInput.Blur()
	m.envSearchInput.SetValue("")
	return m, nil
}

func (m Model) commitNewKey() (tea.Model, tea.Cmd) {
	key := strings.TrimSpace(m.envNewKeyBuffer)
	if key == "" {
		m.envNewMode = false
		m.envNewKeyMode = false
		m.envNewKeyBuffer = ""
		m.envNewSensitive = false
		return m, nil
	}
	if m.envFile == nil {
		m.envFile = &source.EnvFile{Filename: m.envFilePath}
	}
	m.envFile.Vars = append(m.envFile.Vars, source.EnvVar{
		Key:       key,
		Sensitive: m.envNewSensitive,
	})
	m.selectedEnvKey = len(m.envFile.Vars) - 1
	m.envNewKeyMode = false
	m.envEditMode = true
	m.envEditBuffer = ""
	return m, nil
}

func (m Model) commitEnvEdit() (tea.Model, tea.Cmd) {
	vars := filterEnvVars(m.envFile, m.envSearchInput.Value())
	if m.envFile == nil || len(vars) == 0 || m.selectedEnvKey >= len(vars) {
		m.envEditMode = false
		m.envNewMode = false
		return m, nil
	}
	selectedVar := vars[m.selectedEnvKey]
	newValue := m.envEditBuffer

	isNew := m.envNewMode
	var oldValue string
	if !isNew {
		oldValue = selectedVar.Value
	}

	// Update in the full envFile.Vars.
	for i, v := range m.envFile.Vars {
		if v.Key == selectedVar.Key {
			m.envFile.Vars[i].Value = newValue
			break
		}
	}
	m.envEditMode = false
	m.envEditBuffer = ""
	m.envNewMode = false
	m.envNewKeyBuffer = ""
	return m, m.saveEnvVarCmd(selectedVar.Key, oldValue, newValue, selectedVar.Sensitive)
}

func (m Model) restoreEnvValue() (tea.Model, tea.Cmd) {
	if len(m.envHistoryItems) == 0 || m.envHistorySel >= len(m.envHistoryItems) {
		return m, nil
	}
	change := m.envHistoryItems[m.envHistorySel]
	if !change.OldValue.Valid {
		return m, nil
	}
	if m.envFile == nil {
		return m, nil
	}
	for i, v := range m.envFile.Vars {
		if v.Key == change.Key {
			oldCurrent := m.envFile.Vars[i].Value
			m.envFile.Vars[i].Value = change.OldValue.String
			return m, m.saveEnvVarCmd(change.Key, oldCurrent, change.OldValue.String, v.Sensitive)
		}
	}
	return m, nil
}

func filterEnvVars(ef *source.EnvFile, query string) []source.EnvVar {
	if ef == nil {
		return nil
	}
	if query == "" {
		return ef.Vars
	}
	var out []source.EnvVar
	for _, v := range ef.Vars {
		if containsFold(v.Key, query) || containsFold(v.Comment, query) {
			out = append(out, v)
		}
	}
	return out
}

func commandMakefileSection(lines []string, name string) []string {
	if len(lines) == 0 || name == "" {
		return nil
	}
	targetIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "\t") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == name+":" || strings.HasPrefix(trimmed, name+":") || strings.HasPrefix(trimmed, name+" ") {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		return nil
	}
	startIdx := targetIdx
	if targetIdx > 0 && strings.Contains(lines[targetIdx-1], "## "+name) {
		startIdx = targetIdx - 1
	}
	endIdx := len(lines)
	for i := targetIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "\t") {
			continue
		}
		endIdx = i
		break
	}
	return lines[startIdx:endIdx]
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
