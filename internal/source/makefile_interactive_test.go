package source

import (
	"os"
	"testing"
)

func TestInteractiveTagParsing(t *testing.T) {
	mf := `## shell: open python REPL [interactive]
shell:
	python3

## tail: follow log
tail:
	tail -f /var/log/foo

## build: compile [no-interactive]
build:
	go build ./...
`
	tmp, _ := os.CreateTemp("", "Makefile.*")
	tmp.WriteString(mf)
	tmp.Close()
	defer os.Remove(tmp.Name())

	s := &MakefileSource{Path: tmp.Name()}
	cmds, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Command{}
	for _, c := range cmds {
		byName[c.Name] = c
	}
	if !byName["shell"].Interactive {
		t.Errorf("shell should be interactive: %+v", byName["shell"])
	}
	if byName["shell"].Stream {
		t.Errorf("shell should not be stream: %+v", byName["shell"])
	}
	if byName["shell"].Desc != "open python REPL" {
		t.Errorf("shell desc wrong: %q", byName["shell"].Desc)
	}
	if !byName["tail"].Stream {
		t.Errorf("tail should be auto-stream: %+v", byName["tail"])
	}
	if byName["tail"].Interactive {
		t.Errorf("tail should not be interactive: %+v", byName["tail"])
	}
	if byName["build"].Interactive {
		t.Errorf("build should not be interactive (no-interactive): %+v", byName["build"])
	}

	// round-trip render
	state := DocTagState{InteractiveSet: true, Interactive: true}
	if got := renderDocTags(state); got != "[interactive]" {
		t.Errorf("render: got %q", got)
	}
	state = DocTagState{InteractiveSet: true, Interactive: false}
	if got := renderDocTags(state); got != "[no-interactive]" {
		t.Errorf("render no-interactive: got %q", got)
	}
}
