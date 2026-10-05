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
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx"
	"github.com/mpyw/goroutinectx/internal/run"
)

func init() {
	register.Plugin(goroutinectx.Analyzer.Name, newPlugin)
}

// pluginSettings is the settings block, keyed as the flags are.
type pluginSettings struct {
	GoroutineDeriver string   `json:"goroutine-deriver"`
	ExternalSpawner  []string `json:"external-spawner"`
	ContextCarriers  []string `json:"context-carriers"`

	Goroutine    bool `json:"goroutine"`
	Waitgroup    bool `json:"waitgroup"`
	Errgroup     bool `json:"errgroup"`
	Conc         bool `json:"conc"`
	Spawner      bool `json:"spawner"`
	Spawnerlabel bool `json:"spawnerlabel"`
	Gotask       bool `json:"gotask"`
}

// pluginConfig is the plugin built from one settings block.
type pluginConfig struct {
	cfg run.Config
}

func newPlugin(settings any) (register.LinterPlugin, error) {
	d := run.DefaultConfig()
	s := pluginSettings{
		GoroutineDeriver: d.GoroutineDeriver,
		Goroutine:        d.Goroutine,
		Waitgroup:        d.Waitgroup,
		Errgroup:         d.Errgroup,
		Conc:             d.Conc,
		Spawner:          d.Spawner,
		Spawnerlabel:     d.Spawnerlabel,
		Gotask:           d.Gotask,
	}
	// register.DecodeSettings decodes into a zero value, which would turn
	// off every checker left out. Decode onto the defaults instead.
	if settings != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(settings); err != nil {
			return nil, fmt.Errorf("encoding settings: %w", err)
		}
		dec := json.NewDecoder(&buf)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&s); err != nil {
			return nil, fmt.Errorf("decoding settings: %w", err)
		}
	}
	return &pluginConfig{cfg: run.Config{
		GoroutineDeriver: s.GoroutineDeriver,
		ExternalSpawner:  strings.Join(s.ExternalSpawner, ","),
		ContextCarriers:  strings.Join(s.ContextCarriers, ","),
		Goroutine:        s.Goroutine,
		Waitgroup:        s.Waitgroup,
		Errgroup:         s.Errgroup,
		Conc:             s.Conc,
		Spawner:          s.Spawner,
		Spawnerlabel:     s.Spawnerlabel,
		Gotask:           s.Gotask,
	}}, nil
}

// BuildAnalyzers gives goroutinectx's analyzer, configured by the settings.
// It is a copy, so the flags of goroutinectx.Analyzer are left alone.
func (p *pluginConfig) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	cfg := p.cfg
	return []*analysis.Analyzer{{
		Name:     goroutinectx.Analyzer.Name,
		Doc:      goroutinectx.Analyzer.Doc,
		Requires: goroutinectx.Analyzer.Requires,
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, run.Run(pass, cfg)
		},
	}}, nil
}

// GetLoadMode asks for type information, which the SSA checks need.
func (*pluginConfig) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
