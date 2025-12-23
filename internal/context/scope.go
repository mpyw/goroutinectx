package context

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/goroutinectx/internal/directives/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Scope holds context information for a function scope.
type Scope struct {
	CtxNames []string
}

// CtxName returns the first context name, or "ctx" as default.
func (s *Scope) CtxName() string {
	if len(s.CtxNames) > 0 {
		return s.CtxNames[0]
	}
	return "ctx"
}

// BuildFuncScopes identifies functions with context parameters.
func BuildFuncScopes(
	pass *analysis.Pass,
	insp *inspector.Inspector,
	carriers []carrier.Carrier,
) map[ast.Node]*Scope {
	funcScopes := make(map[ast.Node]*Scope)

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, func(n ast.Node) {
		var fnType *ast.FuncType

		switch fn := n.(type) {
		case *ast.FuncDecl:
			fnType = fn.Type
		case *ast.FuncLit:
			fnType = fn.Type
		}

		if scope := findScope(pass, fnType, carriers); scope != nil {
			funcScopes[n] = scope
		}
	})

	return funcScopes
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

		if typeutil.IsContextOrCarrierType(typ, carriers) {
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

// FindEnclosingScope finds the closest enclosing function with a context parameter.
func FindEnclosingScope(funcScopes map[ast.Node]*Scope, stack []ast.Node) *Scope {
	for i := len(stack) - 1; i >= 0; i-- {
		if scope, ok := funcScopes[stack[i]]; ok {
			return scope
		}
	}

	return nil
}
