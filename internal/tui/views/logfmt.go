package views

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	log "charm.land/log/v2"
)

// logLevelRe matches common log-level prefixes like "INFO ", "[warn]", "ERROR:".
// Group 1 = optional "[", group 2 = level, group 3 = optional "]".
var logLevelRe = regexp.MustCompile(
	`^(?i)\s*(\[)?(trace|debug|info|warn|warning|error|err|fatal)(\])?[\s:]`,
)

// hasANSI reports whether s already contains an ANSI escape sequence.
func hasANSI(s string) bool { return strings.Contains(s, "\x1b[") }

// colorizeLogLine detects a leading log level and colors the level tag using
// charm's log styles. Lines that already carry ANSI escapes are passed through
// untouched so native tool colors (docker, kubectl, etc.) are preserved.
func colorizeLogLine(p Palette, line string) string {
	if hasANSI(line) {
		return line
	}
	m := logLevelRe.FindStringSubmatchIndex(line)
	if m == nil {
		return ColorOutputLine(p, line)
	}

	// Whole match bounds + the level word bounds.
	levelStart, levelEnd := m[4], m[5]
	tag := line[levelStart:levelEnd]
	rest := line[levelEnd:]
	prefix := line[:levelStart]

	st := levelStyle(tag).
		Padding(0, 1).
		Bold(true)

	return prefix + st.Render(strings.ToUpper(tag)) + Style(p.Fg, false).Render(rest)
}

// levelStyle returns the charm log Style for a level word.
func levelStyle(tag string) lipgloss.Style {
	styles := log.DefaultStyles()
	switch strings.ToLower(tag) {
	case "trace":
		return styles.Levels[log.DebugLevel]
	case "debug":
		return styles.Levels[log.DebugLevel]
	case "info":
		return styles.Levels[log.InfoLevel]
	case "warn", "warning":
		return styles.Levels[log.WarnLevel]
	case "error", "err":
		return styles.Levels[log.ErrorLevel]
	case "fatal":
		return styles.Levels[log.FatalLevel]
	}
	return styles.Levels[log.InfoLevel]
}
