// Package deriver handles context derivation logic.
package deriver

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// Matcher provides OR/AND matching for derive function specifications.
// The check passes if ANY group is fully satisfied (OR semantics).
// A group is satisfied if ALL functions in that group are called (AND semantics).
type Matcher struct {
	OrGroups [][]funcspec.Spec
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
		var andGroup []funcspec.Spec

		for andPart := range strings.SplitSeq(orPart, "+") {
			andPart = strings.TrimSpace(andPart)
			if andPart == "" {
				continue
			}

			spec := funcspec.Parse(andPart)
			andGroup = append(andGroup, spec)
		}

		if len(andGroup) > 0 {
			m.OrGroups = append(m.OrGroups, andGroup)
		}
	}

	return m
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
			if spec.Matches(fn) {
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

		if fn := funcspec.ExtractFunc(pass, call); fn != nil {
			funcs = append(funcs, fn)
		}

		return true
	})

	return funcs
}

// groupSatisfied checks if ALL specs in the AND group are satisfied.
func groupSatisfied(calledFuncs []*types.Func, andGroup []funcspec.Spec) bool {
	for _, spec := range andGroup {
		if !specSatisfied(calledFuncs, spec) {
			return false
		}
	}

	return true
}

// specSatisfied checks if the spec is satisfied by any of the called functions.
func specSatisfied(calledFuncs []*types.Func, spec funcspec.Spec) bool {
	for _, fn := range calledFuncs {
		if spec.Matches(fn) {
			return true
		}
	}

	return false
}
