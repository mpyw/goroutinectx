// Package plugin registers goroutinectx as a golangci-lint module plugin.
//
// Import it from .custom-gcl.yml to build a golangci-lint binary that holds
// goroutinectx. The settings in .golangci.yml take the names of the flags of
// the goroutinectx command. external-spawner and context-carriers are lists
// there, not comma-separated strings. A setting left out keeps its default.
package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx"
	"github.com/mpyw/goroutinectx/internal/run"
)

func init() {
	register.Plugin(goroutinectx.Analyzer.Name, newPlugin)
}

// pluginAnalyzer is the plugin built from one settings block.
type pluginAnalyzer struct {
	analyzer *analysis.Analyzer
}

func newPlugin(settings any) (register.LinterPlugin, error) {
	// register.DecodeSettings decodes into a zero value, which would turn
	// off every checker left out. Decode onto the defaults instead. The
	// settings are keyed by run.Config's json tags.
	cfg := run.DefaultConfig()
	if settings != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(settings); err != nil {
			return nil, fmt.Errorf("encoding settings: %w", err)
		}
		dec := json.NewDecoder(&buf)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("decoding settings: %w", err)
		}
	}
	a, err := goroutinectx.New(goroutinectx.Config{
		GoroutineDeriver: cfg.GoroutineDeriver,
		ExternalSpawner:  cfg.ExternalSpawner,
		ContextCarriers:  cfg.ContextCarriers,
		Checkers: map[string]bool{
			"goroutine":    cfg.Goroutine,
			"waitgroup":    cfg.Waitgroup,
			"errgroup":     cfg.Errgroup,
			"conc":         cfg.Conc,
			"spawner":      cfg.Spawner,
			"spawnerlabel": cfg.Spawnerlabel,
			"gotask":       cfg.Gotask,
		},
	})
	if err != nil {
		return nil, err
	}
	return &pluginAnalyzer{analyzer: a}, nil
}

// BuildAnalyzers gives the analyzer that goroutinectx.New built from the
// settings. It has no flags, so goroutinectx.Analyzer is left alone.
func (p *pluginAnalyzer) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{p.analyzer}, nil
}

// GetLoadMode asks for type information, which the SSA checks need.
func (*pluginAnalyzer) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
