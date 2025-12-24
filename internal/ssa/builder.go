package ssa

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// BuildSSAAnalyzer is the buildssa analyzer that must be in Requires.
var BuildSSAAnalyzer = buildssa.Analyzer

// Program wraps an SSA program with the analyzed package.
type Program struct {
	*ssa.Program
	Pkg      *ssa.Package
	SrcFuncs []*ssa.Function
}

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

// FuncAt returns the SSA function containing the given position.
func (p *Program) FuncAt(pos ast.Node) *ssa.Function {
	for _, fn := range p.SrcFuncs {
		if fn.Pos() <= pos.Pos() && pos.End() <= fn.Syntax().End() {
			return fn
		}
	}
	return nil
}

// FindFuncLit finds the SSA function for a given FuncLit AST node.
func (p *Program) FindFuncLit(lit *ast.FuncLit) *ssa.Function {
	if p == nil || lit == nil {
		return nil
	}

	topFn := p.FuncAt(lit)
	if topFn == nil {
		return nil
	}

	return p.findFuncLitInFunc(topFn, lit)
}

func (p *Program) findFuncLitInFunc(fn *ssa.Function, lit *ast.FuncLit) *ssa.Function {
	for _, anon := range fn.AnonFuncs {
		syntax := anon.Syntax()
		if syntax == nil {
			continue
		}
		if syntax.Pos() == lit.Pos() {
			return anon
		}
		if found := p.findFuncLitInFunc(anon, lit); found != nil {
			return found
		}
	}
	return nil
}

// FindFuncDecl finds the SSA function for a given FuncDecl AST node.
func (p *Program) FindFuncDecl(decl *ast.FuncDecl) *ssa.Function {
	if p == nil || decl == nil {
		return nil
	}

	for _, fn := range p.SrcFuncs {
		syntax := fn.Syntax()
		if syntax == nil {
			continue
		}
		if fnDecl, ok := syntax.(*ast.FuncDecl); ok && fnDecl.Pos() == decl.Pos() {
			return fn
		}
	}
	return nil
}
