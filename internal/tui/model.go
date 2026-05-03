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
	"github.com/Cerebellum-ITM/cast/internal/library"
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
	TabLibrary
)

// tabCount is the number of top-level tabs. Tab navigation (Tab / Shift+Tab)
// uses this to wrap. Bumping this constant + adding a Tab const + updating
// the names array in renderTabs is the only change required to add a tab.
const tabCount = 5

// AppState tracks whether we are in the splash or the main TUI.
type AppState int

const (
	StateSplash AppState = iota
	StateMain
)

// AppMode toggles what the sidebar and history tab show:
//   - ModeSingle: individual commands (sidebar) and per-run history (tab).
//   - ModeChain : saved chains (sidebar) and per-chain-execution history.
// Auto-queueing via shortcuts still creates chains in both modes.
type AppMode int

const (
	ModeSingle AppMode = iota
	ModeChain
)

// --- Messages ----------------------------------------------------------------

// SplashDoneMsg is emitted by the splash model when the animation completes.
type SplashDoneMsg struct{}

// RunStartMsg signals that command execution has begun.
type RunStartMsg struct {
	Command     string
	Stream      bool
	Interactive bool
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

// ChainsLoadedMsg carries auto-saved chains: deduplicated summaries (for the
// sidebar in chain mode) and individual executions (for the history tab).
type ChainsLoadedMsg struct {
	Chains []db.SequenceSummary
	Runs   []db.ChainRunRecord
}

// EnvHistoryLoadedMsg carries env change records loaded from the database.
type EnvHistoryLoadedMsg struct{ Changes []db.EnvChange }

// EnvChangedMsg is dispatched after a successful env var write.
type EnvChangedMsg struct{ Key string }

// HistoryErrorMsg reports a non-fatal DB error (load or insert).
type HistoryErrorMsg struct{ Err error }

// tickMsg drives the progress bar animation.
type tickMsg struct{}

// clearNoticeMsg is sent by a one-shot tea.Tick to dismiss a status-bar
// notice after a few seconds. The id field guards against stale clears: if
// a newer notice has been posted since the timer was scheduled, the new
// notice's id will differ and the clear is ignored.
type clearNoticeMsg struct{ id int64 }

// noticeTTL is how long a notice persists before auto-fading. Long enough
// to read at a glance, short enough to not linger across actions.
const noticeTTL = 4 * time.Second

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
	makefileDir    string // dirname(makefilePath); passed as `make -C <dir>`
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
	iconStyle views.IconStyle

	// Execution state
	running     bool
	runProgress float64
	output      []string
	showConfirm bool
	// confirmModalSel: 0 = cancel button, 1 = confirm button. Reset to 1 each
	// time the modal opens so default-Enter still confirms.
	confirmModalSel int

	// Delete-command popup. Shown when the user presses DeleteCommand on
	// the commands tab. The selected command is captured at open time so
	// later cursor moves don't change which target gets removed.
	showDeleteCmdConfirm bool
	deleteCmdConfirmSel  int    // 0 cancel, 1 confirm
	deleteCmdName        string
	lastRunCmd  string // current/most-recent step name (used by output header & history)
	// lastRunCommands is the ordered list of targets dispatched on the user's
	// most recent action: one name for a single run, ≥2 for a chain. This is
	// what the rerun card replays; lastRunCmd is per-step and not safe for
	// rerunning a chain (it gets overwritten as steps progress).
	lastRunCommands []string
	// lastRunExtraVars caches the picker-resolved KEY=VAL pairs from the most
	// recent dispatch so RerunLast can skip the picker and reuse them. Empty
	// for plain (non-pick) commands and for chains (chains skip the picker).
	lastRunExtraVars []string
	// pendingRerunExtras is set when a rerun needs to traverse the confirm
	// modal: stash the extras here so the modal's "yes" path can dispatch
	// with them instead of re-running through triggerRun (which would reopen
	// the picker).
	pendingRerunExtras []string
	// pendingRerunChain holds the chain target list when a chain-rerun has
	// to traverse the confirm modal. Yes-path dispatches via
	// startChainFromCommands; no-path clears it.
	pendingRerunChain  []string
	lastRunOK   bool
	hasLastRun  bool
	// rerunFocused is true when the pinned "rerun" card at the top of the
	// sidebar holds keyboard focus. Up from filtered[0] enters this mode;
	// Down leaves it. Independent of m.selected so command indexing across
	// the rest of the model stays untouched.
	rerunFocused bool
	streamCh    <-chan tea.Msg
	runCancel   context.CancelFunc // non-nil while a run is active
	streaming   bool               // current run is a long-lived log stream
	livePulse   bool               // flipped each tick for LIVE dot animation
	interrupted bool               // current/last run was manually canceled
	// quitPending is set when the user pressed Quit while a command was
	// running. For [stream] commands the active run is cancelled
	// immediately (registered as interrupted in history); for everything
	// else a quitSentinel step is appended to the chain, so Quit waits for
	// queued work to finish. Either way, tea.Quit is fired from the
	// runner.DoneMsg handler when the chain finally drains.
	quitPending bool

	// Chain / queue state. A chain is any run where len(chainCommands) > 1.
	// While any command is running, additional shortcuts/triggers append to
	// chainCommands and execute after the current step finishes (abort on
	// failure). The chain is persisted in the DB only after it completes.
	chainCommands []string    // full ordered list of steps in the active chain
	chainStepIdx  int         // index of the currently-running step; -1 idle
	chainRunIDs   []int64     // runs.id for each completed step (len == stepIdx on Done)
	chainStartAt  time.Time   // when step 0 started; used for sequence duration

	// App mode: toggles between single-command view and saved-chain view.
	mode            AppMode
	chains          []db.SequenceSummary  // deduped chains, newest first
	chainRuns       []db.ChainRunRecord   // individual chain executions for history
	chainSel        int                   // cursor in the chain sidebar
	chainHistoryMax int

	// Cursors for the History tab. The tab renders one of two tables
	// depending on m.mode; each gets its own cursor so switching modes
	// doesn't stomp the other selection.
	historySel      int // index into m.history (single mode)
	chainHistorySel int // index into m.chainRuns (chain mode)

	// Explicit chain-builder (ctrl+b from single mode): user picks N commands
	// via space/shortcut, Enter dispatches the chain. Selection order drives
	// execution order.
	chainBuilder bool
	chainChecked []string

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

	// Theme tab state.
	themeTabSel int    // 0..len(themes)-1
	themeError  string // last error from saving the theme; "" when clean
	savedTheme  config.Theme // theme value currently persisted in .cast.toml; "" if none

	// Status-bar notice. Set via setNotice() to surface internal events
	// (snippet saved, theme persisted, etc.) globally — visible from any
	// tab without needing to switch back to where the action originated.
	// noticeID is the freshness counter: each new notice bumps it so a
	// pending auto-clear tick from a previous notice is ignored.
	notice     string
	noticeKind int
	noticeID   int64

	// Library tab state. Snippets are loaded once at New() and refreshed
	// after insert/extract/delete operations. The fuzzy search input lives
	// alongside the global search input but is scoped to library only.
	librarySnippets       []library.Snippet
	libraryFiltered       []library.Snippet
	librarySel            int
	librarySearchInput    textinput.Model
	libraryError          string // sticky error banner (clear on next action)
	libraryFeedback       string // sticky success banner ("inserted X")
	libraryConfirmDelete  bool   // armed by `d`, committed by second `d`/Enter

	// Folder-picker popup. Active when a command tagged with [pick=…] is
	// triggered. Steps run sequentially; each selection accumulates into
	// pickerSelections and pickerExtraVars (KEY=VAL). Cancellation aborts the
	// whole flow.
	showPicker        bool
	pickerCmd         source.Command
	pickerStep        int
	pickerSelections  []string
	pickerExtraVars   []string
	pickerEntries     []views.PickerEntry
	pickerEntriesAll  []views.PickerEntry
	pickerCursor      int
	pickerSearch      string
	pickerBase        string

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

	// Detect whether the local .cast.toml currently pins a theme so the
	// Theme tab can show "saved" vs "active (unsaved)" states.
	var savedTheme config.Theme
	if local, ok := config.LoadLocal(); ok && local.Theme != "" {
		savedTheme = config.Theme(local.Theme)
	}

	// Snippets library: cargar al iniciar. Errores no son fatales; un
	// directorio inexistente o ilegible deja la lista vacía.
	snippets, _ := library.List()

	libSearch := textinput.New()
	libSearch.Placeholder = "search snippets…"
	libSearch.CharLimit = 64

	var lastRunCmds []string
	var lastRunExtras []string
	if database != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if lr, err := database.GetLastRun(ctx, cfg.SourceDir); err == nil && lr != nil {
			lastRunCmds = lr.Commands
			lastRunExtras = lr.ExtraVars
		}
		cancel()
	}

	return Model{
		state:           StateSplash,
		lastRunCommands: lastRunCmds,
		lastRunExtraVars: lastRunExtras,
		hasLastRun:      len(lastRunCmds) > 0,
		keys:            DefaultKeyMap,
		splashModel:     splash.New(cfg.Theme, cfg.Env),
		commands:        commands,
		filtered:        commands,
		historyMax:      cfg.HistoryMax,
		db:              database,
		env:             cfg.Env,
		theme:           cfg.Theme,
		savedTheme:      savedTheme,
		themeTabSel:     themeIndex(cfg.Theme),
		librarySnippets: snippets,
		libraryFiltered: snippets,
		librarySearchInput: libSearch,
		iconStyle:       views.ParseIconStyle(cfg.IconStyle),
		searchInput:     si,
		envSearchInput:  esi,
		spinner:         sp,
		progressBar:     pb,
		makefileLines:   loadFileLines(cfg.SourcePath),
		makefilePath:    cfg.SourcePath,
		makefileDir:     cfg.SourceDir,
		envFile:         envFile,
		envFilePath:     cfg.EnvFilePath,
		outputWidthPct:  cfg.OutputWidthPct,
		sidebarWidthPct: cfg.SidebarWidthPct,
		showCenter:      cfg.ShowCenterPanel,
		chainStepIdx:    -1,
		chainHistoryMax: cfg.ChainHistoryMax,
	}
}

