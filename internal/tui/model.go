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
	"github.com/Cerebellum-ITM/cast/internal/tui/views"
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
type RunStartMsg struct {
	Command string
	Stream  bool
}

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

// maxOutputLines caps the in-memory output buffer so long streams (hours of
// docker logs) don't leak memory.
const maxOutputLines = 2000

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
	envFile         *source.EnvFile
	envFilePath     string
	selectedEnvKey  int
	showSecrets     bool
	envEditMode     bool
	envEditBuffer   string
	envSearchInput  textinput.Model
	envFocus        int // 0=sidebar, 1=history panel
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
	runCancel   context.CancelFunc // non-nil while a run is active
	streaming   bool               // current run is a long-lived log stream
	livePulse   bool               // flipped each tick for LIVE dot animation
	interrupted bool               // current/last run was manually canceled

	// Shortcut edit mode (single-key capture for the selected command).
	shortcutEditMode bool

	// Tag editor popup (toggles for [stream]/[no-stream]/[confirm]/[no-confirm]).
	showTagsPopup  bool
	tagsPopupSel   int
	tagsPopupState source.DocTagState
	tagsPopupName  string
	// Sub-mode inside the tags popup: editing the `[tags=...]` CSV list.
	tagsEditing    bool
	tagsEditBuffer string

	// Output expand popup
	showOutputExpand bool
	outputExpandOff  int

	// Makefile section expand popup
	showMakefileExpand  bool
	makefileExpandOff   int
	makefileExpandLines []string

	// Layout
	width           int
	height          int
	outputWidthPct  int // 30–60 (or 30–50 when center hidden)
	sidebarWidthPct int // 15–40 (or 30–50 when center hidden)
	showCenter      bool

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
		state:           StateSplash,
		keys:            DefaultKeyMap,
		splashModel:     splash.New(cfg.Theme),
		commands:        commands,
		filtered:        commands,
		historyMax:      cfg.HistoryMax,
		db:              database,
		env:             cfg.Env,
		theme:           cfg.Theme,
		searchInput:     si,
		envSearchInput:  esi,
		spinner:         sp,
		progressBar:     pb,
		makefileLines:   loadFileLines(cfg.SourcePath),
		makefilePath:    cfg.SourcePath,
		envFile:         envFile,
		envFilePath:     cfg.EnvFilePath,
		outputWidthPct:  cfg.OutputWidthPct,
		sidebarWidthPct: cfg.SidebarWidthPct,
		showCenter:      cfg.ShowCenterPanel,
	}
}

// outputPanelW returns the output panel width in columns, derived from the
// percentage preference. Includes the left divider char.
func (m Model) outputPanelW() int {
	w := m.width * m.outputWidthPct / 100
	if w < 20 {
		w = 20
	}
	return w
}

// sidebarPanelW returns the left sidebar width in columns. Includes the right
// divider char.
func (m Model) sidebarPanelW() int {
	w := m.width * m.sidebarWidthPct / 100
	if w < 18 {
		w = 18
	}
	return w
}

// outputPctMax returns the maximum allowed output % given the current layout
// (center visible vs hidden and the current sidebar %).
func (m Model) outputPctMax() int {
	if m.showCenter {
		lim := 90 - m.sidebarWidthPct
		if lim > 60 {
			lim = 60
		}
		return lim
	}
	lim := 100 - m.sidebarWidthPct
	if lim > 50 {
		lim = 50
	}
	return lim
}

