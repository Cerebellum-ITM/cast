// Package version exposes the current cast CLI version as a single
// constant. Every user-facing release bump edits this file; all callers
// (splash screen, --version flag, status bar if ever reinstated) read from
// here so the number cannot drift between surfaces.
package version

// Current is the semver string shown in the splash and reported by the
// CLI. Format: MAJOR.MINOR.PATCH, no leading "v".
const Current = "0.18.1"
