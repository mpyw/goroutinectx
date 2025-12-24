// Package ssa provides SSA-based analysis utilities for goroutinectx.
//
// # Overview
//
// This package provides SSA (Static Single Assignment) analysis capabilities
// for more accurate context propagation detection than AST-only analysis.
//
// # SSA vs AST Analysis
//
// SSA analysis can detect context usage that AST analysis might miss:
//
//	func example(ctx context.Context) {
//	    c := ctx  // Variable assignment
//	    go func() {
//	        doWork(c)  // SSA can trace c back to ctx
//	    }()
//	}
//
// # Program Building
//
// Use [BuildProgram] to create an SSA program from analysis pass:
//
//	ssaProg := ssa.BuildProgram(pass)
//
// The [Program] type wraps the SSA representation:
//
//	type Program struct {
//	    prog *ssa.Program
//	    pkg  *ssa.Package
//	}
//
// # Finding Function Literals
//
// Use [Program.FindFuncLit] to get the SSA function for an AST func literal:
//
//	ssaFn := ssaProg.FindFuncLit(astFuncLit)
//	if ssaFn == nil {
//	    // Fall back to AST analysis
//	}
//
// # Tracer
//
// The [Tracer] analyzes SSA functions for context propagation:
//
//	tracer := ssa.NewTracer()
//
//	// Check if closure captures context
//	captures := tracer.ClosureCapturesContext(ssaFn, carriers)
//
//	// Check if closure calls deriver function
//	result := tracer.ClosureCallsDeriver(ssaFn, deriveMatcher)
//	if result.FoundAtStart {
//	    // Deriver called at goroutine start - OK
//	} else if result.FoundOnlyInDefer {
//	    // Deriver only in defer - warning
//	}
//
// # Closure Free Variables
//
// SSA closures track their captured variables as "free variables":
//
//	func outer(ctx context.Context) {
//	    go func() {
//	        doWork(ctx)  // ctx is a FreeVar of this closure
//	    }()
//	}
//
// The tracer checks if any free variable has context type:
//
//	for _, fv := range closure.FreeVars {
//	    if typeutil.IsContextType(fv.Type()) {
//	        return true  // Closure captures context
//	    }
//	}
//
// # Deriver Detection
//
// For goroutine-derive checking, the tracer:
//
//  1. Collects all function calls in the closure body
//  2. Tracks whether calls are in defer statements
//  3. Checks if deriver functions are called at start vs only in defer
//
// This enables warnings like:
//
//	go func() {
//	    defer apm.NewGoroutineContext(ctx)  // Warning: should call at start
//	    doWork(ctx)
//	}()
//
// # Helper Functions
//
// The package exports helper functions for SSA analysis:
//
//   - [ExtractCalledFunc]: Get types.Func from CallCommon
//   - [ExtractIIFE]: Detect immediately-invoked function expressions
//   - [HasFuncArgs]: Check if call has function-typed arguments
package ssa
