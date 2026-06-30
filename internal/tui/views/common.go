package views

import (
	"image/color"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Cerebellum-ITM/cast/internal/db"
)

// Palette holds all resolved color tokens for a theme + environment combination.
type Palette struct {
	Bg, BgPanel, BgDeep, BgSelected, BgHover color.Color
	Fg, FgDim, FgMuted                       color.Color
	Border, Accent                           color.Color
	Cyan, Green, Yellow, Orange, Red         color.Color
	StreamAccent                             color.Color
}

// Style creates a simple foreground lipgloss style.
func Style(c color.Color, bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(c)
	if bold {
		s = s.Bold(true)
	}
	return s
}

// SepLine returns a full-width separator using ─ characters.
func SepLine(p Palette, w int) string {
	return Style(p.Border, false).Render(strings.Repeat("─", w))
}

// Pad returns n spaces for manual indentation.
func Pad(n int) string { return strings.Repeat(" ", n) }

// VisWidth returns the visible column width of a (potentially styled) string.
func VisWidth(s string) int { return lipgloss.Width(s) }

// Truncate clips s to max visual characters, appending … if needed.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

// NoShortcutIcon is rendered in place of the shortcut letter for commands that
// have no assigned shortcut. `⬢` evokes cast's visual identity and reads as
// "a thing you can run, just not with one keypress".
const NoShortcutIcon = "⬢"

// RenderKeyBadge renders a keyboard shortcut badge. Commands without an
// assigned shortcut get a muted icon instead of a blank/letter badge so the
// distinction is obvious at a glance.
func RenderKeyBadge(p Palette, key string) string {
	if key == "" {
		return lipgloss.NewStyle().
			Foreground(p.FgDim).
			Background(p.BgHover).
			Padding(0, 1).
			Render(NoShortcutIcon)
	}
	return lipgloss.NewStyle().
		Foreground(p.BgDeep).Bold(true).
		Background(p.Accent).
		Padding(0, 1).
		Render(key)
}

// RenderTagChip renders a tag with its semantic color on a dark background.
func RenderTagChip(p Palette, text string) string {
	return lipgloss.NewStyle().
		Foreground(TagColor(p, text)).
		Background(p.BgSelected).
		Padding(0, 1).
		Render(text)
}

// RenderInlineTag renders a tag inline (no extra padding).
func RenderInlineTag(p Palette, text string) string {
	return lipgloss.NewStyle().
		Foreground(TagColor(p, text)).
		Background(p.BgSelected).
		Padding(0, 1).
		Render(text)
}

// TagColor maps a tag string to a palette color.
func TagColor(p Palette, tag string) color.Color {
	switch strings.ToLower(tag) {
	case "go":
		return p.Cyan
	case "ci", "golangci":
		return p.Orange
	case "prod", "production":
		return p.Red
	case "staging":
		return p.Orange
	case "dev", "local":
		return p.Green
	default:
		return p.Accent
	}
}

// RenderProgressBar draws a 1-row block-fill progress bar using fillColor for filled blocks.
func RenderProgressBar(p Palette, w int, progress float64, fillColor color.Color) string {
	if w < 2 {
		return ""
	}
	filled := int(float64(w) * progress)
	if filled > w {
		filled = w
	}
	return lipgloss.NewStyle().Foreground(fillColor).Render(strings.Repeat("▓", filled)) +
		lipgloss.NewStyle().Foreground(p.FgMuted).Render(strings.Repeat("░", w-filled))
}

// StatusDot returns a colored ● indicator for a run status.
func StatusDot(p Palette, status db.RunStatus) string {
	switch status {
	case db.StatusSuccess:
		return Style(p.Green, false).Render("●")
	case db.StatusError:
		return Style(p.Red, false).Render("●")
	case db.StatusRunning:
		return Style(p.Yellow, false).Render("●")
	case db.StatusInterrupted:
		return Style(p.Orange, false).Render("⏹")
	default:
		return Style(p.FgDim, false).Render("●")
	}
}

// ColorOutputLine applies syntax color to a terminal output line that carries
// neither native ANSI escapes nor a recognized log level (those are handled
// upstream in colorizeLogLine). A few whole-line markers keep their dedicated
// color; everything else is tokenized by richColorLine so numbers, strings,
// paths, URLs, key=value pairs, IPs and timestamps stand out instead of the
// whole line reading as one flat dim block.
func ColorOutputLine(p Palette, line string) string {
	switch {
	case strings.HasPrefix(line, "✓"):
		return Style(p.Green, false).Render(line)
	case strings.HasPrefix(line, "✗"):
		return Style(p.Red, false).Render(line)
	case strings.HasPrefix(line, "$"):
		return Style(p.Cyan, false).Render(line)
	case strings.HasPrefix(line, "--- PASS"):
		return Style(p.Green, false).Render(line)
	case strings.HasPrefix(line, "--- FAIL"):
		return Style(p.Red, false).Render(line)
	default:
		return richColorLine(p, line)
	}
}

