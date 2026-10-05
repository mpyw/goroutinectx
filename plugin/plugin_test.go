package plugin_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	_ "github.com/mpyw/goroutinectx/plugin"
)

func TestPluginDefaults(t *testing.T) {
	a := buildAnalyzer(t, nil)
	analysistest.Run(t, testdata(t), a, "goroutine", "errgroup", "waitgroup", "conc", "spawner")
}

func TestPluginLists(t *testing.T) {
	a := buildAnalyzer(t, map[string]any{
		"external-spawner": []any{"github.com/example/workerpool.Pool.Submit", "github.com/example/workerpool.Run"},
	})
	analysistest.Run(t, testdata(t), a, "externalspawner")
}

func TestPluginDeriverAndCarriers(t *testing.T) {
	a := buildAnalyzer(t, map[string]any{
		"goroutine-deriver": "github.com/my-example-app/telemetry/apm.NewGoroutineContext",
		"context-carriers":  []any{"github.com/labstack/echo/v4.Context"},
	})
	analysistest.Run(t, testdata(t), a, "carrierderive")
}

func TestPluginTurnsCheckerOn(t *testing.T) {
	a := buildAnalyzer(t, map[string]any{"spawnerlabel": true})
	analysistest.Run(t, testdata(t), a, "spawnerlabel")
}

// TestPluginTurnsCheckerOff runs the goroutine fixture with that checker off.
// Its expectations then go unmet, and nothing is reported.
func TestPluginTurnsCheckerOff(t *testing.T) {
	a := buildAnalyzer(t, map[string]any{"goroutine": false})
	rec := &unmetRecorder{}
	results := analysistest.Run(rec, testdata(t), a, "goroutine")
	if rec.errors == 0 {
		t.Error("the goroutine fixture's expectations were met with the checker off")
	}
	for _, r := range results {
		for _, d := range r.Diagnostics {
			if !strings.Contains(d.Message, "unused goroutinectx:ignore") {
				t.Errorf("reported with the checker off: %s", d.Message)
			}
		}
	}
}

func TestPluginRejectsBadSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings any
	}{
		{"an unknown key", map[string]any{"goroutines": true}},
		{"a bool given a string", map[string]any{"goroutine": "yes"}},
		{"a list given a string", map[string]any{"external-spawner": "pkg.Func"}},
		{"settings that are not a map", []any{"goroutine"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newPlugin(t)(tt.settings)
			if err == nil || !strings.Contains(err.Error(), "decoding settings") {
				t.Fatalf("got error %v, want a decoding error", err)
			}
		})
	}
}

func TestPluginLoadMode(t *testing.T) {
	p, err := newPlugin(t)(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.GetLoadMode(); got != register.LoadModeTypesInfo {
		t.Errorf("load mode %q, want %q", got, register.LoadModeTypesInfo)
	}
}

// unmetRecorder counts the errors analysistest reports, instead of failing.
type unmetRecorder struct{ errors int }

func (r *unmetRecorder) Errorf(string, ...any) { r.errors++ }

// testdata gives the module's fixtures, shared with the analyzer's tests.
func testdata(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// newPlugin finds the constructor the package registered.
func newPlugin(t *testing.T) register.NewPlugin {
	t.Helper()
	np, err := register.GetPlugin("goroutinectx")
	if err != nil {
		t.Fatal(err)
	}
	return np
}

// buildAnalyzer builds the plugin's one analyzer from settings.
func buildAnalyzer(t *testing.T, settings any) *analysis.Analyzer {
	t.Helper()
	p, err := newPlugin(t)(settings)
	if err != nil {
		t.Fatal(err)
	}
	as, err := p.BuildAnalyzers()
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 1 {
		t.Fatalf("got %d analyzers, want 1", len(as))
	}
	return as[0]
}
