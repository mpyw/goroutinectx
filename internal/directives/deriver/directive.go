// Package deriver handles context derivation logic.
package deriver

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// FuncSpec holds parsed components of a derive function specification.
type FuncSpec struct {
	PkgPath  string
	TypeName string // empty for package-level functions
	FuncName string
}

// Matcher provides OR/AND matching for derive function specifications.
// The check passes if ANY group is fully satisfied (OR semantics).
// A group is satisfied if ALL functions in that group are called (AND semantics).
type Matcher struct {
	OrGroups [][]FuncSpec
	Original string
}

// NewMatcher creates a DeriveMatcher from a derive function string.
// The deriveFuncsStr supports OR (comma) and AND (plus) operators.
// Format: "pkg/path.Func" or "pkg/path.Type.Method".
func NewMatcher(deriveFuncsStr string) *Matcher {
	m := &Matcher{
		Original: deriveFuncsStr,
	}

	// Split by comma first (OR groups)
	for orPart := range strings.SplitSeq(deriveFuncsStr, ",") {
		orPart = strings.TrimSpace(orPart)
		if orPart == "" {
			continue
		}

		// Split by plus (AND within group)
		var andGroup []FuncSpec

		for andPart := range strings.SplitSeq(orPart, "+") {
			andPart = strings.TrimSpace(andPart)
			if andPart == "" {
				continue
			}

			spec := parseFunc(andPart)
			andGroup = append(andGroup, spec)
		}

		if len(andGroup) > 0 {
			m.OrGroups = append(m.OrGroups, andGroup)
		}
	}

	return m
}

// parseFunc parses a single derive function string into components.
// Format: "pkg/path.Func" or "pkg/path.Type.Method".
func parseFunc(s string) FuncSpec {
	spec := FuncSpec{}

	lastDot := strings.LastIndex(s, ".")
	if lastDot == -1 {
		spec.FuncName = s

		return spec
	}

	spec.FuncName = s[lastDot+1:]
	prefix := s[:lastDot]

	// Check if there's another dot (indicating Type.Method)
	// Type names start with uppercase in Go.
	secondLastDot := strings.LastIndex(prefix, ".")
	if secondLastDot != -1 {
		potentialTypeName := prefix[secondLastDot+1:]
		if len(potentialTypeName) > 0 && potentialTypeName[0] >= 'A' && potentialTypeName[0] <= 'Z' {
			spec.PkgPath = prefix[:secondLastDot]
			spec.TypeName = potentialTypeName
		} else {
			spec.PkgPath = prefix
			spec.TypeName = ""
		}
	} else {
		spec.PkgPath = prefix
		spec.TypeName = ""
	}

	return spec
}

// SatisfiesAnyGroup checks if the AST node satisfies ANY of the OR groups.
func (m *Matcher) SatisfiesAnyGroup(pass *analysis.Pass, node ast.Node) bool {
	calledFuncs := collectCalledFuncs(pass, node)

	for _, andGroup := range m.OrGroups {
		if groupSatisfied(calledFuncs, andGroup) {
			return true
		}
	}

	return false
}

// IsEmpty returns true if no derive functions are configured.
func (m *Matcher) IsEmpty() bool {
	return len(m.OrGroups) == 0
}

// MatchesFunc checks if the given function matches ANY spec in ANY OR group.
// This is used to check if a call IS a deriver call (not contains one).
func (m *Matcher) MatchesFunc(fn *types.Func) bool {
	for _, andGroup := range m.OrGroups {
		for _, spec := range andGroup {
			if matchesSpec(fn, spec) {
				return true
			}
		}
	}
	return false
}

// collectCalledFuncs collects all types.Func that are called within the node.
// It does NOT traverse into nested function literals.
func collectCalledFuncs(pass *analysis.Pass, node ast.Node) []*types.Func {
	var funcs []*types.Func

	ast.Inspect(node, func(n ast.Node) bool {
		// Don't traverse into nested function literals
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if fn := extractFunc(pass, call); fn != nil {
			funcs = append(funcs, fn)
		}

		return true
	})

	return funcs
}

// extractFunc extracts the types.Func from a call expression.
func extractFunc(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		obj := pass.TypesInfo.ObjectOf(fun)
		if f, ok := obj.(*types.Func); ok {
			return f
		}

	case *ast.SelectorExpr:
		sel := pass.TypesInfo.Selections[fun]
		if sel != nil {
			if f, ok := sel.Obj().(*types.Func); ok {
				return f
			}
		} else {
			obj := pass.TypesInfo.ObjectOf(fun.Sel)
			if f, ok := obj.(*types.Func); ok {
				return f
			}
		}
	}

	return nil
}

// groupSatisfied checks if ALL specs in the AND group are satisfied.
func groupSatisfied(calledFuncs []*types.Func, andGroup []FuncSpec) bool {
	for _, spec := range andGroup {
		if !specSatisfied(calledFuncs, spec) {
			return false
		}
	}

	return true
}

// specSatisfied checks if the spec is satisfied by any of the called functions.
func specSatisfied(calledFuncs []*types.Func, spec FuncSpec) bool {
	for _, fn := range calledFuncs {
		if matchesSpec(fn, spec) {
			return true
		}
	}

	return false
}

// matchesSpec checks if a types.Func matches the given derive function spec.
func matchesSpec(fn *types.Func, spec FuncSpec) bool {
	if fn.Name() != spec.FuncName {
		return false
	}

	if fn.Pkg() == nil || fn.Pkg().Path() != spec.PkgPath {
		return false
	}

	if spec.TypeName != "" {
		return matchesMethod(fn, spec.TypeName)
	}

	return true
}

// matchesMethod checks if a types.Func is a method on the expected type.
func matchesMethod(fn *types.Func, typeName string) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	recv := sig.Recv()
	if recv == nil {
		return false
	}

	recvType := recv.Type()
	if ptr, ok := recvType.(*types.Pointer); ok {
		recvType = ptr.Elem()
	}

	named, ok := recvType.(*types.Named)
	if !ok {
		return false
	}

	return named.Obj().Name() == typeName
}
