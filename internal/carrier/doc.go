// Package carrier provides context carrier type parsing.
//
// # Overview
//
// A "carrier" is a type that carries or wraps context.Context.
// The analyzer treats carrier types like context.Context for
// propagation checking.
//
// # Common Carrier Types
//
// Many web frameworks have their own context types:
//
//	github.com/labstack/echo/v4.Context
//	github.com/gin-gonic/gin.Context
//	github.com/gofiber/fiber/v2.Ctx
//
// # Configuration
//
// Configure carriers via the -context-carriers flag:
//
//	golangci-lint run -- -context-carriers=github.com/labstack/echo/v4.Context
//
// Multiple carriers (comma-separated):
//
//	-context-carriers=github.com/labstack/echo/v4.Context,github.com/gin-gonic/gin.Context
//
// # Carrier Structure
//
//	type Carrier struct {
//	    PkgPath  string  // Package path
//	    TypeName string  // Type name
//	}
//
// # Parsing
//
// Use [Parse] to parse the carrier list:
//
//	carriers, err := carrier.Parse([]string{"github.com/labstack/echo/v4.Context"})
//	// carriers = []Carrier{{
//	//     PkgPath:  "github.com/labstack/echo/v4",
//	//     TypeName: "Context",
//	// }}
//
// # Type Matching
//
// Use [Carrier.Matches] to check if a type matches:
//
//	for _, c := range carriers {
//	    if c.Matches(typ) {
//	        // typ is a carrier type
//	    }
//	}
//
// The matching handles:
//   - Pointer types: *echo.Context matches echo.Context
//   - Version suffixes: echo/v4 matches echo/v4, echo/v5, etc.
//
// # IsCarrierType Helper
//
// Use [IsCarrierType] to check if a type matches any carrier:
//
//	if carrier.IsCarrierType(typ, carriers) {
//	    // typ is one of the carriers
//	}
//
// The checkers do not call it themselves: ctxkind.Classifier holds the
// carriers of a pass and tells a carrier as one kind of context.
//
// # Version Suffix Handling
//
// Package paths with version suffixes are matched flexibly:
//
//	Carrier:  github.com/labstack/echo.Context
//	Matches:  github.com/labstack/echo/v4.Context  // Yes
//	Matches:  github.com/labstack/echo/v5.Context  // Yes
//	Matches:  github.com/labstack/echo.Context     // Yes
package carrier
