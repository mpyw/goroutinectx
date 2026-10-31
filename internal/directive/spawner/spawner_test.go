package spawner

import (
	"go/token"
	"go/types"
	"path/filepath"
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

// TestParseExternalSkipsBlank checks that a blank entry adds no spec.
func TestParseExternalSkipsBlank(t *testing.T) {
	t.Parallel()

	specs, err := ParseExternal([]string{"", "  ", "github.com/example/pool.Run"})
	if err != nil || len(specs) != 1 {
		t.Fatalf("ParseExternal = %v, %v, want one spec", specs, err)
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
	new(Fact).AFact()
}
