package tui

import (
	"testing"
)

func TestEnqueueCommand(t *testing.T) {
	m := Model{chainCommands: []string{"a"}}
	m.enqueueCommand("b")
	m.enqueueCommand("c")
	if got, want := m.chainCommands, []string{"a", "b", "c"}; !eqStr(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func eqStr(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
