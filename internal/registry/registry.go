package registry

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/patterns"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Entry represents a registered API with its pattern.
type Entry struct {
	API     API
	Pattern patterns.Pattern
}

// Registry holds registered APIs and their patterns.
type Registry struct {
	entries []Entry
}

// New creates a new empty registry.
func New() *Registry {
	return &Registry{}
}

// Register adds an API with its pattern to the registry.
func (r *Registry) Register(pattern patterns.Pattern, apis ...API) {
	for _, api := range apis {
		r.entries = append(r.entries, Entry{
			API:     api,
			Pattern: pattern,
		})
	}
}

// Entries returns all registered entries.
func (r *Registry) Entries() []Entry {
	return r.entries
}

// Match attempts to match a call expression against registered APIs.
// Returns the matched entry and callback argument, or nil if no match.
func (r *Registry) Match(pass *analysis.Pass, call *ast.CallExpr) (*Entry, ast.Expr) {
	for i := range r.entries {
		entry := &r.entries[i]
		if callbackArg := r.matchAPI(pass, call, entry.API); callbackArg != nil {
			return entry, callbackArg
		}
	}
	return nil, nil
}

// matchAPI checks if a call matches the given API.
func (r *Registry) matchAPI(pass *analysis.Pass, call *ast.CallExpr, api API) ast.Expr {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Check method/function name
	if sel.Sel.Name != api.Name {
		return nil
	}

	// Check package and type
	switch api.Kind {
	case KindMethod:
		if !r.isMethodCall(pass, sel, api) {
			return nil
		}
	case KindFunc:
		if !r.isFuncCall(pass, sel, api) {
			return nil
		}
	}

	// Get callback argument
	return r.getCallbackArg(call, api)
}

// isMethodCall checks if the selector is a method call on the specified type.
func (r *Registry) isMethodCall(pass *analysis.Pass, sel *ast.SelectorExpr, api API) bool {
	typ := pass.TypesInfo.TypeOf(sel.X)
	if typ == nil {
		return false
	}

	typ = typeutil.UnwrapPointer(typ)

	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	// Handle generic types
	if origin := named.Origin(); origin != nil {
		named = origin
	}

	obj := named.Obj()
	if obj.Name() != api.Type {
		return false
	}

	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}

	return typeutil.MatchPkg(pkg.Path(), api.Pkg)
}

// isFuncCall checks if the selector is a package-level function call.
func (r *Registry) isFuncCall(pass *analysis.Pass, sel *ast.SelectorExpr, api API) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	return typeutil.MatchPkg(pkgName.Imported().Path(), api.Pkg)
}

// getCallbackArg extracts the callback argument from the call.
func (r *Registry) getCallbackArg(call *ast.CallExpr, api API) ast.Expr {
	idx := api.CallbackArgIdx
	if idx < 0 {
		// Negative index means from end (for variadic)
		idx = len(call.Args) + idx
	}

	if idx < 0 || idx >= len(call.Args) {
		return nil
	}

	return call.Args[idx]
}

// MatchFunc attempts to match a types.Func against registered APIs.
// Returns the matched entry or nil if no match.
// This is used for SSA-based analysis where we have types.Func instead of ast.CallExpr.
func (r *Registry) MatchFunc(fn *types.Func) *Entry {
	for i := range r.entries {
		entry := &r.entries[i]
		if matchFuncToAPI(fn, entry.API) {
			return entry
		}
	}
	return nil
}

// matchFuncToAPI checks if a types.Func matches the given API.
func matchFuncToAPI(fn *types.Func, api API) bool {
	if fn == nil {
		return false
	}

	// Check function name
	if fn.Name() != api.Name {
		return false
	}

	// Get package path
	pkg := fn.Pkg()
	if pkg == nil {
		return false
	}

	switch api.Kind {
	case KindMethod:
		// Check receiver type
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			return false
		}
		recv := sig.Recv()
		if recv == nil {
			return false
		}
		recvType := typeutil.UnwrapPointer(recv.Type())
		named, ok := recvType.(*types.Named)
		if !ok {
			return false
		}
		if origin := named.Origin(); origin != nil {
			named = origin
		}
		if named.Obj().Name() != api.Type {
			return false
		}
		return typeutil.MatchPkg(pkg.Path(), api.Pkg)

	case KindFunc:
		// Package-level function
		if api.Type != "" {
			return false // Not a package-level function
		}
		return typeutil.MatchPkg(pkg.Path(), api.Pkg)
	}

	return false
}
