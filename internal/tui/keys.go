package tui

// KeyMap defines keyboard shortcuts for the main TUI.
// Keys are string representations as returned by tea.KeyPressMsg.String().
type KeyMap struct {
	Quit          string
	TabNext       string
	TabPrev       string
	Up            string
	Down          string
	UpVim         string
	DownVim       string
	Top           string
	Bottom        string
	Search        string
	Run           string
	RunAlt        string
	RerunLast     string
	ToggleSecrets  string
	ExpandOutput   string
	ExpandMakefile string
	EnvRestore     string // restore selected history entry (env tab only)
	OutputWider    string // grow the output panel
	OutputNarrower string // shrink the output panel
}

// DefaultKeyMap is the out-of-the-box key configuration.
var DefaultKeyMap = KeyMap{
	Quit:           "q",
	TabNext:        "tab",
	TabPrev:        "shift+tab",
	Up:             "up",
	Down:           "down",
	UpVim:          "k",
	DownVim:        "j",
	Top:            "g",
	Bottom:         "G",
	Search:         "/",
	Run:            "enter",
	RunAlt:         "r",
	RerunLast:      "ctrl+r",
	ToggleSecrets:  "s",
	ExpandOutput:   "ctrl+e",
	ExpandMakefile: "ctrl+o",
	EnvRestore:     "r",
	OutputWider:    "]",
	OutputNarrower: "[",
}