// sidebarPctMax returns the maximum allowed sidebar % given the current layout.
func (m Model) sidebarPctMax() int {
	if m.showCenter {
		lim := 90 - m.outputWidthPct
		if lim > 40 {
			lim = 40
		}
		return lim
	}
	lim := 100 - m.outputWidthPct
	if lim > 50 {
		lim = 50
	}
	return lim
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
		m.streaming = msg.Stream
		m.livePulse = true
		m.interrupted = false
		m.runProgress = 0
		m.output = nil
		m.lastRunCmd = msg.Command
		m.runStartedAt = time.Now()
		return m, tea.Batch(m.spinner.Tick, tickCmd())

	case runner.OutputMsg:
		// Detect "follow mode" in the expand popup BEFORE we append the new
		// line: if the user was scrolled to the bottom (or the buffer is
		// empty), new lines should keep pulling the view down.
		follow := true
		if m.showOutputExpand {
			visH := m.outputExpandVisH()
			prevMaxOff := len(m.output) - visH
			if prevMaxOff < 0 {
				prevMaxOff = 0
			}
			follow = m.outputExpandOff >= prevMaxOff
		}

		m.output = append(m.output, msg.Line)
		if len(m.output) > maxOutputLines {
			m.output = m.output[len(m.output)-maxOutputLines:]
			if m.outputExpandOff > len(m.output) {
				m.outputExpandOff = 0
			}
		}

		if m.showOutputExpand && follow {
			visH := m.outputExpandVisH()
			m.outputExpandOff = len(m.output) - visH
			if m.outputExpandOff < 0 {
				m.outputExpandOff = 0
			}
		}

		m.outputView.GotoBottom()
		return m, waitNext(m.streamCh)

	case runner.DoneMsg:
		startedAt := m.runStartedAt
		if startedAt.IsZero() {
			startedAt = time.Now().Add(-msg.Duration)
		}
		run := db.NewRun(m.lastRunCmd, m.env.String(), startedAt, msg.Duration, msg.Err, msg.Interrupted)
		m.streamCh = nil
		if m.runCancel != nil {
			m.runCancel()
			m.runCancel = nil
		}
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
		m.streaming = false
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
		if m.streaming {
			m.livePulse = !m.livePulse
			// keep progress bar full while streaming — indeterminate runs have no ETA
			m.runProgress = 1.0
		} else {
			m.runProgress = clampProgress(m.runProgress + 0.02)
		}
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
	if m.shortcutEditMode {
		return m.handleShortcutEditKey(msg)
	}
	if m.showTagsPopup {
		return m.handleTagsPopupKey(msg)
	}
	if m.activeTab == TabEnv {
		return m.handleEnvKey(msg)
	}
	if m.searchInput.Focused() {
		return m.handleSearchKey(msg)
	}

	k := msg.String()

	// Command shortcut lookup takes precedence over the single-letter bindings
	// in KeyMap (Top=g, Bottom=G, ToggleSecrets=s, Quit=q). If a filtered
	// command has Shortcut==k, run it. Restricted to the commands tab; the env
	// tab has its own key handler.
	if m.activeTab == TabCommands && len(k) == 1 && !m.running {
		for i, cmd := range m.filtered {
			if cmd.Shortcut == k {
				m.selected = i
				return m.triggerRun()
			}
		}
	}

	switch {
	case k == "ctrl+c":
		if m.running && m.runCancel != nil {
			m.runCancel()
			m.runCancel = nil
			m.interrupted = true
			return m, nil
		}
		return m, tea.Quit
	case k == m.keys.Quit:
		return m, tea.Quit
	case k == m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % 4
	case k == m.keys.TabPrev:
		m.activeTab = (m.activeTab + 3) % 4
	case k == m.keys.Up:
		if m.selected > 0 {
			m.selected--
		}
	case k == m.keys.Down:
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
	case k == m.keys.OutputWider:
		next := m.outputWidthPct + 5
		if max := m.outputPctMax(); next > max {
			next = max
		}
		m.outputWidthPct = next
		m.recalcLayout()
		return m, nil
	case k == m.keys.OutputNarrower:
		min := 30
		next := m.outputWidthPct - 5
		if next < min {
			next = min
		}
		m.outputWidthPct = next
		m.recalcLayout()
		return m, nil
	case k == m.keys.SidebarWider:
		next := m.sidebarWidthPct + 5
		if max := m.sidebarPctMax(); next > max {
			next = max
		}
		m.sidebarWidthPct = next
		m.recalcLayout()
		return m, nil
	case k == m.keys.EditShortcut:
		if m.activeTab == TabCommands && len(m.filtered) > 0 {
			m.shortcutEditMode = true
			m.activeTab = TabCommands
		}
		return m, nil
	case k == m.keys.EditTags:
		if m.activeTab == TabCommands && len(m.filtered) > 0 {
			return m.openTagsPopup()
		}
		return m, nil
	case k == m.keys.MakefilePageUp:
		step := m.makefilePageStep()
		m.makefileOffset -= step
		if m.makefileOffset < 0 {
			m.makefileOffset = 0
		}
		return m, nil
	case k == m.keys.MakefilePageDown:
		step := m.makefilePageStep()
		maxOff := len(m.makefileLines) - step
		if maxOff < 0 {
			maxOff = 0
		}
		m.makefileOffset += step
		if m.makefileOffset > maxOff {
			m.makefileOffset = maxOff
		}
		return m, nil
	case k == m.keys.SidebarNarrower:
		min := 15
		if !m.showCenter {
			min = 30
		}
		next := m.sidebarWidthPct - 5
		if next < min {
			next = min
		}
		m.sidebarWidthPct = next
		m.recalcLayout()
		return m, nil
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

// handleShortcutEditKey captures the next keypress as the new shortcut for the
// currently selected command. Esc cancels; backspace/delete clears. Only
// single-character keys are accepted; modifier combos and named keys are ignored.
func (m Model) handleShortcutEditKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "esc":
		m.shortcutEditMode = false
		return m, nil
	case "backspace", "delete":
		m.setSelectedShortcut("")
		m.shortcutEditMode = false
		return m, nil
	}
	if len(k) != 1 {
		return m, nil
	}
	m.setSelectedShortcut(k)
	m.shortcutEditMode = false
	return m, nil
}

