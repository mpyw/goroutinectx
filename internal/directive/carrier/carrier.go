// Package carrier provides context carrier type parsing.
package carrier

import (
	"go/types"
	"strings"

	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Carrier represents a type that can carry context.
// Format: "pkg/path.TypeName" (e.g., "github.com/labstack/echo/v4.Context").
type Carrier struct {
	PkgPath  string
	TypeName string
}

// Matches checks if the given type matches this carrier.
func (c Carrier) Matches(t types.Type) bool {
	t = typeutil.UnwrapPointer(t)

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	return matchPkg(obj.Pkg().Path(), c.PkgPath) && obj.Name() == c.TypeName
}

// matchPkg checks if pkgPath matches targetPkg, allowing version suffixes.
func matchPkg(pkgPath, targetPkg string) bool {
	if pkgPath == targetPkg {
		return true
	}
	// Check for version suffix like /v2, /v3, etc.
	prefix := targetPkg + "/v"
	if !strings.HasPrefix(pkgPath, prefix) {
		return false
	}
	rest := pkgPath[len(prefix):]
	return len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9'
}

// IsCarrierType checks if the type matches any of the carriers.
func IsCarrierType(t types.Type, carriers []Carrier) bool {
	for _, c := range carriers {
		if c.Matches(t) {
			return true
		}
	}
	return false
}

// Parse parses a comma-separated list of context carriers.
func Parse(s string) []Carrier {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	carriers := make([]Carrier, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		lastDot := strings.LastIndex(part, ".")
		if lastDot == -1 {
			continue // Invalid format
		}

		carriers = append(carriers, Carrier{
			PkgPath:  part[:lastDot],
			TypeName: part[lastDot+1:],
		})
	}

	return carriers
}
