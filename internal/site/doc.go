// Package site gives each go statement and call the checkers check as a
// [Site]: its SSA instruction, the function it calls, and the values it may
// spawn, each with its AST expression and its SSA value.
//
// An [Index] is built once for each pass. It maps each go statement and
// call to its instruction, and each function literal to its SSA function, so
// that a node is looked up once, and not by a scan of the functions:
//
//	s := e.Index.Site(node)
//	for _, arg := range s.Params() {
//	    ok := e.Index.Judge(e.Flow, policy, arg).Passes()
//	}
//
// The receiver of a method is handled here once: [Site.RecvArgs] tells how
// many arguments come before the parameters, and [Site.Recv] gives the
// receiver's value.
package site