// setSelectedShortcut assigns newKey as the shortcut of the currently selected
// command. If another command already owns newKey, its shortcut is cleared so
// the mapping stays unique. Pass "" to remove the shortcut. The change is
// persisted back to the Makefile as a `[sc=X]` tag on the command's `##`
// docstring so it survives across sessions.
func (m *Model) setSelectedShortcut(newKey string) {
	if len(m.filtered) == 0 || m.selected >= len(m.filtered) {
		return
	}
	targetName := m.filtered[m.selected].Name
	var displaced string
	for i := range m.commands {
		if newKey != "" && m.commands[i].Shortcut == newKey && m.commands[i].Name != targetName {
			displaced = m.commands[i].Name
			m.commands[i].Shortcut = ""
		}
		if m.commands[i].Name == targetName {
			m.commands[i].Shortcut = newKey
		}
	}
	m.filtered = filterCommands(m.commands, m.search)

	if m.makefilePath != "" {
		if displaced != "" {
			_ = source.UpdateMakefileShortcut(m.makefilePath, displaced, "")
		}
		_ = source.UpdateMakefileShortcut(m.makefilePath, targetName, newKey)
		m.makefileLines = loadFileLines(m.makefilePath)
	}
}

// openTagsPopup loads the current tag state for the selected command from the
// Makefile and shows the popup. If the file can't be parsed, a zero state is
// used so the user can still add tags from scratch.
func (m Model) openTagsPopup() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.selected >= len(m.filtered) {
		return m, nil
	}
	name := m.filtered[m.selected].Name
	state, _, _ := source.ReadDocTagState(m.makefilePath, name)
	m.tagsPopupName = name
	m.tagsPopupState = state
	m.tagsPopupSel = 0
	m.showTagsPopup = true
	return m, nil
}

// handleTagsPopupKey routes input while the tag-editor popup is open. A
// second sub-mode (tagsEditing) captures text input for the `[tags=...]` CSV
// list.
func (m Model) handleTagsPopupKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if m.tagsEditing {
		switch k {
		case "esc":
			m.tagsEditing = false
			m.tagsEditBuffer = ""
		case "enter":
			return m.commitTagsEdit()
		case "backspace":
			runes := []rune(m.tagsEditBuffer)
			if len(runes) > 0 {
				m.tagsEditBuffer = string(runes[:len(runes)-1])
			}
		default:
			if len(k) == 1 {
				m.tagsEditBuffer += k
			}
		}
		return m, nil
	}

	n := len(views.TagFlagItems)
	switch k {
	case "esc", m.keys.EditTags:
		m.showTagsPopup = false
		return m, nil
	case "up", "k":
		if m.tagsPopupSel > 0 {
			m.tagsPopupSel--
		}
		return m, nil
	case "down", "j":
		if m.tagsPopupSel < n-1 {
			m.tagsPopupSel++
		}
		return m, nil
	case " ", "enter":
		return m.toggleSelectedTag()
	case "t":
		m.tagsEditing = true
		m.tagsEditBuffer = strings.Join(m.tagsPopupState.Tags, ",")
		return m, nil
	case m.keys.EditShortcut:
		m.showTagsPopup = false
		m.shortcutEditMode = true
		return m, nil
	}
	return m, nil
}

// commitTagsEdit parses the CSV buffer, writes [tags=...] to the Makefile,
// and refreshes the popup state from the source file.
func (m Model) commitTagsEdit() (tea.Model, tea.Cmd) {
	var tags []string
	for _, p := range strings.Split(m.tagsEditBuffer, ",") {
		if p = strings.TrimSpace(p); p != "" {
			tags = append(tags, p)
		}
	}
	if m.makefilePath != "" && m.tagsPopupName != "" {
		if err := source.UpdateMakefileTags(m.makefilePath, m.tagsPopupName, tags); err == nil {
			m.makefileLines = loadFileLines(m.makefilePath)
		}
		if state, _, err := source.ReadDocTagState(m.makefilePath, m.tagsPopupName); err == nil {
			m.tagsPopupState = state
			m.applyTagsToCommand(m.tagsPopupName, state.Tags)
			m.filtered = filterCommands(m.commands, m.search)
		}
	}
	m.tagsEditing = false
	m.tagsEditBuffer = ""
	return m, nil
}

