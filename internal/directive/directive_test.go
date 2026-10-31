package directive

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// scanOneComment scans a file that holds only the comment text.
func scanOneComment(text string) Found {
	return Scan(&ast.File{Comments: []*ast.CommentGroup{{List: []*ast.Comment{{Text: text}}}}})
}

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		wantOK   bool
		wantName string
		wantArgs string
	}{
		{name: "canonical", text: "//goroutinectx:ignore", wantOK: true, wantName: "ignore"},
		{name: "args", text: "//goroutinectx:ignore goroutine - reason ", wantOK: true, wantName: "ignore", wantArgs: "goroutine - reason"},
		{name: "spaced", text: "// goroutinectx:ignore", wantOK: false},
		{name: "tab after marker", text: "//\tgoroutinectx:spawner", wantOK: false},
		{name: "space after colon", text: "//goroutinectx: ignore", wantOK: false},
		{name: "block comment", text: "/*goroutinectx:ignore*/", wantOK: false},
		{name: "other tool", text: "//nolint:errcheck", wantOK: false},
		{name: "longer tool", text: "//goroutinectxx:ignore", wantOK: false},
		{name: "shorter tool", text: "//goroutine:ignore", wantOK: false},
		{name: "prose", text: "// goroutinectx is a linter", wantOK: false},
		{name: "uppercase tool", text: "//Goroutinectx:ignore", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d, ok := parse(tt.text)
			if ok != tt.wantOK {
				t.Fatalf("parse(%q) ok = %v, want %v", tt.text, ok, tt.wantOK)
			}
			if d.Name != tt.wantName || d.Args != tt.wantArgs {
				t.Errorf("parse(%q) = {Name: %q, Args: %q}, want {Name: %q, Args: %q}",
					tt.text, d.Name, d.Args, tt.wantName, tt.wantArgs)
			}
		})
	}
}

func TestMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "canonical", text: "//goroutinectx:ignore", want: false},
		{name: "canonical with args", text: "//goroutinectx:spawner //vt:helper", want: false},
		{name: "space after marker", text: "// goroutinectx:ignore", want: true},
		{name: "space after colon", text: "//goroutinectx: ignore", want: true},
		{name: "block comment", text: "/* goroutinectx:spawner */", want: true},
		{name: "uppercase name", text: "//goroutinectx:Ignore", want: true},
		{name: "lookalike name", text: "//goroutinectx:ignored", want: false},
		{name: "longer tool", text: "// goroutinectxx:ignore", want: false},
		{name: "prose", text: "// goroutinectx is a linter", want: false},
		{name: "mentioned mid-comment", text: "// see //goroutinectx:ignore", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			found := scanOneComment(tt.text)
			got := len(found.Problems) == 1 && strings.HasPrefix(found.Problems[0].Message, "malformed ")
			if got != tt.want {
				t.Errorf("Scan(%q) reports it malformed = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestScanUnknownDirective checks that a directive goroutinectx does not
// read is reported, and a known one is not.
func TestScanUnknownDirective(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{
		"ignore":   false,
		"spawner":  false,
		"ignre":    true,
		"ignored":  true,
		"spawnerX": true,
	} {
		found := scanOneComment("//goroutinectx:" + name)
		got := slices.ContainsFunc(found.Problems, func(p Problem) bool {
			return p.Message == "unknown directive goroutinectx:"+name
		})
		if got != want {
			t.Errorf("Scan(%q) reports it unknown = %v, want %v", name, got, want)
		}
	}
}

func TestParseDropsTrailingComment(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]string{
		"//goroutinectx:ignore // reason":           "",
		"//goroutinectx:ignore //reason":            "",
		"//goroutinectx:ignore goroutine // reason": "goroutine",
		"//goroutinectx:spawner //vt:helper":        "",
		"//goroutinectx:ignore // see https://x.y":  "",
	} {
		d, ok := parse(text)
		if !ok || d.Name == "" || d.Args != want {
			t.Errorf("parse(%q) = %q %q %v, want args %q", text, d.Name, d.Args, ok, want)
		}
	}
}

// TestScanSpawners checks which doc comments mark their function a
// spawner, and that a spawner directive anywhere else is reported.
func TestScanSpawners(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		doc      string // The doc comment of f
		spawners int
		problems int
	}{
		{name: "canonical", doc: "//goroutinectx:spawner", spawners: 1},
		{name: "spaced", doc: "// goroutinectx:spawner", problems: 1},
		{name: "block comment", doc: "/* goroutinectx:spawner */", problems: 1},
		{name: "trailing comment", doc: "//goroutinectx:spawner //vt:helper", spawners: 1},
		{name: "longer name", doc: "//goroutinectx:spawnerX", problems: 1},
		{name: "spawnerlabel", doc: "//goroutinectx:spawnerlabel", problems: 1},
		{name: "longer tool", doc: "//goroutinectxx:spawner"},
		{name: "other directive", doc: "//goroutinectx:ignore"},
		{name: "plain comment", doc: "// spawner"},
		{name: "after prose", doc: "// f runs fns.\n//goroutinectx:spawner", spawners: 1},
		{name: "twice", doc: "//goroutinectx:spawner\n//goroutinectx:spawner", spawners: 1},
		{name: "not a doc comment", doc: "//goroutinectx:spawner\n", problems: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := "package p\n\n" + tt.doc + "\nfunc f() {}\n\nfunc g() {}\n"
			file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			found := Scan(file)
			if len(found.Spawners) != tt.spawners {
				t.Errorf("Scan() spawners = %d, want %d", len(found.Spawners), tt.spawners)
			}
			if len(found.Spawners) == 1 && found.Spawners[0].Name.Name != "f" {
				t.Errorf("Scan() marked %s, want f", found.Spawners[0].Name.Name)
			}
			if len(found.Problems) != tt.problems {
				t.Errorf("Scan() problems = %v, want %d", found.Problems, tt.problems)
			}
		})
	}
}
