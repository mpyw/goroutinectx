package ssa

import (
	"go/ast"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
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

// GoAt finds the go instruction of stmt, or nil when SSA has none, as for
// unreachable code.
func (p *Program) GoAt(stmt *ast.GoStmt) *ssa.Go {
	fn := p.innermostFuncAt(stmt)
	if fn == nil {
		return nil
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if g, ok := instr.(*ssa.Go); ok && g.Pos() == stmt.Go {
				return g
			}
		}
	}
	return nil
}

// CallAt finds the Call, Go or Defer instruction of call, or nil.
func (p *Program) CallAt(call *ast.CallExpr) ssa.CallInstruction {
	fn := p.innermostFuncAt(call)
	if fn == nil {
		return nil
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if c, ok := instr.(ssa.CallInstruction); ok && c.Common().Pos() == call.Lparen {
				return c
			}
		}
	}
	return nil
}

// innermostFuncAt finds the innermost SSA function whose syntax holds n.
func (p *Program) innermostFuncAt(n ast.Node) *ssa.Function {
	if p == nil {
		return nil
	}
	fn := p.FuncAt(n)
	for fn != nil {
		i := slices.IndexFunc(fn.AnonFuncs, func(anon *ssa.Function) bool {
			syntax := anon.Syntax()
			return syntax != nil && syntax.Pos() <= n.Pos() && n.End() <= syntax.End()
		})
		if i < 0 {
			return fn
		}
		fn = fn.AnonFuncs[i]
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
