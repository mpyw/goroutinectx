package run

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/checkerdef"
	"github.com/mpyw/goroutinectx/internal/deriver"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/env"
)

// TestRunNeedsItsRequires checks that Run stops with an error, and does not
// panic, when the analyzer that calls it lacks a result it requires, or is
// given a flag value it rejects.
func TestRunNeedsItsRequires(t *testing.T) {
	t.Parallel()

	withInspector := map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New(nil)}
	withSpawners := map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New(nil), spawner.Analyzer: &spawner.Marked{}}

	tests := []struct {
		name     string
		resultOf map[*analysis.Analyzer]any
		cfg      Config
		want     error
	}{
		{name: "no inspector", resultOf: map[*analysis.Analyzer]any{}, cfg: DefaultConfig(), want: ErrNoInspector},
		{name: "no spawners", resultOf: withInspector, cfg: DefaultConfig(), want: ErrNoSpawners},
		{name: "no SSA", resultOf: withSpawners, cfg: DefaultConfig(), want: ErrNoSSA},
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

// TestBuildCheckers checks which checkers buildCheckers builds, and that
// each one runs as a go statement, call or package checker.
func TestBuildCheckers(t *testing.T) {
	t.Parallel()

	derivers, err := deriver.NewMatcher("example.com/apm.NewGoroutineContext")
	if err != nil {
		t.Fatal(err)
	}
	all := DefaultConfig()
	for name := range all.Checkers {
		all.Checkers[name] = true
	}

	tests := []struct {
		name     string
		cfg      Config
		derivers *deriver.Matcher
		want     []checker.Name
	}{
		{
			name: "defaults",
			cfg:  DefaultConfig(),
			want: []checker.Name{checker.Goroutine, checker.Waitgroup, checker.Errgroup, checker.Conc, checker.Spawner},
		},
		{
			name: "defaults with derivers", cfg: DefaultConfig(), derivers: derivers,
			want: []checker.Name{checker.Goroutine, checker.GoroutineDerive, checker.Waitgroup, checker.Errgroup, checker.Conc, checker.Spawner, checker.Gotask},
		},
		{name: "all with derivers", cfg: all, derivers: derivers, want: checkerdef.Names},
		{name: "none", cfg: Config{}},
		{name: "none with derivers", cfg: Config{}, derivers: derivers, want: []checker.Name{checker.GoroutineDerive}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &runner{env: &env.Env{Derivers: tt.derivers}, enabled: make(ignore.EnabledCheckers)}
			packageCheckers := r.buildCheckers(tt.cfg)
			var built []checker.Name
			for _, c := range r.goStmtCheckers {
				built = append(built, c.Name())
			}
			for _, c := range r.callCheckers {
				built = append(built, c.Name())
			}
			for _, c := range packageCheckers {
				built = append(built, c.Name())
			}
			byDefs := func(a, b checker.Name) int {
				return slices.Index(checkerdef.Names, a) - slices.Index(checkerdef.Names, b)
			}
			slices.SortFunc(built, byDefs)
			if !slices.Equal(built, tt.want) {
				t.Errorf("built %v, want %v", built, tt.want)
			}
			enabled := slices.SortedFunc(maps.Keys(r.enabled), byDefs)
			if !slices.Equal(enabled, tt.want) {
				t.Errorf("enabled %v, want %v", enabled, tt.want)
			}
		})
	}
}

// TestConfigUnmarshalJSON checks that settings decode onto the defaults, a
// key matched as a struct field's name would be.
func TestConfigUnmarshalJSON(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	data := `{"Goroutine": false, "spawnerlabel": true, "gotask": null, "external-spawner": ["example.com/pool.Run"]}`
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatal(err)
	}
	want := DefaultConfig()
	want.Checkers[checker.Goroutine] = false
	want.Checkers[checker.Spawnerlabel] = true
	if !maps.Equal(cfg.Checkers, want.Checkers) {
		t.Errorf("checkers %v, want %v", cfg.Checkers, want.Checkers)
	}
	if !slices.Equal(cfg.ExternalSpawner, []string{"example.com/pool.Run"}) {
		t.Errorf("external spawners %v", cfg.ExternalSpawner)
	}

	for _, data := range []string{`{"goroutinederive": true}`, `{"goroutine": "yes"}`, `[]`} {
		if err := json.Unmarshal([]byte(data), new(DefaultConfig())); err == nil {
			t.Errorf("%s decoded with no error", data)
		}
	}
}
