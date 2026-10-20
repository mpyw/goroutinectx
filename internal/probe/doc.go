// Package probe holds the per-node state the checkers get.
//
// # Context Structure
//
//	type Context struct {
//	    Pass     *analysis.Pass     // The analysis pass
//	    SSAProg  *ssa.Program       // SSA program representation
//	    CtxNames []string           // Context variable names in scope
//	    Carriers []carrier.Carrier  // Configured carrier types
//	}
//
// A spawned function or callback, including one in a struct field or an
// element, is resolved through SSA (see package ssa).
package probe