// richTokenRe matches, in priority order, the token kinds richColorLine knows
// how to color. The alternation order matters: more specific tokens (url,
// timestamp, ip, key=value) come before the bare number so the most meaningful
// match wins. The input is guaranteed ANSI-free (colorizeLogLine returns early
// on escape sequences), so a single pass cannot re-match injected escapes.
var richTokenRe = regexp.MustCompile(
	`(?i)(?P<url>https?://[^\s]+)` +
		`|(?P<ts>\b\d{4}-\d{2}-\d{2}[t ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:z|[+-]\d{2}:?\d{2})?)` +
		`|(?P<clock>\b\d{2}:\d{2}:\d{2}(?:\.\d+)?\b)` +
		`|(?P<ip>\b\d{1,3}(?:\.\d{1,3}){3}(?::\d+)?\b)` +
		`|(?P<kv>\b[\w.-]+=(?:"[^"]*"|'[^']*'|[^\s,;]+))` +
		`|(?P<str>"[^"]*"|'[^']*')` +
		`|(?P<path>\.{0,2}/[\w./@~+-]+)` +
		`|(?P<bool>\b(?:true|false|null|nil)\b)` +
		`|(?P<num>\b\d+(?:\.\d+)?\b)`,
)

var (
	scalarBoolRe = regexp.MustCompile(`(?i)^(?:true|false|null|nil|none|yes|no)$`)
	scalarNumRe  = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
)

// richColorLine tokenizes a plain (ANSI-free, level-free) output line and
// colors each recognized token, leaving the gaps in the base Fg color. It
// walks the matches once via FindAllStringSubmatchIndex so styles are injected
// without ever re-scanning the escapes they introduce.
func richColorLine(p Palette, line string) string {
	matches := richTokenRe.FindAllStringSubmatchIndex(line, -1)
	if matches == nil {
		return Style(p.Fg, false).Render(line)
	}
	names := richTokenRe.SubexpNames()
	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if start > last {
			b.WriteString(Style(p.Fg, false).Render(line[last:start]))
		}
		group := ""
		for gi := 1; gi*2 < len(m); gi++ {
			if m[2*gi] >= 0 {
				group = names[gi]
				break
			}
		}
		b.WriteString(renderToken(p, group, line[start:end]))
		last = end
	}
	if last < len(line) {
		b.WriteString(Style(p.Fg, false).Render(line[last:]))
	}
	return b.String()
}

// renderToken colors a single matched token according to its kind. key=value
// is split so the key, the separator and the (type-classified) value each get
// their own color.
func renderToken(p Palette, group, tok string) string {
	switch group {
	case "url", "path":
		return Style(p.Cyan, false).Render(tok)
	case "ts", "clock":
		return Style(p.FgMuted, false).Render(tok)
	case "ip":
		return Style(p.Orange, false).Render(tok)
	case "str":
		return Style(p.Green, false).Render(tok)
	case "bool":
		return Style(p.Orange, false).Render(tok)
	case "num":
		return Style(p.Yellow, false).Render(tok)
	case "kv":
		eq := strings.IndexByte(tok, '=')
		if eq < 0 {
			return Style(p.Fg, false).Render(tok)
		}
		key, val := tok[:eq], tok[eq+1:]
		return Style(p.Cyan, false).Render(key) +
			Style(p.FgDim, false).Render("=") +
			Style(classifyScalar(p, val), false).Render(val)
	default:
		return Style(p.Fg, false).Render(tok)
	}
}

// classifyScalar picks a color for a key=value right-hand side by its shape:
// quoted string, boolean/null keyword, number, or plain text.
func classifyScalar(p Palette, s string) color.Color {
	switch {
	case len(s) >= 2 && (s[0] == '"' || s[0] == '\''):
		return p.Green
	case scalarBoolRe.MatchString(s):
		return p.Orange
	case scalarNumRe.MatchString(s):
		return p.Yellow
	default:
		return p.Fg
	}
}

// HighlightEnvLine applies syntax color to a single .env file line.
func HighlightEnvLine(p Palette, line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		return Style(p.FgDim, false).Render(line)
	}
	idx := strings.Index(trimmed, "=")
	if idx < 1 {
		return Style(p.FgDim, false).Render(line)
	}
	key := trimmed[:idx]
	val := trimmed[idx+1:]
	isNumeric := val != "" && func() bool {
		for _, r := range val {
			if (r < '0' || r > '9') && r != '.' && r != '-' {
				return false
			}
		}
		return true
	}()
	var valStr string
	if isNumeric {
		valStr = Style(p.Yellow, false).Render(val)
	} else {
		valStr = Style(p.Green, false).Render(val)
	}
	return Style(p.Cyan, false).Render(key) +
		Style(p.FgDim, false).Render("=") +
		valStr
}

// HighlightMakefileLine applies syntax highlighting to a single Makefile line.
func HighlightMakefileLine(p Palette, line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "##"), strings.HasPrefix(trimmed, "#"),
		strings.HasPrefix(trimmed, ".PHONY"):
		return Style(p.FgDim, false).Render(line)
	case strings.Contains(line, " = ") || strings.Contains(line, " := "):
		idx := strings.Index(line, "=")
		if idx > 0 {
			return Style(p.Cyan, false).Render(line[:idx]) +
				Style(p.FgDim, false).Render("=") +
				Style(p.Yellow, false).Render(line[idx+1:])
		}
	case !strings.HasPrefix(line, "\t") && strings.HasSuffix(trimmed, ":"):
		return Style(p.Accent, false).Render(line)
	case strings.HasPrefix(line, "\t@echo"):
		return Style(p.FgDim, false).Render("\t@") +
			Style(p.Green, false).Render(strings.TrimPrefix(line, "\t@"))
	case strings.HasPrefix(line, "\t@"):
		return Style(p.FgDim, false).Render("\t@") +
			Style(p.Fg, false).Render(strings.TrimPrefix(line, "\t@"))
	}
	return Style(p.FgDim, false).Render(line)
}
