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

// NewMatcher creates a Matcher from the OR groups of -goroutine-deriver.
// Each group names one function, or several joined by + that must all be
// called (AND). A function is "pkg/path.Func" or "pkg/path.Type.Method". It
// rejects an empty group, an empty function of a group, and a specification
// funcspec.Parse rejects. Original joins the groups with commas, as the
// flag spells them.
func NewMatcher(groups []string) (*Matcher, error) {
	m := &Matcher{
		Original: strings.Join(groups, ","),
	}

	for i, group := range groups {
		if strings.TrimSpace(group) == "" {
			return nil, fmt.Errorf("-goroutine-deriver: group %d is empty", i+1)
		}

		// Split by plus (AND within group)
		var andGroup []funcspec.Spec
		for andPart := range strings.SplitSeq(group, "+") {
			andPart = strings.TrimSpace(andPart)
			if andPart == "" {
				return nil, fmt.Errorf("-goroutine-deriver: %q: a function joined by + is empty", group)
			}

			spec, err := funcspec.Parse(andPart)
			if err != nil {
				return nil, fmt.Errorf("-goroutine-deriver: %w", err)
			}
			andGroup = append(andGroup, spec)
		}
		m.OrGroups = append(m.OrGroups, andGroup)
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
