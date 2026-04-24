# Debugging view rendering

Technique for reproducing and fixing layout/ANSI bugs in `internal/tui/views`
without running the full TUI. Use this when a view renders at the wrong width,
has ANSI bleeding, drops characters at a panel boundary, or looks cut when
content exactly fills a container.

## The throwaway `cmd/debugview` trick

Spin up a temporary `cmd/debugview/main.go` that imports the view package
directly, renders the component with synthetic props at multiple widths, and
prints the rendered output with widths annotated. Delete it when done.

```go
// cmd/debugview/main.go
package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/source"
	"github.com/Cerebellum-ITM/cast/internal/tui/views"
)

func stripANSI(s string) string {
	out, inEsc := "", false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out += string(r)
	}
	return out
}

func main() {
	p := views.Palette{ /* minimum fields the view reads — use real hex colors */ }
	cmds := []source.Command{ /* representative sample: short name, long name, with/without tag */ }

	for _, w := range []int{22, 28, 34} {
		sb := views.Sidebar(p, views.SidebarProps{Commands: cmds, Width: w, Height: 14})
		for i, row := range strings.Split(sb, "\n") {
			plain := stripANSI(row)
			fmt.Printf("[%d] |%s| w=%d (expect %d)\n", i, plain, lipgloss.Width(plain), w)
		}
	}
}
```

```bash
mkdir -p cmd/debugview && $EDITOR cmd/debugview/main.go
go run ./cmd/debugview
rm -rf cmd/debugview
```

## Why it works

- A `cmd/` subpackage is the only way to import `internal/*` from outside its
  own subtree — a scratch file in `/tmp` fails with "use of internal package
  not allowed".
- View functions in `internal/tui/views/` are pure (Props + Palette → string),
  so you can call them directly without bringing up a Bubble Tea program.

## What to check

- Strip ANSI and compare widths: `lipgloss.Width(raw) vs lipgloss.Width(plain)`.
  If they disagree, the terminal/lipgloss is swallowing a styled character and
  you have a rendering bug, not a math bug.
- Test multiple `Width:` values — narrow edge + normal + wide. Most layout
  bugs only manifest when the content exactly fills the container (off-by-one
  at the boundary).
- Print `%q` on the raw string when inspecting ANSI. The fastest way to spot
  missing padding is to see `\x1b[…bg m \x1b[m` collapse to `\x1b[…bg m\x1b[m`
  (trailing styled space dropped — see `lipgloss-pitfalls.md`).
- Use a representative input sample: short name, long-enough-to-truncate name,
  with tag, without tag, multi-char shortcut, etc. Bugs often hide in the
  truncation path, not the happy path.

## Minimal reductions

If the bug is in a chip/badge helper rather than a composite view, you can skip
the view entirely and directly render just the helper:

```go
chip := views.RenderTagChip(p, "prod")
fmt.Printf("chip: %q  visW=%d\n", chip, lipgloss.Width(chip))

wrapped := lipgloss.NewStyle().Width(lipgloss.Width(chip)).Render(chip)
fmt.Printf("wrapped: %q\n", wrapped)
```

This is how the "trailing styled space" bug was isolated — the issue survived
without any sidebar code, making it clear the root cause was lipgloss itself.
