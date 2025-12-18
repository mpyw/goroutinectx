# goroutinectx Architecture

## Design Principles

Based on analysis of successful Go linters (staticcheck, errcheck, contextcheck, nilaway, bodyclose, ineffassign), goroutinectx follows these design principles:

### 1. Single-Purpose Focus

Like `errcheck` (error handling only) and `bodyclose` (res.Body.Close only), goroutinectx focuses exclusively on **goroutine context propagation**. We don't try to be a general-purpose linter.

### 2. go/analysis Framework

All modern Go linters use `golang.org/x/tools/go/analysis`. Benefits:
- Integration with `go vet`
- Integration with `golangci-lint`
- Standard fact mechanism for cross-package analysis
- Built-in testing via `analysistest`

### 3. Library-First Design

goroutinectx is designed as an importable library:
- `analyzer.go` - Main analyzer definition
- `internal/` - Implementation packages
- No standalone CLI (use with singlechecker or multichecker)

This enables both standalone usage and programmatic integration.

## Directory Structure

```
goroutinectx/
├── analyzer.go                # Main analyzer (orchestration, flags)
├── analyzer_test.go           # Integration tests using analysistest
├── waitgroup_test.go          # Waitgroup tests (Go 1.25+ build tag)
├── internal/
│   ├── checkers/              # Checker implementations
│   │   ├── checker.go         # CallChecker, GoStmtChecker interfaces
│   │   ├── errgroup/          # errgroup.Group.Go() checker
│   │   ├── waitgroup/         # sync.WaitGroup.Go() checker
│   │   ├── goroutine/         # go statement checker
│   │   ├── spawner/           # spawner directive checker
│   │   ├── goroutinederive/   # goroutine_derive checker
│   │   └── gotask/            # gotask library checker
│   ├── context/               # Context scope detection
│   │   └── scope.go           # ContextScope, FindScope
│   ├── directives/            # Directive parsing
│   │   ├── ignore/            # //goroutinectx:ignore
│   │   ├── spawner/           # //goroutinectx:spawner
│   │   ├── carrier/           # Context carrier types
│   │   └── deriver/           # DeriveMatcher for OR/AND deriver matching
│   └── typeutil/              # Type checking utilities
├── testdata/
│   ├── metatest/              # Test metadata validation
│   │   ├── structure.json     # Test definitions
│   │   ├── structure.schema.json  # JSON Schema
│   │   └── validation_test.go # Metadata validator
│   └── src/                   # Test fixtures for analysistest
│       ├── goroutine/         # goroutine checker tests
│       ├── errgroup/          # errgroup checker tests
│       ├── waitgroup/         # waitgroup checker tests
│       ├── spawner/           # spawner tests
│       ├── goroutinederive/   # Single deriver tests
│       ├── goroutinederiveand/    # AND deriver tests
│       ├── goroutinederivemixed/  # Mixed AND/OR tests
│       ├── gotask/            # gotask checker tests
│       └── carrier/           # Context carrier tests
├── docs/
│   ├── ARCHITECTURE.md        # Technical specification (this file)
│   ├── DESIGN.md              # Design decisions
│   └── TUTORIAL.md            # Learning guide
├── .github/workflows/         # CI configuration
├── .golangci.yaml             # golangci-lint configuration
├── CLAUDE.md                  # AI assistant guidance
└── README.md
```

## Core Components

### analyzer.go

Main entry point. Responsibilities:
1. Define flags (`-goroutine-deriver`, `-context-carriers`, checker toggles)
2. Use `inspector.WithStack` to traverse AST with stack context
3. Build `funcScopes` map (function node -> ContextScope)
4. For each node, find nearest enclosing function with context
5. Dispatch to appropriate checkers (CallChecker or GoStmtChecker)

### internal/checkers/checker.go

Core interfaces:

```go
// Separated interfaces (Interface Segregation Principle)
type CallChecker interface {
    CheckCall(cctx *context.CheckContext, call *ast.CallExpr)
}

type GoStmtChecker interface {
    CheckGoStmt(cctx *context.CheckContext, stmt *ast.GoStmt)
}
```

### internal/context/scope.go

Context scope detection:

```go
// Scope tracks context variable(s) in a function
type Scope struct {
    Vars []*types.Var  // All context variables
    Name string        // First variable name (for error messages)
}

// CheckContext holds runtime context for checks
type CheckContext struct {
    Pass      *analysis.Pass
    Scope     *Scope
    IgnoreMap ignore.Map
    Carriers  []carrier.Carrier
}

func FindScope(pass *analysis.Pass, fnType *ast.FuncType, carriers []carrier.Carrier) *Scope
```

