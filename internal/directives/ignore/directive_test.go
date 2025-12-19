package ignore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestAllCheckerNames(t *testing.T) {
	names := AllCheckerNames()
	if len(names) != 7 {
		t.Errorf("Expected 7 checker names, got %d", len(names))
	}

	expected := map[CheckerName]bool{
		Goroutine:       true,
		GoroutineDerive: true,
		Waitgroup:       true,
		Errgroup:        true,
		Spawner:         true,
		Spawnerlabel:    true,
		Gotask:          true,
	}

	for _, name := range names {
		if !expected[name] {
			t.Errorf("Unexpected checker name: %s", name)
		}
	}
}

func TestParseIgnoreComment(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		want   []CheckerName
		wantOk bool
	}{
		{
			name:   "basic ignore all",
			text:   "//goroutinectx:ignore",
			want:   nil,
			wantOk: true,
		},
		{
			name:   "ignore specific checker",
			text:   "//goroutinectx:ignore goroutine",
			want:   []CheckerName{Goroutine},
			wantOk: true,
		},
		{
			name:   "ignore multiple checkers",
			text:   "//goroutinectx:ignore goroutine,errgroup",
			want:   []CheckerName{Goroutine, Errgroup},
			wantOk: true,
		},
		{
			name:   "ignore with comment dash",
			text:   "//goroutinectx:ignore - this is a reason",
			want:   nil,
			wantOk: true,
		},
		{
			name:   "ignore specific with comment",
			text:   "//goroutinectx:ignore goroutine - this is a reason",
			want:   []CheckerName{Goroutine},
			wantOk: true,
		},
		{
			name:   "not an ignore comment",
			text:   "// regular comment",
			want:   nil,
			wantOk: false,
		},
		{
			name:   "ignore with leading space",
			text:   "// goroutinectx:ignore",
			want:   nil,
			wantOk: true,
		},
		{
			name:   "ignore with inline comment",
			text:   "//goroutinectx:ignore goroutine // comment",
			want:   []CheckerName{Goroutine},
			wantOk: true,
		},
		{
			name:   "ignore dash only",
			text:   "//goroutinectx:ignore -",
			want:   nil,
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseIgnoreComment(tt.text)
			if ok != tt.wantOk {
				t.Errorf("parseIgnoreComment() ok = %v, want %v", ok, tt.wantOk)
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseIgnoreComment() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseIgnoreComment()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuild(t *testing.T) {
	src := `package test

//goroutinectx:ignore
func ignored() {}

//goroutinectx:ignore goroutine
func ignoredGoroutine() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	m := Build(fset, file)

	// Should have 2 entries
	if len(m) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(m))
	}
}

func TestShouldIgnore(t *testing.T) {
	src := `package test

//goroutinectx:ignore
func line3() {}

//goroutinectx:ignore goroutine
func line6() {}

//goroutinectx:ignore errgroup
func line9() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	m := Build(fset, file)

	// Line 3: ignore all -> should ignore goroutine
	if !m.ShouldIgnore(3, Goroutine) && !m.ShouldIgnore(4, Goroutine) {
		t.Error("Expected line 3-4 to ignore goroutine")
	}

	// Line 6: ignore goroutine -> should ignore goroutine
	if !m.ShouldIgnore(6, Goroutine) && !m.ShouldIgnore(7, Goroutine) {
		t.Error("Expected line 6-7 to ignore goroutine")
	}

	// Line 6: ignore goroutine -> should NOT ignore errgroup
	if m.ShouldIgnore(6, Errgroup) || m.ShouldIgnore(7, Errgroup) {
		t.Error("Expected line 6-7 to NOT ignore errgroup")
	}

	// Line 9: ignore errgroup -> should NOT ignore goroutine
	if m.ShouldIgnore(9, Goroutine) || m.ShouldIgnore(10, Goroutine) {
		t.Error("Expected line 9-10 to NOT ignore goroutine")
	}

	// Line 100: no comment -> should NOT ignore anything
	if m.ShouldIgnore(100, Goroutine) {
		t.Error("Expected line 100 to NOT ignore goroutine")
	}
}

func TestGetUnusedIgnores(t *testing.T) {
	src := `package test

//goroutinectx:ignore
func unusedIgnoreAll() {}

//goroutinectx:ignore goroutine
func unusedIgnoreSpecific() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	m := Build(fset, file)

	// All checkers enabled but not used
	enabled := EnabledCheckers{
		Goroutine:       true,
		GoroutineDerive: true,
		Waitgroup:       true,
		Errgroup:        true,
		Spawner:         true,
		Spawnerlabel:    true,
		Gotask:          true,
	}

	unused := m.GetUnusedIgnores(enabled)

	// Should have 2 unused ignores
	if len(unused) != 2 {
		t.Errorf("Expected 2 unused ignores, got %d", len(unused))
	}
}

func TestGetUnusedIgnoresWithUsed(t *testing.T) {
	fset := token.NewFileSet()

	// Create a simple file manually
	file := &ast.File{
		Comments: []*ast.CommentGroup{
			{
				List: []*ast.Comment{
					{Slash: token.Pos(10), Text: "//goroutinectx:ignore"},
				},
			},
		},
	}

	// Build manually since we don't have proper position info
	m := Build(fset, file)

	// Use one of the entries
	enabled := EnabledCheckers{Goroutine: true}
	line := fset.Position(token.Pos(10)).Line
	m.ShouldIgnore(line, Goroutine)

	unused := m.GetUnusedIgnores(enabled)

	// Should have 0 unused ignores (the one we have was used)
	if len(unused) != 0 {
		t.Errorf("Expected 0 unused ignores, got %d", len(unused))
	}
}
