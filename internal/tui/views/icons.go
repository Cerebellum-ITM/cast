package views

// IconStyle selects which glyph table is returned by Icons().
type IconStyle int

const (
	// IconNerdFont uses Nerd Font private-use codepoints. This is the
	// project default — every new icon should be added to the NerdFont
	// branch first; the emoji branch is a fallback for terminals without
	// a Nerd Font–patched terminal font.
	IconNerdFont IconStyle = iota
	// IconEmoji uses generic Unicode emoji that render in any terminal.
	IconEmoji
)

// ParseIconStyle maps a string ("nerdfont" | "emoji") to an IconStyle.
// Unknown / empty values fall back to nerdfont.
func ParseIconStyle(s string) IconStyle {
	if s == "emoji" {
		return IconEmoji
	}
	return IconNerdFont
}

// IconSet is the central registry of glyphs used by views. Add new icons here
// (always with a Nerd Font codepoint as the canonical entry; the emoji is a
// best-effort fallback) and reference them from views via Icons(style).
type IconSet struct {
	// Folder glyphs used by the picker.
	FolderGeneric string
	FolderGit     string
	FolderMake    string
	FolderOdoo    string
	FolderNode    string
	FolderPython  string
	FolderGo      string
	FolderRust    string

	// Status / state glyphs.
	Warning string
	Folder  string // alias for FolderGeneric — used outside the picker

	// Picker chrome.
	PickerTitle string // glyph next to the picker title

	// Library / snippets.
	Snippet string // each row in the snippets library
}

// Icons returns the icon set for the requested style.
func Icons(style IconStyle) IconSet {
	if style == IconEmoji {
		return IconSet{
			FolderGeneric: "📁",
			FolderGit:     "🌿",
			FolderMake:    "🛠 ",
			FolderOdoo:    "🟣",
			FolderNode:    "📦",
			FolderPython:  "🐍",
			FolderGo:      "🐹",
			FolderRust:    "🦀",
			Warning:       "⚠️",
			Folder:        "📁",
			PickerTitle:   "📁",
			Snippet:       "📋",
		}
	}
	// Nerd Font (default). Codepoints come from the Nerd Fonts cheat sheet
	// (https://www.nerdfonts.com/cheat-sheet). Use the explicit \uXXXX form
	// so private-use glyphs survive editor round-trips.
	return IconSet{
		FolderGeneric: "",  // nf-fa-folder
		FolderGit:     "",  // nf-fa-git
		FolderMake:    "",  // nf-fa-wrench (Makefile / build tool)
		FolderOdoo:    "",  // nf-fa-puzzle_piece (module / addon)
		FolderNode:    "",  // nf-dev-nodejs_small
		FolderPython:  "",  // nf-dev-python
		FolderGo:      "",  // nf-dev-go (gopher)
		FolderRust:    "",  // nf-dev-rust
		Warning:       "",  // nf-fa-exclamation_triangle
		Folder:        "",  // nf-fa-folder
		PickerTitle:   "",  // nf-fa-folder_open
		Snippet:       "",  // nf-fa-code (snippet / library)
	}
}
