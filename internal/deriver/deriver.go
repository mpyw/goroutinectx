package deriver

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// Matcher provides OR/AND matching for derive function specifications.
// The check passes if ANY group is fully satisfied (OR semantics).
// A group is satisfied if ALL functions in that group are called (AND semantics).
type Matcher struct {
	OrGroups [][]funcspec.Spec
	Original string
}

// NewMatcher creates a Matcher from a derive function string.
// Supports OR (comma) and AND (plus) operators.
// Format: "pkg/path.Func" or "pkg/path.Type.Method". It rejects a
// specification funcspec.Parse rejects.
func NewMatcher(deriveFuncsStr string) (*Matcher, error) {
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

			spec, err := funcspec.Parse(andPart)
			if err != nil {
				return nil, fmt.Errorf("-goroutine-deriver: %w", err)
			}
			andGroup = append(andGroup, spec)
		}

		if len(andGroup) > 0 {
			m.OrGroups = append(m.OrGroups, andGroup)
		}
	}

	return m, nil
}

// IsEmpty returns true if no derive functions are configured. A nil Matcher
// is empty, so a run without -goroutine-deriver can pass nil.
func (m *Matcher) IsEmpty() bool {
	return m == nil || len(m.OrGroups) == 0
}

// MatchesFunc checks if the given function matches ANY spec in ANY OR group.
func (m *Matcher) MatchesFunc(fn *types.Func) bool {
	if m == nil {
		return false
	}
	for _, andGroup := range m.OrGroups {
		for _, spec := range andGroup {
			if spec.Matches(fn) {
				return true
			}
		}
	}
	return false
}
