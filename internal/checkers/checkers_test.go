package checkers

import (
	"slices"
	"testing"

	"github.com/mpyw/goroutinectx/internal/checkername"
)

// TestDefsFollowAll checks that Defs defines each checker of
// checkername.All once, in its order.
func TestDefsFollowAll(t *testing.T) {
	t.Parallel()

	names := make([]checkername.Name, len(Defs))
	for i, d := range Defs {
		names[i] = d.Name
	}
	if !slices.Equal(names, checkername.All) {
		t.Errorf("Defs names = %v, want checkername.All = %v", names, checkername.All)
	}
}
