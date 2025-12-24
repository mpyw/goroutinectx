// Package funcspec provides function specification parsing and matching.
//
// # Overview
//
// This package parses function specifications from flag values and provides
// matching against types.Func objects.
//
// # Specification Format
//
// A function specification has the format:
//
//	pkg/path.FuncName           # Package-level function
//	pkg/path.TypeName.Method    # Method on type
//
// Examples:
//
//	golang.org/x/sync/errgroup.Group.Go
//	github.com/sourcegraph/conc/pool.Pool.Go
//	context.WithCancel
//
// # Spec Structure
//
//	type Spec struct {
//	    PkgPath  string  // Full package path
//	    TypeName string  // Type name (empty for package functions)
//	    FuncName string  // Function or method name
//	}
//
// # Parsing
//
// Use [Parse] to create a Spec from a string:
//
//	spec := funcspec.Parse("github.com/pkg.Type.Method")
//	// spec.PkgPath  = "github.com/pkg"
//	// spec.TypeName = "Type"
//	// spec.FuncName = "Method"
//
//	spec := funcspec.Parse("context.WithCancel")
//	// spec.PkgPath  = "context"
//	// spec.TypeName = ""  (empty - package function)
//	// spec.FuncName = "WithCancel"
//
// # Matching
//
// Use [Spec.Matches] to check if a types.Func matches:
//
//	fn := pass.TypesInfo.ObjectOf(ident).(*types.Func)
//	if spec.Matches(fn) {
//	    // Function matches the specification
//	}
//
// The matching handles:
//   - Package path matching (including version suffixes like /v2)
//   - Type name for methods
//   - Function/method name
//
// # Full Name
//
// Use [Spec.FullName] for display in error messages:
//
//	spec := funcspec.Spec{
//	    PkgPath:  "golang.org/x/sync/errgroup",
//	    TypeName: "Group",
//	    FuncName: "Go",
//	}
//	spec.FullName()  // Returns "errgroup.Group.Go"
//
// # Extracting Function from Call
//
// Use [ExtractFunc] to get the types.Func from a call expression:
//
//	fn := funcspec.ExtractFunc(pass, callExpr)
//	if fn != nil && spec.Matches(fn) {
//	    // Call matches the specification
//	}
//
// This handles various call forms:
//   - Direct calls: pkg.Func()
//   - Method calls: obj.Method()
//   - Interface method calls
package funcspec