// applyTagsToCommand mirrors the fresh category tags onto the in-memory
// command so chips in the sidebar and center header refresh immediately.
func (m *Model) applyTagsToCommand(name string, tags []string) {
	for i := range m.commands {
		if m.commands[i].Name == name {
			m.commands[i].Tags = tags
			return
		}
	}
}

// toggleSelectedTag flips the tag currently highlighted in the popup, persists
// the change to the Makefile, and reloads in-memory state so both the popup
// and the sidebar reflect the new source of truth.
func (m Model) toggleSelectedTag() (tea.Model, tea.Cmd) {
	if m.tagsPopupSel < 0 || m.tagsPopupSel >= len(views.TagFlagItems) {
		return m, nil
	}
	item := views.TagFlagItems[m.tagsPopupSel]
	currentlyOn := flagOn(m.tagsPopupState, item.Flag)
	newOn := !currentlyOn

	if m.makefilePath != "" && m.tagsPopupName != "" {
		if err := source.UpdateMakefileFlag(m.makefilePath, m.tagsPopupName, item.Flag, newOn); err == nil {
			m.makefileLines = loadFileLines(m.makefilePath)
		}
		if state, _, err := source.ReadDocTagState(m.makefilePath, m.tagsPopupName); err == nil {
			m.tagsPopupState = state
			// Reflect the flag change on the in-memory command so runtime
			// behavior (triggerRun, Stream dispatch) matches the source.
			m.applyDocStateToCommand(m.tagsPopupName, state)
			m.filtered = filterCommands(m.commands, m.search)
		}
	}
	return m, nil
}

// applyDocStateToCommand copies the runtime-relevant fields (Confirm,
// NoConfirm, Stream) from the parsed doc state onto the matching command.
// Shortcut is intentionally left alone so .cast.toml overrides survive.
func (m *Model) applyDocStateToCommand(name string, state source.DocTagState) {
	for i := range m.commands {
		if m.commands[i].Name != name {
			continue
		}
		m.commands[i].Confirm = state.Confirm
		m.commands[i].NoConfirm = state.NoConfirm
		if state.StreamSet {
			m.commands[i].Stream = state.Stream
		}
		return
	}
}

// flagOn mirrors the popup's checkbox-state logic so the handler and the view
// stay in agreement about what "on" means for each flag.
func flagOn(s source.DocTagState, flag string) bool {
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

// makefilePageStep returns how many lines pgup/pgdown should shift the preview.
// Scaled to roughly half the center panel height so scrolling feels proportional.
func (m Model) makefilePageStep() int {
	step := (m.height - headerH - statusH) / 2
	if step < 5 {
		step = 5
	}
	return step
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
	case "ctrl+c":
		// Cancel the running command but keep the popup open so the user can
		// review the trailing output.
		if m.running && m.runCancel != nil {
			m.runCancel()
			m.runCancel = nil
			m.interrupted = true
			return m, nil
		}
		return m, tea.Quit
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
	case "ctrl+c":
		if m.running && m.runCancel != nil {
			m.runCancel()
			m.runCancel = nil
			m.interrupted = true
			return m, nil
		}
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case "tab", m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % 4
	case m.keys.TabPrev:
		m.activeTab = (m.activeTab + 3) % 4
	case "left", "h":
		m.envFocus = 0
	case "right", "l":
		m.envFocus = 1
	case m.keys.Up, "k":
		m.envNavUp()
	case m.keys.Down, "j":
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
	// Confirmation precedence (top wins):
	//   1. [no-confirm] tag in the Makefile   → never ask, even in prod/staging
	//   2. [confirm] tag                      → always ask, any env
	//   3. env != dev                         → ask by default
	switch {
	case cmd.NoConfirm:
		// fall through to dispatch
	case cmd.Confirm, m.env != config.EnvLocal:
		m.showConfirm = true
		return m, nil
	}
	return m.dispatchRun()
}

func (m Model) dispatchRun() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	cmdMeta := m.filtered[m.selected]
	name := cmdMeta.Name
	ctx, cancel := context.WithCancel(context.Background())
	ch := runner.StreamRun(ctx, name)
	m.streamCh = ch
	m.runCancel = cancel
	stream := cmdMeta.Stream
	startCmd := func() tea.Msg { return RunStartMsg{Command: name, Stream: stream} }
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
	outputW := m.outputPanelW()
	sidebarW := m.sidebarPanelW()
	centerW := m.width - sidebarW - outputW - borders
	if !m.showCenter {
		centerW = 0
	} else if centerW < 10 {
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
