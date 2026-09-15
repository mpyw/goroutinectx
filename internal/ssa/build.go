package ssa

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

// BuildSSAAnalyzer is the buildssa analyzer that must be in Requires.
var BuildSSAAnalyzer = buildssa.Analyzer

// Build creates an SSA program from the analysis pass.
func Build(pass *analysis.Pass) *Program {
	ssaResult, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok || ssaResult == nil {
		return nil
	}

	return &Program{
		Program:  ssaResult.Pkg.Prog,
		Pkg:      ssaResult.Pkg,
		SrcFuncs: ssaResult.SrcFuncs,
	}
}
