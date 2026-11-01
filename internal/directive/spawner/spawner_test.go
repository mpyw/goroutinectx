package spawner

import (
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzerExportsFacts checks that Analyzer exports a fact for each
// exported function marked with the directive, and only for those.
func TestAnalyzerExportsFacts(t *testing.T) {
	t.Parallel()

	testdata := filepath.Join("..", "..", "..", "testdata")
	abs, err := filepath.Abs(testdata)
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, abs, Analyzer, "spawnermarked")
}

// TestParseExternal checks that ParseExternal gives a spec for each entry.
func TestParseExternal(t *testing.T) {
	t.Parallel()

	specs, err := ParseExternal([]string{" github.com/example/pool.Run ", "github.com/example/pool.Pool.Go"})
	if err != nil || len(specs) != 2 {
		t.Fatalf("ParseExternal = %v, %v, want two specs", specs, err)
	}
}

// TestParseExternalRejects checks that a blank entry, an entry that holds a
// comma, and a specification with no import path are errors.
func TestParseExternalRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"an empty item", []string{"github.com/example/pool.Run", ""}, "-external-spawner: item 2 is empty"},
		{"a blank item", []string{"  "}, "-external-spawner: item 1 is empty"},
		{
			"an item with a comma", []string{"github.com/example/pool.Run,github.com/example/pool.Go"},
			`-external-spawner: "github.com/example/pool.Run,github.com/example/pool.Go": each item is one function, and commas separate functions only on the command line`,
		},
		{"no import path", []string{"pool.Run"}, `-external-spawner: "pool.Run": "pool" is not an import path`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseExternal(tt.parts); err == nil || !strings.HasPrefix(err.Error(), tt.want) {
				t.Errorf("got error %v, want one starting with %q", err, tt.want)
			}
		})
	}
}

// TestNilMapHasNoSpawner checks that a nil Map, as a run without spawners
// gives, marks nothing.
func TestNilMapHasNoSpawner(t *testing.T) {
	t.Parallel()

	fn := types.NewFunc(token.NoPos, nil, "Run", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	if (*Map)(nil).IsSpawner(fn) {
		t.Error("a nil Map marked a function")
	}
	if (*Map)(nil).HasDirective(fn) {
		t.Error("a nil Map found a directive")
	}
	new(Fact).AFact()
}
