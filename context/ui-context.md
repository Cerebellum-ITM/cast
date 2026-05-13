# UI Context

## Theme

cast is a terminal UI rendered with `lipgloss`. The aesthetic is a
**technical-minimal dark workspace** with vivid env-aware accents.
Three themes ship:

- **catppuccin** (default) — muted pastel palette.
- **dracula** — high-contrast purple-leaning palette.
- **nord** — cool-toned arctic palette.

Three environments tint the accent:

- **local** — theme's default accent (cyan-ish per theme).
- **staging** — orange accent.
- **prod** — red accent.

The resolved `views.Palette` is produced by
`paletteFor(theme, env)` in `internal/tui/styles.go` and threaded
through every render function as a Props field.

## Colors

Colors are not CSS variables — they are `lipgloss.Color` values held
on the `views.Palette` struct (`internal/tui/views/common.go`). The
palette is the **single source of truth**: a view that needs a color
reads it from `palette.<Field>`. Hardcoding a hex string anywhere
outside `internal/tui/styles.go` is a bug.

`basePalette(theme)` returns the per-theme base colors; the env
overlay (`local`/`staging`/`prod`) replaces the accent fields. Adding
a new color role means:

1. Add a field to `views.Palette`.
2. Set it for every theme in `basePalette`.
3. Override per env in `paletteFor` if env-sensitive.

## Typography

Terminal-rendered. Fonts are controlled by the user's terminal
emulator, not by cast. Cells are monospace; cast assumes Nerd Font
glyphs render at single-cell width (with the documented `emoji`
fallback for non-patched terminals).

## Icons

- **Default style:** `nerdfont`. Glyphs from
  <https://www.nerdfonts.com/cheat-sheet>.
- **Fallback:** `emoji` — enabled via `[ui] icons = "emoji"` in
  `~/.config/cast/cast.toml`.
- **Source of truth:** `IconSet` in `internal/tui/views/icons.go`.
  Every icon used anywhere in cast has a field in `IconSet` with
  both a Nerd Font value and an emoji fallback.
- **Access pattern:** `Icons(style).<Field>` inside `views/`; outside
  `views/`, callers must accept `views.IconStyle` as a parameter and
  resolve through `views.Icons(style)`. **Never inline a literal
  glyph in a view, model, or any other package.**

## Layout Patterns

cast's TUI is a three-pane layout assembled from view functions:

- **Header** (`render.go` → `renderHeader`) — title, env pill (single
  vs chain mode), tab bar across `commands`, `history`, `env`,
  `theme`, `library`.
- **Body** — three columns:
  - **Sidebar** (`views.Sidebar`) — search row, command list cards,
    keyboard hints. In chain builder, cards gain an accent bar and
    order number.
  - **Center** (`views.Commands` / `views.History` / `views.EnvPane` /
    etc.) — preview of the highlighted item, follows sidebar
    selection.
  - **Output** (`views.Output`) — live terminal output with real-time
    progress bar and a RECENT run list.
- **Status bar** (`views.StatusBar`) — one-row bottom bar with command
  count and active env.
- **Modal** (`views.Modal`) — centered overlay for production-confirm
  and similar prompts; uses the env accent for its border.
- **Picker** (`internal/tui/picker.go`) — centered fzf-like folder
  picker triggered by `[pick=…]`; rows decorated with content-aware
  glyphs (Odoo modules, Git repos, Makefiles, `package.json`, …).

## Mode pill

The header carries a mode pill toggled with `ctrl+s`:

- **SINGLE** — cyan (theme default).
- **CHAIN** — orange.

Chains-in-progress render at the top of the sidebar in a `CHAIN (N)`
block, with the running step marked `▶`.

## Aesthetic guardrails

- Borders use `lipgloss`'s `Rounded` style by default; never invent a
  new border style per view.
- Padding/indent inside a bordered box: see
  [`docs/ai/lipgloss-pitfalls.md`](../docs/ai/lipgloss-pitfalls.md)
  before "fixing" a misalignment.
- All glyphs decorating list rows (folder icons, command status icons,
  spinner phases) go through `Icons(style)` — never hard-coded.
