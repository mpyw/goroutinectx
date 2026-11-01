package flow

import (
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// Program wraps an SSA program with the analyzed package.
type Program struct {
	*ssa.Program
	// SrcFuncs are the functions of the package's source, each before the
	// closures it makes: those of buildssa, and the function literals of
	// package-level variables, which buildssa leaves out.
	SrcFuncs []*ssa.Function
}

// ProgramAnalyzer is the buildssa analyzer that must be in Requires.
var ProgramAnalyzer = buildssa.Analyzer

// BuildProgram creates an SSA program from the analysis pass.
func BuildProgram(pass *analysis.Pass) *Program {
	ssaResult, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok || ssaResult == nil {
		return nil
	}

	return &Program{
		Program:  ssaResult.Pkg.Prog,
		SrcFuncs: append(slices.Clone(ssaResult.SrcFuncs), programInitClosures(ssaResult.Pkg)...),
	}
}

// programInitClosures gives the function literals of pkg's package-level
// variables, and the closures they make. They are closures of the package
// initializer, which has no syntax of its own.
func programInitClosures(pkg *ssa.Package) []*ssa.Function {
	init := pkg.Func("init")
	if init == nil {
		return nil
	}
	var funcs []*ssa.Function
	var add func(fn *ssa.Function)
	add = func(fn *ssa.Function) {
		funcs = append(funcs, fn)
		for _, anon := range fn.AnonFuncs {
			add(anon)
		}
	}
	for _, anon := range init.AnonFuncs {
		add(anon)
	}
	return funcs
}
