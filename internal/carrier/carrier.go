package carrier

import (
	"fmt"
	"go/types"
	"slices"
	"strings"

	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Carrier represents a type that can carry context.
// Format: "pkg/path.TypeName" (e.g., "github.com/labstack/echo/v4.Context").
type Carrier struct {
	PkgPath  string
	TypeName string
}

// Matches checks if the given type matches this carrier, through pointers
// and aliases. An interface that embeds the carrier, named or not, and a
// type parameter whose constraint is or embeds it match too.
func (c Carrier) Matches(t types.Type) bool {
	return typeutil.Embeds(t, c.matchesNamed)
}

func (c Carrier) matchesNamed(t types.Type) bool {
	named, ok := typeutil.UnwrapPointer(t).(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	return typeutil.PkgPathMatches(obj.Pkg().Path(), c.PkgPath) && obj.Name() == c.TypeName
}

// IsCarrierType checks if the type matches any of the carriers.
func IsCarrierType(t types.Type, carriers []Carrier) bool {
	return slices.ContainsFunc(carriers, func(c Carrier) bool { return c.Matches(t) })
}

// Parse parses a list of context carriers, each pkg/path.Type. A blank
// entry and an entry that holds a comma are errors. So is an entry with no
// type, or whose package is a bare name that is not a standard library
// package: it would never match.
func Parse(parts []string) ([]Carrier, error) {
	var carriers []Carrier

	for i, part := range parts {
		if err := typeutil.CheckItem("-context-carriers", i, part, "type"); err != nil {
			return nil, err
		}
		part = strings.TrimSpace(part)

		lastDot := strings.LastIndex(part, ".")
		if lastDot <= 0 || lastDot == len(part)-1 {
			return nil, fmt.Errorf("-context-carriers: %q: want pkg/path.Type", part)
		}
		c := Carrier{PkgPath: part[:lastDot], TypeName: part[lastDot+1:]}
		if err := typeutil.CheckPkgPath(part, c.PkgPath); err != nil {
			return nil, fmt.Errorf("-context-carriers: %w", err)
		}
		carriers = append(carriers, c)
	}

	return carriers, nil
}
