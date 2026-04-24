package views

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	log "charm.land/log/v2"
)

// logLevelRe finds a log-level word in the first ~40 chars of a line, allowing
// a leading timestamp or prefix (e.g. "12:34:56 INFO foo" or "2026-01-01T12:00:00Z WARN bar").
// The match is anchored at a word boundary so "uninformed" won't match INFO.
// Capture group 1 is the level word itself.
var logLevelRe = regexp.MustCompile(
	`(?i)(?:^|[\s\[\|>])(trace|debug|info|warn|warning|error|err|fatal)\b`,
)

// hasANSI reports whether s already contains an ANSI escape sequence.
func hasANSI(s string) bool { return strings.Contains(s, "\x1b[") }

// colorizeLogLine finds the first log-level word in the line and renders it
// with charm's log styles. Lines that already carry ANSI escapes are passed
// through untouched so native tool colors (docker, kubectl, etc.) are preserved.
func colorizeLogLine(p Palette, line string) string {
	if hasANSI(line) {
		return line
	}
	// Only scan the prefix — avoids colorizing a stray "error" 200 chars deep.
	scanEnd := 40
	if len(line) < scanEnd {
		scanEnd = len(line)
	}
	m := logLevelRe.FindStringSubmatchIndex(line[:scanEnd])
	if m == nil {
		return ColorOutputLine(p, line)
	}

	// m[2]/m[3] = bounds of capture group 1 (the level word).
	levelStart, levelEnd := m[2], m[3]
	prefix := line[:levelStart]
	tag := line[levelStart:levelEnd]
	rest := line[levelEnd:]

	badge := levelStyle(p, tag).Bold(true).Render(padLevel(tag))

	return Style(p.FgDim, false).Render(prefix) +
		badge +
		Style(p.Fg, false).Render(rest)
}

// padLevel uppercases and pads the level token so colored badges line up.
func padLevel(tag string) string {
	u := strings.ToUpper(tag)
	// Normalize common aliases to 4-char badges.
	switch u {
	case "WARNING":
		u = "WARN"
	case "ERR":
		u = "ERROR"
	}
	// Pad to 5 chars so INFO/WARN/ERROR/DEBUG/TRACE/FATAL align.
	if len(u) < 5 {
		u += strings.Repeat(" ", 5-len(u))
	}
	return " " + u + " "
}

// levelStyle maps a level word to a lipgloss style, preferring charm-log's
// palette but falling back to our theme palette so colors stay readable.
func levelStyle(p Palette, tag string) lipgloss.Style {
	styles := log.DefaultStyles()
	var base lipgloss.Style
	var fallback = p.Fg
	switch strings.ToLower(tag) {
	case "trace":
		base = styles.Levels[log.DebugLevel]
		fallback = p.FgDim
	case "debug":
		base = styles.Levels[log.DebugLevel]
		fallback = p.Cyan
	case "info":
		base = styles.Levels[log.InfoLevel]
		fallback = p.Green
	case "warn", "warning":
		base = styles.Levels[log.WarnLevel]
		fallback = p.Yellow
	case "error", "err":
		base = styles.Levels[log.ErrorLevel]
		fallback = p.Red
	case "fatal":
		base = styles.Levels[log.FatalLevel]
		fallback = p.Red
	default:
		return lipgloss.NewStyle().Foreground(p.Fg)
	}
	// charm-log's default styles may ship without foreground when NO_COLOR is set,
	// so we overlay our fallback to guarantee visible color on all terminals.
	return base.Foreground(fallback)
}
