package ignore

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/directive"
)

func TestBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		src      string
		trailing map[int]bool // Line of each ignore → whether it follows code
	}{
		{
			name: "no directive",
			src:  "package p\n\n// A comment.\nfunc f() {}\n",
		},
		{
			name: "unknown checker only",
			src:  "package p\n\n//goroutinectx:ignore typo\nfunc f() {}\n",
		},
		{
			name:     "own line",
			src:      "package p\n\nfunc f() {\n\t//goroutinectx:ignore\n\tgo f()\n}\n",
			trailing: map[int]bool{4: false},
		},
		{
			name:     "trailing",
			src:      "package p\n\nfunc f() {\n\tgo f() //goroutinectx:ignore goroutine\n}\n",
			trailing: map[int]bool{4: true},
		},
		{
			name:     "after the end of a multi-line call",
			src:      "package p\n\nfunc g(int) {}\n\nfunc f() {\n\tg(\n\t\t1,\n\t) //goroutinectx:ignore\n}\n",
			trailing: map[int]bool{8: true},
		},
		{
			name:     "with an unknown checker",
			src:      "package p\n\nfunc f() {\n\t//goroutinectx:ignore gorutine\n\tgo f() //goroutinectx:ignore\n}\n",
			trailing: map[int]bool{5: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "p.go", tt.src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			m := Build(fset, file, directive.Scan(file).Ignores)
			if len(m) != len(tt.trailing) {
				t.Errorf("Build() has %d ignores, want %d", len(m), len(tt.trailing))
			}
			for line, trailing := range tt.trailing {
				entry := m[line]
				if entry == nil {
					t.Errorf("Build() has no ignore on line %d", line)
					continue
				}
				if entry.trailing != trailing {
					t.Errorf("Build() ignore on line %d trailing = %v, want %v", line, entry.trailing, trailing)
				}
			}
		})
	}
}

// TestCovering checks which ignore covers a line, and that asking marks
// none of them used.
func TestCovering(t *testing.T) {
	t.Parallel()

	src := "package p\n\nfunc f() {\n\t//goroutinectx:ignore goroutine\n\tgo f()\n\tgo f() //goroutinectx:ignore\n\tgo f()\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	m := Build(fset, file, directive.Scan(file).Ignores)

	tests := []struct {
		line int
		name checkername.Name
		want *Entry
	}{
		{line: 4, name: checkername.Goroutine, want: m[4]},
		{line: 5, name: checkername.Goroutine, want: m[4]},
		{line: 5, name: checkername.Errgroup},
		{line: 6, name: checkername.Errgroup, want: m[6]},
		{line: 7, name: checkername.Goroutine}, // The ignore above follows code
		{line: 8, name: checkername.Goroutine},
	}
	for _, tt := range tests {
		if got := m.Covering(tt.line, tt.name); got != tt.want {
			t.Errorf("Covering(%d, %s) = %v, want %v", tt.line, tt.name, got, tt.want)
		}
	}

	enabled := EnabledCheckers{checkername.Goroutine: true}
	if unused := m.GetUnusedIgnores(enabled); len(unused) != 2 {
		t.Errorf("GetUnusedIgnores() after Covering = %v, want both ignores", unused)
	}
	m[4].MarkUsed(checkername.Goroutine)
	if unused := m.GetUnusedIgnores(enabled); len(unused) != 1 || unused[0].Pos != m[6].pos {
		t.Errorf("GetUnusedIgnores() after MarkUsed = %v, want the one on line 6", unused)
	}
}
