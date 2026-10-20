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

// pluginConfig is the plugin built from one settings block.
type pluginConfig struct {
	cfg run.Config
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
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &pluginConfig{cfg: cfg}, nil
}

// BuildAnalyzers gives goroutinectx's analyzer, configured by the settings.
// It is a copy, so the flags of goroutinectx.Analyzer are left alone.
func (p *pluginConfig) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	cfg := p.cfg
	return []*analysis.Analyzer{{
		Name:      goroutinectx.Analyzer.Name,
		Doc:       goroutinectx.Analyzer.Doc,
		Requires:  goroutinectx.Analyzer.Requires,
		FactTypes: goroutinectx.Analyzer.FactTypes,
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, run.Run(pass, cfg)
		},
	}}, nil
}

// GetLoadMode asks for type information, which the SSA checks need.
func (*pluginConfig) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
