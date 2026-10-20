// Package probe holds the per-node state the checkers get, and the AST
// lookups that SSA does not cover.
//
// # Context Structure
//
//	type Context struct {
//	    Pass      *analysis.Pass        // The analysis pass
//	    Inspector *inspector.Inspector  // Inspector over the pass files
//	    SSAProg   *ssa.Program          // SSA program representation
//	    CtxNames  []string              // Context variable names in scope
//	    Carriers  []carrier.Carrier     // Configured carrier types
//	}
//
// # AST Lookups
//
// A spawned function or callback is resolved through SSA (see package ssa).
// Two forms are still looked up on the AST: a struct field and an indexed
// element of a composite literal assigned to a variable.
//
//	s := S{F: func() { ... }}
//	go s.F()        // SelectorExprCapturesContext
//
//	fs := []func(){func() { ... }}
//	go fs[0]()      // IndexExprCapturesContext
//
// The function literal found is checked by [Context.FuncLitUsesContext]: its
// body references a context or a configured carrier, outside nested literals.
package probe
