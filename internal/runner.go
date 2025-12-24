package internal

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/directive/carrier"
	"github.com/mpyw/goroutinectx/internal/directive/ignore"
	"github.com/mpyw/goroutinectx/internal/probe"
	"github.com/mpyw/goroutinectx/internal/scope"
	"github.com/mpyw/goroutinectx/internal/ssa"
)

// Runner executes checkers on the analysis pass.
type Runner struct {
	goStmtCheckers []GoStmtChecker
	callCheckers   []CallChecker
	ssaProg        *ssa.Program
	tracer         *ssa.Tracer
	carriers       []carrier.Carrier
	ignoreMaps     map[string]ignore.Map
	skipFiles      map[string]bool
}

// NewRunner creates a new runner.
func NewRunner(
	goStmtCheckers []GoStmtChecker,
	callCheckers []CallChecker,
	ssaProg *ssa.Program,
	carriers []carrier.Carrier,
	ignoreMaps map[string]ignore.Map,
	skipFiles map[string]bool,
) *Runner {
	return &Runner{
		goStmtCheckers: goStmtCheckers,
		callCheckers:   callCheckers,
		ssaProg:        ssaProg,
		tracer:         ssa.NewTracer(),
		carriers:       carriers,
		ignoreMaps:     ignoreMaps,
		skipFiles:      skipFiles,
	}
}

// Run executes all checkers on the pass.
func (r *Runner) Run(pass *analysis.Pass, insp *inspector.Inspector) {
	// Build context scopes for functions with context parameters
	funcScopes := scope.Build(pass, insp, r.carriers)

	// Node types we're interested in
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.GoStmt)(nil),
		(*ast.CallExpr)(nil),
	}

	// Check nodes within context-aware functions
	insp.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}

		filename := pass.Fset.Position(n.Pos()).Filename
		if r.skipFiles[filename] {
			return true
		}

		s := scope.FindEnclosing(funcScopes, stack)
		if s == nil {
			return true // No context in scope
		}

		cctx := &probe.Context{
			Pass:     pass,
			Tracer:   r.tracer,
			SSAProg:  r.ssaProg,
			CtxNames: s.CtxNames,
			Carriers: r.carriers,
		}

		switch node := n.(type) {
		case *ast.GoStmt:
			r.checkGoStmt(cctx, node)
		case *ast.CallExpr:
			r.checkCallExpr(cctx, node)
		}

		return true
	})
}

// checkGoStmt runs all GoStmt checkers.
func (r *Runner) checkGoStmt(cctx *probe.Context, stmt *ast.GoStmt) {
	for _, checker := range r.goStmtCheckers {
		if r.shouldIgnore(cctx.Pass, stmt.Pos(), checker.Name()) {
			continue
		}

		result := checker.CheckGoStmt(cctx, stmt)
		if result.OK {
			continue
		}

		msg := result.Message
		if result.DeferMsg != "" {
			msg = result.DeferMsg
		}

		if msg != "" {
			cctx.Pass.Reportf(stmt.Pos(), "%s", msg)
		}
	}
}

// checkCallExpr runs all Call checkers.
func (r *Runner) checkCallExpr(cctx *probe.Context, call *ast.CallExpr) {
	for _, checker := range r.callCheckers {
		if !checker.MatchCall(cctx.Pass, call) {
			continue
		}

		if r.shouldIgnore(cctx.Pass, call.Pos(), checker.Name()) {
			continue
		}

		result := checker.CheckCall(cctx, call)
		if result.OK {
			continue
		}

		if result.Message != "" {
			reportPos := getCallReportPos(call)
			cctx.Pass.Reportf(reportPos, "%s", result.Message)
		}
	}
}

// getCallReportPos returns the best position to report for a call expression.
func getCallReportPos(call *ast.CallExpr) token.Pos {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Pos()
	}
	return call.Pos()
}

// shouldIgnore checks if the position should be ignored for the given checker.
func (r *Runner) shouldIgnore(pass *analysis.Pass, pos token.Pos, checkerName ignore.CheckerName) bool {
	filename := pass.Fset.Position(pos).Filename
	ignoreMap, ok := r.ignoreMaps[filename]
	if !ok {
		return false
	}
	line := pass.Fset.Position(pos).Line
	return ignoreMap.ShouldIgnore(line, checkerName)
}
