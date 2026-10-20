package ssa

import (
	"go/ast"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Program wraps an SSA program with the analyzed package.
type Program struct {
	*ssa.Program
	SrcFuncs []*ssa.Function
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

// FuncLitCapturesContext reports whether the closure built from lit takes a
// context parameter, or captures a context or a carrier. ok is false when lit
// has no SSA function.
func (p *Program) FuncLitCapturesContext(lit *ast.FuncLit, carriers []carrier.Carrier) (captures, ok bool) {
	fn := p.FindFuncLit(lit)
	if fn == nil {
		return false, false
	}

	if slices.ContainsFunc(fn.Params, func(param *ssa.Parameter) bool { return typeutil.IsContextType(param.Type()) }) {
		return true, true
	}

	return ClosureCapturesContext(fn, carriers), true
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
		SrcFuncs: ssaResult.SrcFuncs,
	}
}
