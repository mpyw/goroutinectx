package run

import (
	"errors"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// TestRunNeedsItsRequires checks that Run stops with an error, and does not
// panic, when the analyzer that calls it lacks a result it requires, or is
// given a flag value it rejects.
func TestRunNeedsItsRequires(t *testing.T) {
	t.Parallel()

	withInspector := map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New(nil)}

	tests := []struct {
		name     string
		resultOf map[*analysis.Analyzer]any
		cfg      Config
		want     error
	}{
		{name: "no inspector", resultOf: map[*analysis.Analyzer]any{}, cfg: DefaultConfig(), want: ErrNoInspector},
		{name: "no spawners", resultOf: withInspector, cfg: DefaultConfig(), want: errNoSpawners},
		{name: "rejected flag", resultOf: withInspector, cfg: Config{GoroutineDeriver: "apm.NewGoroutineContext"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Run(&analysis.Pass{ResultOf: tt.resultOf}, tt.cfg)
			switch {
			case err == nil:
				t.Fatal("Run gave no error")
			case tt.want != nil && !errors.Is(err, tt.want):
				t.Errorf("Run gave %v, want %v", err, tt.want)
			}
		})
	}
}
