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
// The function handles pointer types automatically:
//
//	IsContextType(contextContext)   // true
//	IsContextType(*contextContext)  // true
//
// # Implementation Details
//
// The type checking works by:
//  1. Unwrapping pointer types
//  2. Checking if the type is a named type
//  3. Comparing package path and type name
//
// Example internal flow:
//
//	func IsContextType(t types.Type) bool {
//	    t = unwrapPointer(t)            // *context.Context → context.Context
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
//	import "github.com/mpyw/goroutinectx/internal/directive/carrier"
//
//	if typeutil.IsContextType(typ) || carrier.IsCarrierType(typ, carriers) {
//	    // typ is context.Context or a configured carrier
//	}
package typeutil
