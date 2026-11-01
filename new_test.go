package goroutinectx_test

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/goroutinectx"
)

// TestNew runs analyzers that New builds, side by side with different
// settings, to prove that they share nothing with each other or with the
// flags of goroutinectx.Analyzer.
func TestNew(t *testing.T) {
	t.Parallel()
	derive := mustNew(t, goroutinectx.Config{GoroutineDeriver: []string{"github.com/my-example-app/telemetry/apm.NewGoroutineContext"}})
	label := mustNew(t, goroutinectx.Config{Checkers: map[string]bool{"spawnerlabel": true}})
	def := mustNew(t, goroutinectx.Config{})
	if def.Flags.Lookup("goroutine") != nil {
		t.Error("an analyzer New built has flags, want none")
	}
	analysistest.Run(t, analysistest.TestData(), derive, "goroutinederive")
	analysistest.Run(t, analysistest.TestData(), label, "spawnerlabel")
	analysistest.Run(t, analysistest.TestData(), def, "goroutine")
}

func TestNewRejectsBadConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  goroutinectx.Config
		want string
	}{
		{"an unknown checker", goroutinectx.Config{Checkers: map[string]bool{"goroutines": true}}, `unknown checker "goroutines"`},
		// Of several unknown checkers, the first by name, on every call.
		{"several unknown checkers", goroutinectx.Config{Checkers: map[string]bool{"c": true, "a": true, "goroutine": true, "b": false}}, `unknown checker "a"`},
		{"a deriver without an import path", goroutinectx.Config{GoroutineDeriver: []string{"apm.NewGoroutineContext"}}, "import path"},
		{"an empty deriver group", goroutinectx.Config{GoroutineDeriver: []string{""}}, "group 1 is empty"},
		{"a carrier without an import path", goroutinectx.Config{ContextCarriers: []string{"echo.Context"}}, "import path"},
	}
	for _, tt := range tests {
		for range 5 {
			if _, err := goroutinectx.New(tt.cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%s: got error %v, want one containing %q", tt.name, err, tt.want)
			}
		}
	}
}

func mustNew(t *testing.T, cfg goroutinectx.Config) *analysis.Analyzer {
	t.Helper()
	a, err := goroutinectx.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// TestNewSharesNoRequires changes the Requires of one analyzer, and checks
// that the next one New builds still has its own.
func TestNewSharesNoRequires(t *testing.T) {
	t.Parallel()
	a := mustNew(t, goroutinectx.Config{})
	b := mustNew(t, goroutinectx.Config{})
	want := b.Requires[0]
	a.Requires[0] = nil
	if b.Requires[0] != want || goroutinectx.Analyzer.Requires[0] != want {
		t.Error("a change to one analyzer's Requires reached another")
	}
}
