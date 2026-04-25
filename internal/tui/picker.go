package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui/views"
)

// openPicker initializes folder-picker state for cmd and lists the first
// step's entries. Returns the model with showPicker=true so the overlay
// renders on the next frame.
func (m Model) openPicker(cmd source.Command) (tea.Model, tea.Cmd) {
	m.pickerCmd = cmd
	m.pickerStep = 0
	m.pickerSelections = m.pickerSelections[:0]
	m.pickerExtraVars = m.pickerExtraVars[:0]
	m.pickerCursor = 0
	m.pickerSearch = ""
	m.showPicker = true
	m.refreshPickerEntries()
	return m, nil
}

// refreshPickerEntries resolves the current step's base directory, lists it,
// applies the step filter, and stores both the unfiltered and search-filtered
// lists on the model.
func (m *Model) refreshPickerEntries() {
	step := m.pickerCmd.Picks[m.pickerStep]
	base := source.ResolvePickBase(step.BaseDirTemplate, m.pickerSelections)
	m.pickerBase = base

	cwd, _ := os.Getwd()
	abs := base
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, base)
	}
	entries, _ := os.ReadDir(abs)
	var filtered []views.PickerEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != "." && name != ".." {
			// Hide dotfiles but allow them through if no filter was specified
			// and the user explicitly named them in the spec — for now, skip.
			continue
		}
		if !source.MatchPickFilter(name, step.Filter) {
			continue
		}
		filtered = append(filtered, views.PickerEntry{
			Name: name,
			Icon: pickerIcon(filepath.Join(abs, name)),
		})
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	m.pickerEntriesAll = filtered
	m.applyPickerSearch()
}

// applyPickerSearch filters pickerEntriesAll by the current search buffer
// (case-insensitive substring) and resets the cursor.
func (m *Model) applyPickerSearch() {
	if m.pickerSearch == "" {
		m.pickerEntries = m.pickerEntriesAll
	} else {
		q := strings.ToLower(m.pickerSearch)
		var out []views.PickerEntry
		for _, e := range m.pickerEntriesAll {
			if strings.Contains(strings.ToLower(e.Name), q) {
				out = append(out, e)
			}
		}
		m.pickerEntries = out
	}
	if m.pickerCursor >= len(m.pickerEntries) {
		m.pickerCursor = 0
	}
}

// pickerIcon picks a nerd-font glyph that hints at the directory's contents.
// Best-effort — silently falls back to a plain folder if the dir can't be read.
func pickerIcon(path string) string {
	const folder = "📁"
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return folder
	}
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(path, name))
		return err == nil
	}
	switch {
	case has(".git"):
		return ""
	case has("Makefile"):
		return ""
	case has("__manifest__.py"), has("__openerp__.py"):
		return "" // Odoo module
	case has("package.json"):
		return ""
	case has("__init__.py"), has("pyproject.toml"), has("setup.py"):
		return ""
	case has("go.mod"):
		return ""
	case has("Cargo.toml"):
		return ""
	}
	return folder
}

// handlePickerKey routes keystrokes while the folder picker is open.
func (m Model) handlePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "esc", "ctrl+c":
		m.cancelPicker()
		return m, nil
	case "up", "ctrl+k":
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.pickerCursor < len(m.pickerEntries)-1 {
			m.pickerCursor++
		}
		return m, nil
	case "left":
		// Step back to the previous pick if we have one.
		if m.pickerStep > 0 {
			m.pickerStep--
			if len(m.pickerSelections) > m.pickerStep {
				m.pickerSelections = m.pickerSelections[:m.pickerStep]
			}
			if len(m.pickerExtraVars) > m.pickerStep {
				m.pickerExtraVars = m.pickerExtraVars[:m.pickerStep]
			}
			m.pickerSearch = ""
			m.refreshPickerEntries()
		}
		return m, nil
	case "backspace":
		if m.pickerSearch != "" {
			runes := []rune(m.pickerSearch)
			m.pickerSearch = string(runes[:len(runes)-1])
			m.applyPickerSearch()
			return m, nil
		}
		// Empty buffer + backspace: step back like Left.
		if m.pickerStep > 0 {
			m.pickerStep--
			if len(m.pickerSelections) > m.pickerStep {
				m.pickerSelections = m.pickerSelections[:m.pickerStep]
			}
			if len(m.pickerExtraVars) > m.pickerStep {
				m.pickerExtraVars = m.pickerExtraVars[:m.pickerStep]
			}
			m.pickerSearch = ""
			m.refreshPickerEntries()
		}
		return m, nil
	case "enter":
		return m.commitPickerStep()
	}
	if len(k) == 1 {
		m.pickerSearch += k
		m.applyPickerSearch()
	}
	return m, nil
}

// commitPickerStep records the current selection, advances to the next step,
// or — when all steps are done — closes the picker and dispatches the run.
func (m Model) commitPickerStep() (tea.Model, tea.Cmd) {
	if len(m.pickerEntries) == 0 || m.pickerCursor >= len(m.pickerEntries) {
		return m, nil
	}
	chosen := m.pickerEntries[m.pickerCursor].Name
	m.pickerSelections = append(m.pickerSelections, chosen)
	step := m.pickerCmd.Picks[m.pickerStep]
	varName := source.PickVarName(step, m.pickerStep+1)
	// Resolve the value to the path relative to cwd so recipes can `cd $(VAR)`.
	resolved := source.ResolvePickBase(step.BaseDirTemplate, m.pickerSelections[:len(m.pickerSelections)-1])
	full := filepath.Join(resolved, chosen)
	m.pickerExtraVars = append(m.pickerExtraVars, varName+"="+full)

	m.pickerStep++
	if m.pickerStep >= len(m.pickerCmd.Picks) {
		m.showPicker = false
		extra := append([]string(nil), m.pickerExtraVars...)
		// Reset picker state, then dispatch the command with extras.
		m.pickerSelections = m.pickerSelections[:0]
		m.pickerExtraVars = m.pickerExtraVars[:0]
		m.pickerStep = 0
		return m.dispatchPickedCommand(m.pickerCmd, extra)
	}
	m.pickerSearch = ""
	m.pickerCursor = 0
	m.refreshPickerEntries()
	return m, nil
}

// cancelPicker tears down picker state without dispatching anything.
func (m *Model) cancelPicker() {
	m.showPicker = false
	m.pickerSelections = m.pickerSelections[:0]
	m.pickerExtraVars = m.pickerExtraVars[:0]
	m.pickerStep = 0
	m.pickerSearch = ""
	m.pickerEntries = nil
	m.pickerEntriesAll = nil
}