### internal/directives/ignore/

Comment directive support:

```go
type Map map[int]struct{}  // Line numbers with ignore comments

func Build(fset *token.FileSet, file *ast.File) Map
func (m Map) ShouldIgnore(line int) bool  // Checks same line and previous line
```

### internal/directives/deriver/

Deriver function matching with OR/AND logic:

```go
type DeriveMatcher struct {
    OrGroups [][]DeriveFuncSpec  // Each group must have ALL specs satisfied
    Original string               // Original flag value for error messages
}

func Parse(s string) *DeriveMatcher
func (m *DeriveMatcher) SatisfiesAnyGroup(pass *analysis.Pass, node ast.Node) bool
```

## Checker Implementations

| Checker | Package | Interface | Checks |
|---------|---------|-----------|--------|
| errgroup | internal/checkers/errgroup | CallChecker | Context in `g.Go()` |
| waitgroup | internal/checkers/waitgroup | CallChecker | Context in `wg.Go()` |
| goroutine | internal/checkers/goroutine | GoStmtChecker | Context in `go func()` |
| spawner | internal/checkers/spawner | CallChecker | Context in `//goroutinectx:spawner` marked function calls |
| goroutinederive | internal/checkers/goroutinederive | GoStmtChecker | Specific function call in `go func()` |
| gotask | internal/checkers/gotask | CallChecker | Deriver in gotask task functions |

## Analysis Flow

```go
insp.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
    scope := findEnclosingScope(funcScopes, stack)
    if scope == nil {
        return true  // No context in scope
    }

    switch node := n.(type) {
    case *ast.GoStmt:
        for _, checker := range goStmtCheckers {
            checker.CheckGoStmt(cctx, node)
        }
    case *ast.CallExpr:
        for _, checker := range callCheckers {
            checker.CheckCall(cctx, node)
        }
    }
    return true
})
```

## Goroutine Checker Details

### Context Usage Detection

The goroutine checker detects whether a context variable is used within a goroutine's function body:

```go
func UsesContext(pass *analysis.Pass, body *ast.BlockStmt, contextVar *types.Var) bool
```

**Key behaviors:**
- Uses type identity, not name matching (handles shadowing correctly)
- Checks ALL context parameters, not just the first one
- Context usage in nested closures does NOT count (intentional design)

### Higher-Order Function Support

For patterns like `g.Go(fn)` where `fn` is a variable:

```go
func FindFuncLitAssignment(pass *analysis.Pass, ident *ast.Ident) *ast.FuncLit
```

Traces variable assignments to find the original function literal.

### Deriver Matching

When `-goroutine-deriver` is set, goroutines must call the specified function(s):

- **OR (comma)**: `pkg1.Func1,pkg2.Func2` - at least one must be called
- **AND (plus)**: `pkg1.Func1+pkg2.Func2` - all must be called
- **Mixed**: `pkg1.A+pkg1.B,pkg2.C` - (A AND B) OR C

## Testing

### analysistest

All checker tests use `golang.org/x/tools/go/analysis/analysistest`:

```go
func TestGoroutine(t *testing.T) {
    testdata := analysistest.TestData()
    analysistest.Run(t, testdata, goroutinectx.Analyzer, "goroutine")
}
```

Test fixtures use `// want` comments:
```go
func bad(ctx context.Context) {
    go func() {  // want "goroutine does not use context"
        doSomething()
    }()
}
```

### Test Metadata (structure.json)

Tests are documented in `testdata/metatest/structure.json`:
- JSON Schema validation ensures structure consistency
- `validation_test.go` verifies function comments match metadata
- Supports good/bad/limitation/notChecked variants

### Build Tags

Waitgroup tests require Go 1.25+ (`sync.WaitGroup.Go()` was added in Go 1.25):
- `waitgroup_test.go` has `//go:build go1.25` tag
- `testdata/src/waitgroup/*.go` files have build tags
- CI runs tests on both Go 1.24 and 1.25

## Comparison with Related Tools

| Aspect | contextcheck | goroutinectx |
|--------|-------------|--------------|
| Focus | `Background()`/`TODO()` detection | Goroutine context propagation |
| Granularity | Function-level | Statement-level |
| Custom types | Limited | `-context-carriers` flag |
| Goroutine awareness | No | Yes (primary focus) |

## Error Philosophy

- **False positives are worse than false negatives**
- Users can suppress with `//goroutinectx:ignore`
- When in doubt, don't report

## Future Considerations

Potential enhancements (not currently planned):
- golangci-lint integration
- More context carrier patterns
- Cross-function context tracking
