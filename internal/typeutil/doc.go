// Package typeutil provides type checking utilities for goroutinectx.
//
// # Overview
//
// This package provides helper functions for type checking, focused on
// detecting context.Context types.
//
// # Context Type Detection
//
// Use [IsContextType] to check if a type is context.Context:
//
//	if typeutil.IsContextType(typ) {
//	    // typ is context.Context
//	}
//
// The function handles pointers, aliases and type parameters:
//
//	IsContextType(contextContext)   // true
//	IsContextType(*contextContext)  // true
//	IsContextType(Ctx)              // true, for type Ctx = context.Context
//	IsContextType(C)                // true, for [C context.Context]
//	IsContextType(MyCtx)            // true, for type MyCtx interface{ context.Context }
//
// # Implementation Details
//
// The type checking works by:
//  1. Unwrapping pointer types and aliases ([UnwrapPointer])
//  2. Checking if the type is a named type
//  3. Comparing package path and type name
//  4. Failing that, checking the types an interface embeds, named or not,
//     and the constraint of a type parameter ([Embeds])
//
// Example internal flow:
//
//	func isContextNamed(t types.Type) bool {
//	    t = UnwrapPointer(t)            // *Ctx → context.Context
//	    named, ok := t.(*types.Named)   // Get named type
//	    if !ok { return false }
//	    obj := named.Obj()
//	    return obj.Pkg().Path() == "context" && obj.Name() == "Context"
//	}
//
// # Carrier Type Detection
//
// For carrier types (types that wrap context), use the carrier package:
//
//	import "github.com/mpyw/goroutinectx/internal/carrier"
//
//	if typeutil.IsContextType(typ) || carrier.IsCarrierType(typ, carriers) {
//	    // typ is context.Context or a configured carrier
//	}
package typeutil
