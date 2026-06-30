package tui

import "testing"

// When a command runs, the center detail panel collapses and the logs panel
// zooms to fill everything to the right of the (unchanged) sidebar. The widths
// must still tile exactly: sidebar + divider + logs == m.width.
func TestRunningZoomLayout(t *testing.T) {
	m := Model{
		width:           120,
		height:          40,
		showCenter:      true,
		sidebarWidthPct: 25,
		outputWidthPct:  30,
	}

	// Idle: center visible, logs use the configured percentage.
	if !m.mainShowCenter() {
		t.Fatalf("idle: expected center visible")
	}
	if got, want := m.mainOutputW(), m.outputPanelW(); got != want {
		t.Fatalf("idle: mainOutputW = %d, want outputPanelW %d", got, want)
	}

	// Running: center hidden, logs zoom, sidebar untouched.
	m.running = true
	if m.mainShowCenter() {
		t.Fatalf("running: expected center hidden")
	}
	if got, want := m.sidebarPanelW(), 120*25/100; got != want {
		t.Fatalf("running: sidebar width changed to %d, want %d (must stay put)", got, want)
	}
	// sbInner + 1 divider col + outInner must equal the full width.
	sbInner := m.sidebarPanelW() - 1
	outInner := m.mainOutputW() - 1
	if total := sbInner + 1 + outInner; total != m.width {
		t.Fatalf("running: panels tile to %d cols, want %d", total, m.width)
	}
	// Logs must be wider while running than the idle percentage layout.
	if m.mainOutputW() <= m.outputPanelW() {
		t.Fatalf("running: logs did not zoom (%d <= idle %d)", m.mainOutputW(), m.outputPanelW())
	}
}
