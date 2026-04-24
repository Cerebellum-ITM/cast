# lipgloss pitfalls

Known gotchas hit while working on the cast TUI. Read this when a view renders
at the wrong width, has visible misalignment, or loses colored characters at a
panel boundary.

## Multi-line indentation: use `MarginLeft` not string prefix

Never indent a multi-line lipgloss render (boxes, borders, tall components) by
prepending spaces with `Pad(n) + widget`. String concatenation only prepends to
the **first line** — the remaining lines (e.g. the body and bottom of a
rounded border) render flush-left, causing visual misalignment.

**Wrong:**
```go
box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(w).Render(val)
lines = append(lines, Pad(2)+box)  // only the top border gets the indent
```

**Correct:**
```go
box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).MarginLeft(2).Width(w).Render(val)
lines = append(lines, box)  // MarginLeft applies to every line
```

This applies to any multi-line component: bordered boxes, tall panels, stacked
renders. Single-line styled strings (`Pad(2) + Style(...).Render(text)`) are
fine.

## Trailing styled space dropped by `Width()`

When a content string ends with a character that is **entirely styled
background** (typically a chip/badge rendered with `Padding(0,1)` whose right
padding is a single bg-colored space) and is then wrapped in
`lipgloss.NewStyle().Width(N).Render(...)` with content whose visual width
already equals `N`, lipgloss collapses that trailing styled space. The chip
visually loses its right padding and looks cut.

**Workaround**: append an unstyled `" "` after the chip and reserve one extra
column in the width math. See `renderCommandCard` in
`internal/tui/views/sidebar.go` (the `trailBuf` variable).

Diagnostic signature in the rendered ANSI: the chip's trailing padding
sequence `\x1b[…bg m \x1b[m` collapses to `\x1b[…bg m\x1b[m` (same bg open +
reset, but with no space between them). `lipgloss.Width` still reports the
intended width — the bug is purely in the emitted string.
