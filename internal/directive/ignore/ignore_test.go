package ignore

import (
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/directive"
)

// TestBuildIgnoreForms checks which comments are an ignore, the checkers
// each names, and the ignores Build reports for an unknown checker.
func TestBuildIgnoreForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		want    []checker.Name
		unknown string // the unknown checker the problem names
		wantOK  bool
	}{
		{name: "canonical", text: "//goroutinectx:ignore", wantOK: true},
		{name: "spaced", text: "// goroutinectx:ignore", wantOK: false},
		{name: "trailing space", text: "//goroutinectx:ignore  ", wantOK: true},
		{name: "one checker", text: "//goroutinectx:ignore goroutine", want: []checker.Name{checker.Goroutine}, wantOK: true},
		{name: "spaced with checker", text: "// goroutinectx:ignore goroutine", wantOK: false},
		{name: "block comment", text: "/* goroutinectx:ignore */", wantOK: false},
		{name: "checker list", text: "//goroutinectx:ignore goroutine,errgroup", want: []checker.Name{checker.Goroutine, checker.Errgroup}, wantOK: true},
		{name: "checker list with spaces", text: "//goroutinectx:ignore goroutine, errgroup", want: []checker.Name{checker.Goroutine, checker.Errgroup}, wantOK: true},
		{name: "dash reason", text: "//goroutinectx:ignore - fire-and-forget", wantOK: true},
		{name: "bare dash", text: "//goroutinectx:ignore -", wantOK: true},
		{name: "checker with dash reason", text: "//goroutinectx:ignore goroutine - fire-and-forget", want: []checker.Name{checker.Goroutine}, wantOK: true},
		{name: "checker with slash reason", text: "//goroutinectx:ignore errgroup // want `x`", want: []checker.Name{checker.Errgroup}, wantOK: true},
		{name: "checker list with slash reason", text: "//goroutinectx:ignore goroutine,errgroup //reason", want: []checker.Name{checker.Goroutine, checker.Errgroup}, wantOK: true},
		{name: "bare slash reason", text: "//goroutinectx:ignore // fire-and-forget", wantOK: true},
		{name: "bare slash reason without space", text: "//goroutinectx:ignore //fire-and-forget", wantOK: true},
		{name: "bare want comment", text: "//goroutinectx:ignore // want `x`", wantOK: true},
		{name: "slash reason holding a URL", text: "//goroutinectx:ignore // see https://example.com", wantOK: true},
		{name: "conc", text: "//goroutinectx:ignore conc", want: []checker.Name{checker.Conc}, wantOK: true},
		{name: "reason without dash", text: "//goroutinectx:ignore intentionally detached", unknown: "intentionally detached", wantOK: true},
		{name: "checker then reason without dash", text: "//goroutinectx:ignore goroutine intentionally detached", unknown: "goroutine intentionally detached", wantOK: true},
		{name: "misspelled checker", text: "//goroutinectx:ignore gorutine", unknown: "gorutine", wantOK: true},
		{name: "one misspelled in a list", text: "//goroutinectx:ignore goroutine,typo", unknown: "typo", wantOK: true},
		{name: "longer name", text: "//goroutinectx:ignored", wantOK: false},
		{name: "longer name with args", text: "//goroutinectx:ignoregoroutine", wantOK: false},
		{name: "longer tool", text: "//goroutinectxx:ignore", wantOK: false},
		{name: "other directive", text: "//goroutinectx:spawner", wantOK: false},
		{name: "other tool", text: "//nolint:ignore", wantOK: false},
		{name: "plain comment", text: "// ignore this", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "p.go", "package p\n\n"+tt.text+"\n", parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			m, problems := Build(fset, file, directive.Scan(file).Ignores)
			var got []checker.Name
			if e := m[3]; e != nil {
				got = e.checkers
			}
			problem := ""
			if len(problems) == 1 {
				problem = problems[0].Message
			}
			if ok := len(m) == 1 || problem != ""; ok != tt.wantOK {
				t.Fatalf("Build(%q) found an ignore = %v, want %v", tt.text, ok, tt.wantOK)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Build(%q) ignore checkers = %q, want %q", tt.text, got, tt.want)
			}
			if tt.unknown == "" && problem != "" {
				t.Errorf("Build(%q) problem = %q, want none", tt.text, problem)
			}
			if tt.unknown != "" && !strings.HasPrefix(problem, "unknown checker \""+tt.unknown+"\" in goroutinectx:ignore (want one of ") {
				t.Errorf("Build(%q) problem = %q, want one naming %q", tt.text, problem, tt.unknown)
			}
		})
	}
}

func TestBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		src      string
		trailing map[int]bool // Line of each ignore → whether it follows code
		problems int
	}{
		{
			name: "no directive",
			src:  "package p\n\n// A comment.\nfunc f() {}\n",
		},
		{
			name:     "unknown checker only",
			src:      "package p\n\n//goroutinectx:ignore typo\nfunc f() {}\n",
			problems: 1,
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
			problems: 1,
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
			m, problems := Build(fset, file, directive.Scan(file).Ignores)
			if len(problems) != tt.problems {
				t.Errorf("Build() problems = %v, want %d", problems, tt.problems)
			}
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
	m, _ := Build(fset, file, directive.Scan(file).Ignores)

	tests := []struct {
		line int
		name checker.Name
		want *Entry
	}{
		{line: 4, name: checker.Goroutine, want: m[4]},
		{line: 5, name: checker.Goroutine, want: m[4]},
		{line: 5, name: checker.Errgroup},
		{line: 6, name: checker.Errgroup, want: m[6]},
		{line: 7, name: checker.Goroutine}, // The ignore above follows code
		{line: 8, name: checker.Goroutine},
	}
	for _, tt := range tests {
		if got := m.Covering(tt.line, tt.name); got != tt.want {
			t.Errorf("Covering(%d, %s) = %v, want %v", tt.line, tt.name, got, tt.want)
		}
	}

	enabled := EnabledCheckers{checker.Goroutine: true}
	if unused := m.GetUnusedIgnores(enabled); len(unused) != 2 {
		t.Errorf("GetUnusedIgnores() after Covering = %v, want both ignores", unused)
	}
	m[4].MarkUsed(checker.Goroutine)
	if unused := m.GetUnusedIgnores(enabled); len(unused) != 1 || unused[0].Pos != m[6].pos {
		t.Errorf("GetUnusedIgnores() after MarkUsed = %v, want the one on line 6", unused)
	}
}
