package context

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Scope tracks context availability in a function scope.
// It tracks all context parameters, not just the first one.
type Scope struct {
	// Vars contains all context variables (from go/types).
	// Multiple contexts can be present (e.g., func(ctx1, ctx2 context.Context)).
	Vars []*types.Var

	// Name is the first variable name (for error messages).
	// Using the first name provides consistent, predictable error messages.
	Name string
}

// FindScope finds all context parameters in a function and creates a Scope.
// Returns nil if no context parameter is found.
// If carriers is non-nil, it also considers those types as context carriers.
func FindScope(pass *analysis.Pass, fnType *ast.FuncType, carriers []carrier.Carrier) *Scope {
	if fnType == nil || fnType.Params == nil {
		return nil
	}

	var (
		vars      []*types.Var
		firstName string
	)

	for _, field := range fnType.Params.List {
		tv, ok := pass.TypesInfo.Types[field.Type]
		if !ok {
			continue
		}

		if !typeutil.IsContextOrCarrierType(tv.Type, carriers) {
			continue
		}

		// Found context parameter(s) - collect all names in this field
		for _, name := range field.Names {
			obj := pass.TypesInfo.ObjectOf(name)
			if v, ok := obj.(*types.Var); ok {
				vars = append(vars, v)

				if firstName == "" {
					firstName = name.Name
				}
			}
		}
	}

	if len(vars) == 0 {
		return nil
	}

	return &Scope{
		Vars: vars,
		Name: firstName,
	}
}

// IsContextVar checks if obj matches any of the tracked context variables.
func (s *Scope) IsContextVar(obj types.Object) bool {
	for _, v := range s.Vars {
		if obj == v {
			return true
		}
	}

	return false
}

// UsesContext checks if the given AST node uses any context.Context variable.
// It checks for:
//  1. The original context parameters tracked in this scope
//  2. ANY variable with type context.Context (to handle shadowing like `ctx := errgroup.WithContext(ctx)`)
//
// It uses type information to correctly handle shadowing.
// It does NOT descend into nested function literals (closures) - each closure
// should be checked separately with its own scope.
// Returns true if ANY context.Context variable is used.
func (s *Scope) UsesContext(pass *analysis.Pass, node ast.Node) bool {
	if s == nil || len(s.Vars) == 0 {
		return false
	}

	found := false

	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		// Skip nested function literals - they will be checked separately
		if _, ok := n.(*ast.FuncLit); ok && n != node {
			return false
		}

		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		obj := pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}

		// Check 1: Is it one of the original context parameters?
		if s.IsContextVar(obj) {
			found = true

			return false
		}

		// Check 2: Is it ANY variable with type context.Context?
		// This handles shadowing like: ctx := errgroup.WithContext(ctx)
		if v, ok := obj.(*types.Var); ok {
			if typeutil.IsContextType(v.Type()) {
				found = true

				return false
			}
		}

		return true
	})

	return found
}

// HasContextOrCarrierParam checks if the function type has a context.Context
// or a context carrier type as a parameter.
func HasContextOrCarrierParam(pass *analysis.Pass, fnType *ast.FuncType, carriers []carrier.Carrier) bool {
	return FindScope(pass, fnType, carriers) != nil
}