// syncMakefilePreviewToSelection scrolls the inline Makefile preview so the
// currently selected command's target lines are visible at the top. Called
// whenever m.selected (or the filtered slice) changes so the preview follows
// the cursor in the sidebar. No-op when no command is selected or the target
// can't be located in the loaded Makefile.
func (m *Model) syncMakefilePreviewToSelection() {
	if len(m.filtered) == 0 || m.selected < 0 || m.selected >= len(m.filtered) {
		return
	}
	idx := source.MakefileTargetLineIndex(m.makefileLines, m.filtered[m.selected].Name)
	if idx < 0 {
		return
	}
	m.makefileOffset = idx
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
	return tea.Batch(m.splashModel.Init(), m.loadHistoryCmd(), m.loadEnvHistoryCmd(), m.loadChainsCmd())
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
		m.syncMakefilePreviewToSelection()
		return m, nil

	case tea.KeyPressMsg:
		if m.state == StateSplash {
			m.state = StateMain
			m.syncMakefilePreviewToSelection()
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
		m.chainRunIDs = append(m.chainRunIDs, run.ID)

		// Prepend to history regardless of whether the chain continues so
		// intermediate steps are visible in the Runs panel in real time.
		m.history = append([]db.Run{run}, m.history...)
		if m.historyMax > 0 && len(m.history) > m.historyMax {
			m.history = m.history[:m.historyMax]
		}
		m.historySel = 0

		stepSucceeded := run.Status == db.StatusSuccess
		if stepSucceeded {
			if nextM, nextCmd, ok := m.advanceChain(); ok {
				return nextM, nextCmd
			}
		}
		// Chain ended (last step done, or a step failed/was interrupted).
		// A chain is anything that ran ≥ 2 steps — including auto-queued ones
		// where the user pressed shortcuts mid-run. Persist the full intent
		// (m.chainCommands, not the executed prefix) as both the sequence
		// record AND the rerun card target so Ctrl+R replays the whole
		// chain even if a middle step failed.
		if len(m.chainRunIDs) >= 2 {
			m.persistChain(run.Status)
			// Filter the sentinel before persisting "last run" so Ctrl+R
			// re-runs the real steps and not the queued exit.
			persistCmds := make([]string, 0, len(m.chainCommands))
			for _, c := range m.chainCommands {
				if c == quitSentinel {
					continue
				}
				persistCmds = append(persistCmds, c)
			}
			m.lastRunCommands = persistCmds
			m.lastRunExtraVars = m.lastRunExtraVars[:0]
			m.hasLastRun = true
			m.persistLastRun(persistCmds, nil)
		}
		m.chainCommands = nil
		m.chainStepIdx = -1
		m.chainRunIDs = m.chainRunIDs[:0]
		if m.quitPending {
			// User pressed Quit during this chain. Whether the sentinel
			// was reached organically (drained advanceChain) or the chain
			// died early (failure/interrupt discarded the rest), exit
			// once the persisted-state work is done.
			m.quitPending = false
			m.running = false
			m.streaming = false
			return m, tea.Quit
		}
		loadChains := m.loadChainsCmd()
		done := func() tea.Msg { return RunDoneMsg{Status: run.Status, Run: run} }
		if loadChains == nil {
			return m, done
		}
		return m, tea.Batch(done, loadChains)

	case RunDoneMsg:
		m.running = false
		m.streaming = false
		m.runProgress = 1.0
		m.hasLastRun = true
		m.lastRunOK = msg.Status == db.StatusSuccess
		// History already prepended per-step in the runner.DoneMsg handler.
		return m, nil

	case ChainsLoadedMsg:
		m.chains = msg.Chains
		m.chainRuns = msg.Runs
		if m.chainSel >= len(m.chains) {
			m.chainSel = 0
		}
		if m.chainHistorySel >= len(m.chainRuns) {
			m.chainHistorySel = 0
		}
		return m, nil

	case HistoryLoadedMsg:
		m.history = msg.Runs
		if m.historySel >= len(m.history) {
			m.historySel = 0
		}
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

	case clearNoticeMsg:
		// Only clear if no newer notice has superseded this one. Compare
		// ids so a fast sequence of setNotice calls doesn't blink the
		// status bar — the latest notice always wins.
		if msg.id == m.noticeID {
			m.notice = ""
		}
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

// quitSentinel is a synthetic chain step that means "exit cast". It is
// appended to m.chainCommands when the user presses Quit during a non-stream
// run, so the queued exit waits for any in-flight or pending steps to finish
// before tearing down the program. advanceChain intercepts the value before
// it can be dispatched as a make target.
const quitSentinel = "__cast:quit__"

// quitOrCancel handles the Quit keystroke.
//
//   - Idle: exits immediately.
//   - Stream run: cancels the active process (SIGINT → SIGKILL via the runner
//     watchdog) and lets the resulting DoneMsg flow through normally so the
//     run is recorded as interrupted in history. The DoneMsg handler then
//     fires tea.Quit because m.quitPending is true.
//   - Non-stream run: appends quitSentinel to m.chainCommands so the exit
//     queues behind the running step and any commands the user enqueued with
//     Enter mid-run. advanceChain catches the sentinel and emits tea.Quit.
//   - Interactive (running but no runCancel): can normally not be reached
//     because tea.ExecProcess suspends the TUI; if it ever is, fall through
//     to plain tea.Quit since the child either already exited or owns stdin.
//
// Idempotent: a second Quit while quitPending is already set is a no-op so
// double-tapping `q` does not escalate into a process kill on a non-stream
// run.
func (m Model) quitOrCancel() (tea.Model, tea.Cmd) {
	if !m.running {
		return m, tea.Quit
	}
	if m.quitPending {
		return m, nil
	}
	m.quitPending = true
	if m.streaming && m.runCancel != nil {
		m.runCancel()
		m.runCancel = nil
		m.interrupted = true
		notice := m.setNotice("quit pending — cancelling stream", views.NoticeInfo)
		return m, notice
	}
	if m.runCancel == nil {
		// Defensive: running but no cancel handle (interactive transition).
		// Nothing to wait on — just exit.
		return m, tea.Quit
	}
	if m.chainStepIdx < 0 {
		// A solo command is running without a chain context yet. Seed the
		// chain so the running step is at index 0 and the sentinel sits at
		// index 1, ready for advanceChain to pick up on DoneMsg.
		m.chainCommands = []string{m.lastRunCmd}
		m.chainStepIdx = 0
	}
	m.chainCommands = append(m.chainCommands, quitSentinel)
	notice := m.setNotice("quit queued — exits when chain finishes", views.NoticeInfo)
	return m, notice
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.showPicker {
		return m.handlePickerKey(msg)
	}
	if m.showConfirm {
		return m.handleConfirmModal(msg)
	}
	if m.showDeleteCmdConfirm {
		return m.handleDeleteCmdModal(msg)
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
	if m.activeTab == TabTheme {
		return m.handleThemeKey(msg)
	}
	if m.activeTab == TabLibrary {
		return m.handleLibraryKey(msg)
	}
	if m.searchInput.Focused() {
		return m.handleSearchKey(msg)
	}

	k := msg.String()

	// Command shortcut lookup takes precedence over the single-letter bindings
	// in KeyMap (Top=g, Bottom=G, ToggleSecrets=s, Quit=q). If a filtered
	// command has Shortcut==k, run it. Restricted to the commands tab; the env
	// tab has its own key handler.
	// Single-letter shortcut lookup for commands. In single mode it runs (or
	// auto-queues while running); in chain mode the letter keys don't bind,
	// so chain mode falls straight through to the nav/mode logic below.
	// When the builder is active, shortcuts toggle selection instead of
	// triggering a run.
	if m.activeTab == TabCommands && m.mode == ModeSingle && len(k) == 1 {
		for i, cmd := range m.filtered {
			if cmd.Shortcut == k {
				if m.chainBuilder {
					m.toggleChainSelection(cmd.Name)
					return m, nil
				}
				if m.running {
					m.enqueueCommand(cmd.Name)
					return m, nil
				}
				m.selected = i
				m.syncMakefilePreviewToSelection()
				return m.triggerRun()
			}
		}
	}

	// Mode toggle works from any tab — flips sidebar and history view.
	if k == m.keys.ModeToggle {
		if m.mode == ModeSingle {
			m.mode = ModeChain
		} else {
			m.mode = ModeSingle
		}
		// Leaving single mode also tears down the builder so it doesn't
		// "ghost" in chain mode where it has no meaning.
		m.chainBuilder = false
		m.chainChecked = nil
		return m, nil
	}

	// Builder toggle — only meaningful in single mode on the commands tab.
	if m.activeTab == TabCommands && m.mode == ModeSingle && k == m.keys.ChainBuilder {
		m.chainBuilder = !m.chainBuilder
		if !m.chainBuilder {
			m.chainChecked = nil
		}
		return m, nil
	}

	// Inside builder: space toggles current row, Enter dispatches the chain,
	// Esc cancels. Up/Down still navigate normally (main switch handles it).
	if m.activeTab == TabCommands && m.mode == ModeSingle && m.chainBuilder {
		switch k {
		case "space", " ":
			if len(m.filtered) > 0 {
				m.toggleChainSelection(m.filtered[m.selected].Name)
			}
			return m, nil
		case "esc":
			m.chainBuilder = false
			m.chainChecked = nil
			return m, nil
		case m.keys.Run, m.keys.RunAlt:
			if len(m.chainChecked) == 0 {
				return m, nil
			}
			return m.startChainFromSelection()
		}
	}

	// History tab is its own full-width view with a navigable table —
	// the sidebar is hidden, so navigation keys drive the row cursor and
	// Enter re-runs the highlighted row (single command or full chain).
	if m.activeTab == TabHistory {
		if m.mode == ModeChain {
			n := len(m.chainRuns)
			switch k {
			case m.keys.Up:
				if m.chainHistorySel > 0 {
					m.chainHistorySel--
				}
				return m, nil
			case m.keys.Down:
				if m.chainHistorySel < n-1 {
					m.chainHistorySel++
				}
				return m, nil
			case m.keys.Top:
				m.chainHistorySel = 0
				return m, nil
			case m.keys.Bottom:
				if n > 0 {
					m.chainHistorySel = n - 1
				}
				return m, nil
			case m.keys.Run, m.keys.RunAlt:
				if m.running {
					return m, nil
				}
				if m.chainHistorySel >= n {
					return m, nil
				}
				cmds := m.chainRuns[m.chainHistorySel].Commands
				if len(cmds) == 0 {
					return m, nil
				}
				return m.startChainFromCommands(cmds)
			}
		} else {
			n := len(m.history)
			switch k {
			case m.keys.Up:
				if m.historySel > 0 {
					m.historySel--
				}
				return m, nil
			case m.keys.Down:
				if m.historySel < n-1 {
					m.historySel++
				}
				return m, nil
			case m.keys.Top:
				m.historySel = 0
				return m, nil
			case m.keys.Bottom:
				if n > 0 {
					m.historySel = n - 1
				}
				return m, nil
			case m.keys.Run, m.keys.RunAlt:
				if m.historySel >= n {
					return m, nil
				}
				name := m.history[m.historySel].Command
				if m.running {
					m.enqueueCommand(name)
					return m, nil
				}
				cmd := m.findCommand(name)
				if cmd == nil {
					notice := m.setNotice("'"+name+"' is no longer in the Makefile", views.NoticeError)
					return m, notice
				}
				return m.startSingleRun(*cmd, nil)
			}
		}
	}

	// Chain-mode sidebar navigation and re-run on the commands tab.
	if m.activeTab == TabCommands && m.mode == ModeChain {
		switch k {
		case m.keys.Up:
			if m.chainSel > 0 {
				m.chainSel--
			}
			return m, nil
		case m.keys.Down:
			if m.chainSel < len(m.chains)-1 {
				m.chainSel++
			}
			return m, nil
		case m.keys.Run, m.keys.RunAlt:
			if !m.running && m.chainSel < len(m.chains) && len(m.chains[m.chainSel].Commands) > 0 {
				return m.startChainFromCommands(m.chains[m.chainSel].Commands)
			}
			return m, nil
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
		return m.quitOrCancel()
	case k == m.keys.QuitAlt:
		return m.quitOrCancel()
	case k == m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % tabCount
	case k == m.keys.TabPrev:
		m.activeTab = (m.activeTab + tabCount - 1) % tabCount
	case k == m.keys.Up:
		if m.rerunFocused {
			// already at the very top
		} else if m.selected == 0 && m.hasRerunCard() {
			m.rerunFocused = true
		} else if m.selected > 0 {
			m.selected--
			m.syncMakefilePreviewToSelection()
		}
	case k == m.keys.Down:
		if m.rerunFocused {
			m.rerunFocused = false
		} else if m.selected < len(m.filtered)-1 {
			m.selected++
			m.syncMakefilePreviewToSelection()
		}
	case k == m.keys.Top:
		if m.hasRerunCard() {
			m.rerunFocused = true
		} else {
			m.selected = 0
			m.syncMakefilePreviewToSelection()
		}
	case k == m.keys.Bottom:
		m.rerunFocused = false
		if len(m.filtered) > 0 {
			m.selected = len(m.filtered) - 1
			m.syncMakefilePreviewToSelection()
		}
	case k == m.keys.Search:
		m.searchInput.Focus()
		return m, textinput.Blink
	case k == m.keys.RerunLast:
		// Must come before Run/RunAlt: when both share a binding (default
		// ctrl+r), the rerun semantic wins so picker-typed commands don't
		// reopen the picker on Ctrl+R.
		if !m.running {
			return m.rerunLast()
		}
	case k == m.keys.Run, k == m.keys.RunAlt:
		if m.rerunFocused {
			return m.rerunLast()
		}
		return m.triggerRun()
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
	case k == m.keys.ExtractSnippet:
		if m.activeTab == TabCommands && len(m.filtered) > 0 {
			return m.extractCurrentToLibrary()
		}
		return m, nil
	case k == m.keys.DeleteCommand:
		if m.activeTab == TabCommands && m.mode == ModeSingle && !m.chainBuilder &&
			!m.running && len(m.filtered) > 0 && m.selected < len(m.filtered) {
			m.deleteCmdName = m.filtered[m.selected].Name
			m.deleteCmdConfirmSel = 1
			m.showDeleteCmdConfirm = true
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

// themeOrder is the display order of selectable themes in the Theme tab.
// Add new themes here when palettes are added in styles.go.
var themeOrder = []config.Theme{
	config.ThemeCatppuccin,
	config.ThemeDracula,
	config.ThemeNord,
}

// themeIndex returns the slot of t in themeOrder, or 0 when not found.
func themeIndex(t config.Theme) int {
	for i, x := range themeOrder {
		if x == t {
			return i
		}
	}
	return 0
}

// handleThemeKey routes input while the Theme tab is active. Up/Down move
// the cursor with live preview; Enter persists the selection to the local
// .cast.toml.
func (m Model) handleThemeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		m.activeTab = TabCommands
		return m, nil
	case m.keys.QuitAlt:
		return m.quitOrCancel()
	case "tab", m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % tabCount
		return m, nil
	case m.keys.TabPrev:
		m.activeTab = (m.activeTab + tabCount - 1) % tabCount
		return m, nil
	case m.keys.Up, "k":
		if m.themeTabSel > 0 {
			m.themeTabSel--
			m.theme = themeOrder[m.themeTabSel] // live preview
			m.themeError = ""
		}
		return m, nil
	case m.keys.Down, "j":
		if m.themeTabSel < len(themeOrder)-1 {
			m.themeTabSel++
			m.theme = themeOrder[m.themeTabSel]
			m.themeError = ""
		}
		return m, nil
	case "enter":
		return m.commitThemeSelection()
	}
	return m, nil
}

// commitThemeSelection writes the current cursor's theme to the local
// .cast.toml so the choice survives across sessions. Errors are surfaced
// in m.themeError so the view can render a red banner.
func (m Model) commitThemeSelection() (tea.Model, tea.Cmd) {
	if m.themeTabSel < 0 || m.themeTabSel >= len(themeOrder) {
		return m, nil
	}
	chosen := themeOrder[m.themeTabSel]
	path := config.LocalPath()
	if err := config.WriteLocalTheme(path, string(chosen)); err != nil {
		m.themeError = err.Error()
		notice := m.setNotice("theme save failed: "+err.Error(), views.NoticeError)
		return m, notice
	}
	m.theme = chosen
	m.savedTheme = chosen
	m.themeError = ""
	notice := m.setNotice("theme '"+string(chosen)+"' saved to .cast.toml", views.NoticeSuccess)
	return m, notice
}

// --- Status-bar notices -----------------------------------------------------

// setNotice posts a transient toast in the bottom status bar. Returns the
// tea.Cmd that schedules the auto-clear so callers can chain it via
// tea.Batch when they also need to dispatch other commands.
//
// Successive calls overwrite the previous notice and bump noticeID; the
// pending clear from any earlier notice becomes a no-op when its
// clearNoticeMsg arrives.
func (m *Model) setNotice(text string, kind views.NoticeKind) tea.Cmd {
	m.noticeID++
	id := m.noticeID
	m.notice = text
	m.noticeKind = int(kind)
	return tea.Tick(noticeTTL, func(time.Time) tea.Msg {
		return clearNoticeMsg{id: id}
	})
}

// --- Library tab ------------------------------------------------------------

// handleLibraryKey routes input while the Library tab is active.
//
// Navigation:  ↑/↓/j/k       move, /  focus search, esc   close tab
// Actions:     ⏎ insert      d  delete (twice to confirm), tab  next tab
//
// All operations refresh state on success so the sidebar (and any future
// surface that shows snippets) stays in sync.
func (m Model) handleLibraryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if m.librarySearchInput.Focused() {
		switch k {
		case "esc":
			m.librarySearchInput.Blur()
			m.librarySearchInput.SetValue("")
			m.refilterLibrary()
			return m, nil
		case "enter":
			m.librarySearchInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.librarySearchInput, cmd = m.librarySearchInput.Update(msg)
		m.refilterLibrary()
		return m, cmd
	}

	switch k {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.libraryConfirmDelete {
			m.libraryConfirmDelete = false
			return m, nil
		}
		m.activeTab = TabCommands
		return m, nil
	case "q", m.keys.QuitAlt:
		return m.quitOrCancel()
	case "tab", m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % tabCount
		return m, nil
	case m.keys.TabPrev:
		m.activeTab = (m.activeTab + tabCount - 1) % tabCount
		return m, nil
	case m.keys.Up, "k":
		if m.librarySel > 0 {
			m.librarySel--
		}
		m.libraryConfirmDelete = false
		return m, nil
	case m.keys.Down, "j":
		if m.librarySel < len(m.libraryFiltered)-1 {
			m.librarySel++
		}
		m.libraryConfirmDelete = false
		return m, nil
	case m.keys.Top:
		m.librarySel = 0
		return m, nil
	case m.keys.Bottom:
		if len(m.libraryFiltered) > 0 {
			m.librarySel = len(m.libraryFiltered) - 1
		}
		return m, nil
	case m.keys.Search:
		m.librarySearchInput.Focus()
		return m, textinput.Blink
	case "enter":
		if m.libraryConfirmDelete {
			return m.commitLibraryDelete()
		}
		return m.insertSelectedSnippet()
	case "d":
		if m.libraryConfirmDelete {
			return m.commitLibraryDelete()
		}
		if len(m.libraryFiltered) > 0 {
			m.libraryConfirmDelete = true
			m.libraryError = ""
			m.libraryFeedback = ""
		}
		return m, nil
	}
	return m, nil
}

// refilterLibrary rebuilds m.libraryFiltered from m.librarySnippets using
// the current search query. Empty query → full list. Match is a
// case-insensitive substring on Name or Desc, mirroring sidebar's
// commandMatches semantics.
func (m *Model) refilterLibrary() {
	q := m.librarySearchInput.Value()
	if q == "" {
		m.libraryFiltered = m.librarySnippets
	} else {
		var out []library.Snippet
		for _, s := range m.librarySnippets {
			if containsFold(s.Name, q) || containsFold(s.Desc, q) {
				out = append(out, s)
			}
		}
		m.libraryFiltered = out
	}
	if m.librarySel >= len(m.libraryFiltered) {
		m.librarySel = 0
	}
}

// reloadLibrary refreshes both the unfiltered slice and the filtered view.
// Called after every insert/extract/delete.
func (m *Model) reloadLibrary() {
	if snips, err := library.List(); err == nil {
		m.librarySnippets = snips
	}
	m.refilterLibrary()
}

// insertSelectedSnippet appends the focused snippet's body to the current
// Makefile and reloads cast's command list so the new target appears
// immediately. ErrTargetExists is surfaced as a sticky error banner.
func (m Model) insertSelectedSnippet() (tea.Model, tea.Cmd) {
	if len(m.libraryFiltered) == 0 || m.librarySel >= len(m.libraryFiltered) {
		return m, nil
	}
	if m.makefilePath == "" {
		m.libraryError = "no Makefile loaded"
		return m, nil
	}
	snip := m.libraryFiltered[m.librarySel]
	if err := source.AppendMakefileTarget(m.makefilePath, snip.Body); err != nil {
		m.libraryError = err.Error()
		m.libraryFeedback = ""
		cmd := m.setNotice("insert failed: "+err.Error(), views.NoticeError)
		return m, cmd
	}
	m.libraryFeedback = "inserted '" + snip.Name + "' into Makefile"
	m.libraryError = ""
	// Re-parse Makefile so the new target shows up in the sidebar.
	src := &source.MakefileSource{Path: m.makefilePath}
	if cmds, err := src.Load(); err == nil {
		m.commands = cmds
		m.filtered = filterCommands(m.commands, m.search)
	}
	m.makefileLines = loadFileLines(m.makefilePath)
	m.activeTab = TabCommands
	cmd := m.setNotice("inserted '"+snip.Name+"'", views.NoticeSuccess)
	return m, cmd
}

// extractCurrentToLibrary captures the target highlighted in the commands
// tab and saves it as a snippet under ~/.config/cast/snippets/. Reloads
// the library list so the new entry is visible immediately.
func (m Model) extractCurrentToLibrary() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.selected >= len(m.filtered) {
		return m, nil
	}
	cmd := m.filtered[m.selected]
	body, err := source.ExtractMakefileTarget(m.makefilePath, cmd.Name)
	if err != nil {
		m.libraryError = err.Error()
		notice := m.setNotice("extract failed: "+err.Error(), views.NoticeError)
		return m, notice
	}
	if err := library.Save(library.Snippet{Name: cmd.Name, Body: body}); err != nil {
		m.libraryError = err.Error()
		notice := m.setNotice("save failed: "+err.Error(), views.NoticeError)
		return m, notice
	}
	m.libraryFeedback = "saved '" + cmd.Name + "' to library"
	m.libraryError = ""
	m.reloadLibrary()
	notice := m.setNotice("saved '"+cmd.Name+"' to snippets library", views.NoticeSuccess)
	return m, notice
}

// commitLibraryDelete deletes the currently focused snippet from disk.
// Idempotent on repeat presses (file already gone → silently re-list).
func (m Model) commitLibraryDelete() (tea.Model, tea.Cmd) {
	if len(m.libraryFiltered) == 0 || m.librarySel >= len(m.libraryFiltered) {
		m.libraryConfirmDelete = false
		return m, nil
	}
	target := m.libraryFiltered[m.librarySel].Name
	if err := library.Delete(target); err != nil {
		m.libraryError = err.Error()
		m.libraryConfirmDelete = false
		notice := m.setNotice("delete failed: "+err.Error(), views.NoticeError)
		return m, notice
	}
	m.libraryFeedback = "deleted '" + target + "'"
	m.libraryError = ""
	m.libraryConfirmDelete = false
	m.reloadLibrary()
	notice := m.setNotice("deleted '"+target+"' from library", views.NoticeInfo)
	return m, notice
}

// hasRerunCard reports whether the pinned rerun card should be visible at
// the top of the sidebar. Hidden in chain mode (where the sidebar lists
// chains, not commands) and on tabs other than commands.
func (m Model) hasRerunCard() bool {
	return m.activeTab == TabCommands && m.mode == ModeSingle && m.hasLastRun && len(m.lastRunCommands) > 0
}

func (m Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.search = ""
		m.filtered = m.commands
		m.selected = 0
		m.syncMakefilePreviewToSelection()
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
	m.syncMakefilePreviewToSelection()
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
		if state.InteractiveSet {
			m.commands[i].Interactive = state.Interactive
			if state.Interactive {
				m.commands[i].Stream = false
			}
		} else {
			m.commands[i].Interactive = false
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
	case "interactive":
		return s.InteractiveSet && s.Interactive
	case "no-interactive":
		return s.InteractiveSet && !s.Interactive
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
	k := msg.String()
	switch k {
	case "left", "h":
		m.confirmModalSel = 0
		return m, nil
	case "right", "l":
		m.confirmModalSel = 1
		return m, nil
	case "tab":
		m.confirmModalSel = 1 - m.confirmModalSel
		return m, nil
	}
	// Enter dispatches per current selection; y/n are direct hotkeys.
	switch k {
	case "esc", "n":
		m.showConfirm = false
		m.pendingRerunExtras = nil
		m.pendingRerunChain = nil
	case "enter":
		if m.confirmModalSel == 0 {
			m.showConfirm = false
			m.pendingRerunExtras = nil
			m.pendingRerunChain = nil
			return m, nil
		}
		fallthrough
	case "y":
		m.showConfirm = false
		// Chain rerun: dispatch the saved chain end-to-end.
		if len(m.pendingRerunChain) > 0 {
			cmds := m.pendingRerunChain
			m.pendingRerunChain = nil
			return m.startChainFromCommands(cmds)
		}
		// Single-command rerun: pendingRerunExtras is non-nil exactly when
		// the modal was opened from rerunLast, so the picker must not
		// reopen. Plain triggerRun → confirm leaves it nil and falls
		// through to the normal dispatch.
		if m.pendingRerunExtras != nil && len(m.filtered) > 0 {
			extras := m.pendingRerunExtras
			m.pendingRerunExtras = nil
			cmdMeta := m.filtered[m.selected]
			return m.startSingleRun(cmdMeta, extras)
		}
		return m.dispatchRun()
	}
	return m, nil
}

// handleDeleteCmdModal routes input while the delete-command popup is open.
// Mirrors handleConfirmModal so the keybinds feel identical: ←/→ move,
// tab swaps focus, enter commits the focused button, y/n are direct hotkeys.
// Default focus is "cancel" — destructive defaults must require an explicit
// move before Enter deletes anything.
func (m Model) handleDeleteCmdModal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "left", "h":
		m.deleteCmdConfirmSel = 0
		return m, nil
	case "right", "l":
		m.deleteCmdConfirmSel = 1
		return m, nil
	case "tab":
		m.deleteCmdConfirmSel = 1 - m.deleteCmdConfirmSel
		return m, nil
	}
	switch k {
	case "esc", "n":
		m.showDeleteCmdConfirm = false
		m.deleteCmdName = ""
		return m, nil
	case "enter":
		if m.deleteCmdConfirmSel == 0 {
			m.showDeleteCmdConfirm = false
			m.deleteCmdName = ""
			return m, nil
		}
		fallthrough
	case "y":
		return m.commitDeleteCommand()
	}
	return m, nil
}

// commitDeleteCommand removes the staged target from the Makefile on disk,
// reloads the in-memory command list, and surfaces a status notice. Cursor
// is clamped to the new list length so the sidebar stays valid.
func (m Model) commitDeleteCommand() (tea.Model, tea.Cmd) {
	name := m.deleteCmdName
	m.showDeleteCmdConfirm = false
	m.deleteCmdName = ""
	if name == "" || m.makefilePath == "" {
		return m, nil
	}
	if err := source.RemoveMakefileTarget(m.makefilePath, name); err != nil {
		notice := m.setNotice("delete failed: "+err.Error(), views.NoticeError)
		return m, notice
	}
	src := &source.MakefileSource{Path: m.makefilePath}
	if cmds, err := src.Load(); err == nil {
		m.commands = cmds
		m.filtered = filterCommands(m.commands, m.search)
	}
	if m.selected >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.selected = len(m.filtered) - 1
		} else {
			m.selected = 0
		}
	}
	m.makefileLines = loadFileLines(m.makefilePath)
	m.syncMakefilePreviewToSelection()
	notice := m.setNotice("deleted '"+name+"' from Makefile", views.NoticeInfo)
	return m, notice
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
	m.makefileExpandLines = source.MakefileTargetLines(m.makefileLines, cmd.Name)
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
	case "q", m.keys.QuitAlt:
		return m.quitOrCancel()
	case "tab", m.keys.TabNext:
		m.activeTab = (m.activeTab + 1) % tabCount
	case m.keys.TabPrev:
		m.activeTab = (m.activeTab + tabCount - 1) % tabCount
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

// --- run dispatch ------------------------------------------------------------

func (m Model) triggerRun() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	cmd := m.filtered[m.selected]
	if m.running {
		m.enqueueCommand(cmd.Name)
		return m, nil
	}
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
	if len(cmdMeta.Picks) > 0 {
		return m.openPicker(cmdMeta)
	}
	return m.startSingleRun(cmdMeta, nil)
}

// dispatchPickedCommand is called by the picker once all pick steps are done.
// It bypasses the picker check in dispatchRun and forwards the resolved
// KEY=VAL pairs straight to the runner.
func (m Model) dispatchPickedCommand(cmdMeta source.Command, extraVars []string) (tea.Model, tea.Cmd) {
	return m.startSingleRun(cmdMeta, extraVars)
}

// startSingleRun begins fresh execution of cmdMeta. The chain state is reset
// to a single-element chain; subsequent enqueues (shortcuts while running)
// append to it. If the run ends without additions, no chain is persisted.
func (m Model) startSingleRun(cmdMeta source.Command, extraVars []string) (tea.Model, tea.Cmd) {
	m.chainCommands = []string{cmdMeta.Name}
	m.chainStepIdx = 0
	m.chainRunIDs = m.chainRunIDs[:0]
	m.chainStartAt = time.Now()
	// Cache the resolved picks so RerunLast can replay this exact invocation
	// without reopening the picker. nil extras (plain command) overwrite any
	// previously cached extras from a different command.
	m.lastRunExtraVars = append(m.lastRunExtraVars[:0], extraVars...)
	m.lastRunCommands = []string{cmdMeta.Name}
	m.hasLastRun = true
	m.persistLastRun([]string{cmdMeta.Name}, extraVars)
	return m.dispatchCommand(cmdMeta, extraVars)
}

// persistLastRun upserts the rerun target into the project_last_runs table.
// Best-effort: any DB error is swallowed so a transient failure can never
// block execution. The in-memory cache (m.lastRunCommands / extras) is
// already authoritative for the current session.
func (m Model) persistLastRun(commands, extraVars []string) {
	if m.db == nil || m.makefileDir == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = m.db.UpsertLastRun(ctx, m.makefileDir, db.LastRun{
		Commands:  append([]string(nil), commands...),
		ExtraVars: append([]string(nil), extraVars...),
	})
}

// dispatchCommand fires the given command as a single step. Caller has
// already configured chain state (chainCommands / chainStepIdx) as needed.
func (m Model) dispatchCommand(cmdMeta source.Command, extraVars []string) (tea.Model, tea.Cmd) {
	if cmdMeta.Interactive {
		return m.dispatchInteractive(cmdMeta.Name, extraVars)
	}
	name := cmdMeta.Name
	ctx, cancel := context.WithCancel(context.Background())
	ch := runner.StreamRun(ctx, m.makefileDir, name, extraVars)
	m.streamCh = ch
	m.runCancel = cancel
	stream := cmdMeta.Stream
	startCmd := func() tea.Msg { return RunStartMsg{Command: name, Stream: stream} }
	return m, tea.Batch(startCmd, waitNext(ch))
}

// enqueueCommand appends name to the active chain. Called when the user
// presses a shortcut (or Enter) while another command is still running.
func (m *Model) enqueueCommand(name string) {
	m.chainCommands = append(m.chainCommands, name)
}

// advanceChain dispatches the next step in chainCommands after a successful
// step. Returns (model, nil, false) if there are no more steps to run.
func (m Model) advanceChain() (tea.Model, tea.Cmd, bool) {
	next := m.chainStepIdx + 1
	if next >= len(m.chainCommands) {
		return m, nil, false
	}
	if m.chainCommands[next] == quitSentinel {
		// Drain reached the queued Quit. Stop here so the DoneMsg handler
		// runs persistChain on the real steps and then returns tea.Quit
		// because m.quitPending is set.
		return m, nil, false
	}
	m.chainStepIdx = next
	name := m.chainCommands[next]
	for _, c := range m.commands {
		if c.Name == name {
			// Chained steps don't reopen the picker — chains pre-dated this
			// feature and treating them otherwise would block mid-chain on a
			// modal. Picks are only honored on the user-initiated run.
			model, cmd := m.dispatchCommand(c, nil)
			return model, cmd, true
		}
	}
	// Unknown target (removed from Makefile mid-chain): treat as failure.
	return m, nil, false
}

// loadChainsCmd fetches both the deduplicated chain summaries (sidebar) and
// the per-execution chain history (history tab).
func (m Model) loadChainsCmd() tea.Cmd {
	if m.db == nil {
		return nil
	}
	database := m.db
	limit := m.chainHistoryMax
	if limit <= 0 {
		limit = 100
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		summaries, err := database.ListChainSummaries(ctx, limit)
		if err != nil {
			return HistoryErrorMsg{Err: fmt.Errorf("load chains: %w", err)}
		}
		runs, err := database.ListChainRuns(ctx, limit)
		if err != nil {
			return HistoryErrorMsg{Err: fmt.Errorf("load chain runs: %w", err)}
		}
		return ChainsLoadedMsg{Chains: summaries, Runs: runs}
	}
}

// persistChain upserts the sequence definition, opens+closes a sequence_run
// row, and links the per-step runs. Called once a chain (len >= 2) ends.
func (m Model) persistChain(finalStatus db.RunStatus) {
	if m.db == nil || len(m.chainRunIDs) < 2 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmds := m.chainCommands[:len(m.chainRunIDs)] // only actually-executed steps
	seqID, err := m.db.UpsertChainSequence(ctx, cmds)
	if err != nil {
		return
	}
	srID, err := m.db.StartSequenceRun(ctx, seqID, m.chainStartAt)
	if err != nil {
		return
	}
	_ = m.db.FinishSequenceRun(ctx, srID, finalStatus, time.Now(), time.Since(m.chainStartAt))
	for i, runID := range m.chainRunIDs {
		_, _ = m.db.SQL().ExecContext(ctx,
			`UPDATE runs SET sequence_run_id = ?, step_index = ? WHERE id = ?`,
			srID, i+1, runID)
	}
	keep := m.chainHistoryMax
	if keep <= 0 {
		keep = 100
	}
	_ = m.db.PruneChainRuns(ctx, keep)
}

// toggleChainSelection adds/removes name from the builder selection,
// preserving execution order.
func (m *Model) toggleChainSelection(name string) {
	for i, n := range m.chainChecked {
		if n == name {
			m.chainChecked = append(m.chainChecked[:i], m.chainChecked[i+1:]...)
			return
		}
	}
	m.chainChecked = append(m.chainChecked, name)
}

// startChainFromSelection dispatches the current builder selection as a
// chain and exits builder mode.
func (m Model) startChainFromSelection() (tea.Model, tea.Cmd) {
	cmds := append([]string(nil), m.chainChecked...)
	m.chainBuilder = false
	m.chainChecked = nil
	return m.startChainFromCommands(cmds)
}

// startChainFromCommands kicks off a pre-built chain (used by the builder
// flow and the chain-mode re-run flow).
func (m Model) startChainFromCommands(names []string) (tea.Model, tea.Cmd) {
	if len(names) == 0 {
		return m, nil
	}
	first := names[0]
	var head *source.Command
	for i, c := range m.commands {
		if c.Name == first {
			cc := m.commands[i]
			head = &cc
			break
		}
	}
	if head == nil {
		return m, nil
	}
	m.chainCommands = names
	m.chainStepIdx = 0
	m.chainRunIDs = m.chainRunIDs[:0]
	m.chainStartAt = time.Now()
	// Persist the full chain as the rerun target so Ctrl+R / sidebar card
	// replays every step in order. Chains skip the picker, so no ExtraVars.
	m.lastRunCommands = append([]string(nil), names...)
	m.lastRunExtraVars = m.lastRunExtraVars[:0]
	m.hasLastRun = true
	m.persistLastRun(names, nil)
	return m.dispatchCommand(*head, nil)
}

// dispatchInteractive runs the target with the real TTY attached, suspending
// the Bubble Tea program while the process is alive. No streaming channel is
// used; the DoneMsg comes from tea.ExecProcess' callback.
func (m Model) dispatchInteractive(name string, extraVars []string) (tea.Model, tea.Cmd) {
	m.streamCh = nil
	m.runCancel = nil
	startCmd := func() tea.Msg { return RunStartMsg{Command: name, Interactive: true} }
	return m, tea.Sequence(startCmd, runner.InteractiveRun(m.makefileDir, name, extraVars))
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

// rerunLast replays the most recent dispatch, reusing cached pick selections
// so the folder picker does not reopen. Falls back to history[0] when no
// in-memory last-run exists yet (e.g. after the TUI was just launched).
// Confirmation precedence is preserved: NoConfirm skips the modal, Confirm
// or non-local env still asks. The modal's yes-path uses pendingRerunExtras
// to dispatch with the cached vars instead of going through triggerRun.
func (m Model) rerunLast() (tea.Model, tea.Cmd) {
	if m.running {
		return m, nil
	}
	cmds := append([]string(nil), m.lastRunCommands...)
	// Defensive copy: startSingleRun/startChainFromCommands rewrite the
	// cached slice in place; passing it directly would alias source and dest.
	extras := append([]string(nil), m.lastRunExtraVars...)
	if len(cmds) == 0 {
		if len(m.history) == 0 {
			return m, nil
		}
		cmds = []string{m.history[0].Command}
		extras = nil
	}

	// Chain rerun: dispatch the whole sequence. Chains never carry picker
	// extras (chained steps skip the picker by design), so confirmation
	// follows the head command's flags and any non-local env still asks.
	if len(cmds) > 1 {
		head := m.findCommand(cmds[0])
		if head == nil {
			return m, nil
		}
		switch {
		case head.NoConfirm:
		case head.Confirm, m.env != config.EnvLocal:
			m.pendingRerunChain = cmds
			m.showConfirm = true
			return m, nil
		}
		return m.startChainFromCommands(cmds)
	}

	name := cmds[0]
	cmd := m.findCommand(name)
	if cmd == nil {
		return m, nil
	}
	// Mirror selection so the confirm modal labels the right command.
	for i, c := range m.filtered {
		if c.Name == name {
			m.selected = i
			break
		}
	}
	// If the command needs picks and we have nothing cached, fall back to
	// the regular flow (which opens the picker). This only happens on the
	// very first rerun after launch when history pre-exists but cache is empty.
	if len(cmd.Picks) > 0 && len(extras) == 0 {
		return m.triggerRun()
	}

	switch {
	case cmd.NoConfirm:
	case cmd.Confirm, m.env != config.EnvLocal:
		m.pendingRerunExtras = append([]string(nil), extras...)
		m.showConfirm = true
		return m, nil
	}
	return m.startSingleRun(*cmd, extras)
}

// findCommand returns a copy of the command matching name from m.commands,
// or nil when not found (e.g. removed from the Makefile after the rerun was
// recorded).
func (m Model) findCommand(name string) *source.Command {
	for i := range m.commands {
		if m.commands[i].Name == name {
			c := m.commands[i]
			return &c
		}
	}
	return nil
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
		if commandMatches(c, query) {
			out = append(out, c)
		}
	}
	return out
}

// commandMatches reports whether the query substring matches the command's
// name, description, or any of its [tags=…] values (case-insensitive).
func commandMatches(c source.Command, query string) bool {
	if containsFold(c.Name, query) || containsFold(c.Desc, query) {
		return true
	}
	for _, t := range c.Tags {
		if containsFold(t, query) {
			return true
		}
	}
	return false
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
