package tui

// KeyMap defines keyboard shortcuts for the main TUI.
// Keys are string representations as returned by tea.KeyPressMsg.String().
//
// Note: single-letter keys that appear here (q, g, G, s, r) are always checked
// *after* per-command shortcuts in handleKey — so if a Makefile command has
// e.g. Shortcut="g", it takes precedence over Top here. Inside the env tab and
// inside popups, j/k stay available for local scrolling (handled there).
type KeyMap struct {
	Quit            string
	TabNext         string
	TabPrev         string
	Up              string
	Down            string
	Top             string
	Bottom          string
	Search          string
	Run             string
	RunAlt          string
	RerunLast       string
	ToggleSecrets   string
	ExpandOutput    string
	ExpandMakefile  string
	EnvRestore      string // restore selected history entry (env tab only)
	OutputWider     string // grow the output panel
	OutputNarrower  string // shrink the output panel
	SidebarWider    string // grow the left sidebar
	SidebarNarrower string // shrink the left sidebar
}

// DefaultKeyMap is the out-of-the-box key configuration.
var DefaultKeyMap = KeyMap{
	Quit:            "q",
	TabNext:         "tab",
	TabPrev:         "shift+tab",
	Up:              "up",
	Down:            "down",
	Top:             "g",
	Bottom:          "G",
	Search:          "/",
	Run:             "enter",
	RunAlt:          "ctrl+r",
	RerunLast:       "ctrl+r",
	ToggleSecrets:   "s",
	ExpandOutput:    "ctrl+e",
	ExpandMakefile:  "ctrl+o",
	EnvRestore:      "r",
	OutputWider:     "]",
	OutputNarrower:  "[",
	SidebarWider:    "}",
	SidebarNarrower: "{",
}
