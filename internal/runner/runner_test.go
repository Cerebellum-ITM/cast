package runner

import (
	"reflect"
	"testing"
)

func TestMakeArgs(t *testing.T) {
	cases := []struct {
		name              string
		dir, file, target string
		vars              []string
		want              []string
	}{
		{
			name:   "dir+file+vars",
			dir:    "/proj", file: "Makefile.personal", target: "build",
			vars: []string{"A=1", "B=2"},
			want: []string{"-C", "/proj", "-f", "Makefile.personal", "A=1", "B=2", "build"},
		},
		{
			name:   "file without dir",
			file:   "Makefile.ci", target: "lint",
			want: []string{"-f", "Makefile.ci", "lint"},
		},
		{
			name:   "bare target",
			target: "test",
			want:   []string{"test"},
		},
		{
			name: "dir without file",
			dir:  "/proj", target: "run",
			want: []string{"-C", "/proj", "run"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := makeArgs(tc.dir, tc.file, tc.target, tc.vars)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("makeArgs = %q, want %q", got, tc.want)
			}
		})
	}
}
