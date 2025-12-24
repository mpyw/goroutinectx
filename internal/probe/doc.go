// Package probe provides AST analysis helpers for context propagation detection.
//
// # Overview
//
// The probe package provides [Context] which encapsulates the analysis state
// and provides methods for probing AST nodes. It is the primary interface
// between checkers and the underlying analysis infrastructure.
//
// # Context Structure
//
//	type Context struct {
//	    Pass     *analysis.Pass       // The analysis pass
//	    Tracer   *ssa.Tracer          // SSA-based value tracer
//	    SSAProg  *ssa.Program         // SSA program representation
//	    CtxNames []string             // Context variable names in scope
//	    Carriers []carrier.Carrier    // Configured carrier types
//	}
//
// # Analysis Methods
//
// Context provides several categories of analysis methods:
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│                      Analysis Method Categories                      │
//	├──────────────────────┬──────────────────────────────────────────────┤
//	│ Context Capture      │ FuncLitCapturesContext, FuncLitUsesContext   │
//	│ Parameter Detection  │ FuncLitHasContextParam, FuncTypeHasContextParam│
//	│ Factory Functions    │ FactoryCallReturnsContextUsingFunc           │
//	│ Variable Resolution  │ FuncLitOfIdent                               │
//	│ SSA Analysis         │ FuncLitCapturesContextSSA                    │
//	└──────────────────────┴──────────────────────────────────────────────┘
//
// # SSA vs AST Analysis
//
// The probe package uses a two-tier analysis approach:
//
//  1. SSA-based analysis (preferred): More accurate but may not always succeed
//  2. AST-based analysis (fallback): Always available, conservative
//
// Example pattern used by checkers:
//
//	func checkFunc(cctx *probe.Context, lit *ast.FuncLit) bool {
//	    // Try SSA first
//	    if result, ok := cctx.FuncLitCapturesContextSSA(lit); ok {
//	        return result
//	    }
//	    // Fall back to AST
//	    return cctx.FuncLitCapturesContext(lit)
//	}
//
// # Context Capture Detection
//
// The package determines if a closure "captures" context through several mechanisms:
//
//	// Direct capture - context variable referenced in closure body
//	go func() {
//	    doWork(ctx)  // ctx is captured from enclosing scope
//	}()
//
//	// Parameter capture - closure has context parameter
//	go func(ctx context.Context) {
//	    doWork(ctx)  // ctx passed as parameter
//	}(ctx)
//
//	// Factory capture - closure returned by factory that receives context
//	go makeWorker(ctx)()  // factory receives context
//
// # Carrier Types
//
// Beyond context.Context, the analyzer supports custom carrier types configured
// via the -carrier flag. These are types that wrap or carry context:
//
//	// Example: echo.Context carries request context
//	// Configure with: -carrier=github.com/labstack/echo/v4.Context
//	func handler(c echo.Context) {
//	    go func() {
//	        processRequest(c)  // c is a carrier type
//	    }()
//	}
package probe
