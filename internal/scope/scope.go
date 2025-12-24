// Package scope provides context scope detection for functions.
package scope

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/directive/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Scope holds context information for a function scope.
type Scope struct {
	CtxNames []string
}

// Map maps AST nodes to their scopes.
type Map map[ast.Node]*Scope

// Build identifies functions with context parameters.
func Build(pass *analysis.Pass, insp *inspector.Inspector, carriers []carrier.Carrier) Map {
	m := make(Map)

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, func(n ast.Node) {
		var fnType *ast.FuncType

		switch fn := n.(type) {
		case *ast.FuncDecl:
			fnType = fn.Type
		case *ast.FuncLit:
			fnType = fn.Type
		}

		if scope := findScope(pass, fnType, carriers); scope != nil {
			m[n] = scope
		}
	})

	return m
}

// findScope checks if the function has context parameters.
func findScope(pass *analysis.Pass, fnType *ast.FuncType, carriers []carrier.Carrier) *Scope {
	if fnType == nil || fnType.Params == nil {
		return nil
	}

	var ctxNames []string

	for _, field := range fnType.Params.List {
		typ := pass.TypesInfo.TypeOf(field.Type)
		if typ == nil {
			continue
		}

		if typeutil.IsContextType(typ) || carrier.IsCarrierType(typ, carriers) {
			for _, name := range field.Names {
				ctxNames = append(ctxNames, name.Name)
			}
		}
	}

	if len(ctxNames) == 0 {
		return nil
	}

	return &Scope{CtxNames: ctxNames}
}

// FindEnclosing finds the closest enclosing function with a context parameter.
func FindEnclosing(scopes Map, stack []ast.Node) *Scope {
	for i := len(stack) - 1; i >= 0; i-- {
		if scope, ok := scopes[stack[i]]; ok {
			return scope
		}
	}

	return nil
}
