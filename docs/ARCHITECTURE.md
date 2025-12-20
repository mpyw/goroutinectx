# goroutinectx Architecture

This document describes the architecture, design decisions, and technical specifications of goroutinectx.

## Problem Statement

In Go applications, [`context.Context`](https://pkg.go.dev/context#Context) is used for:
- Request cancellation
- Timeout propagation
- Trace/span correlation (APM, distributed tracing)
- Request-scoped values

However, developers often forget to pass context to goroutines, breaking the propagation chain. This leads to:
- Incomplete traces in APM tools
- Uncancellable goroutines (resource leaks)
- Lost request-scoped data
- Goroutines running beyond request lifetime

## Goals

1. **Detect missing context propagation** in goroutines and related patterns
2. **Integration** with existing Go tooling (`go vet`, `golangci-lint`)
3. **Zero false positives** - prefer missing issues over false alarms
4. **Type-safe analysis** - use [`go/types`](https://pkg.go.dev/go/types) for accurate detection

## Non-Goals

1. Auto-fixing (may be added later)
2. Runtime checking
3. Configuration files (flags only for v1)
4. Custom rule definition (future version)

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

Comment directive support with checker-specific ignores and unused detection:

```go
type CheckerName string  // goroutine, goroutinederive, waitgroup, errgroup, spawner, spawnerlabel, gotask

type Entry struct {
    pos      token.Pos
    checkers []CheckerName        // Empty = ignore all
    used     map[CheckerName]bool // Track usage per checker
}

type Map map[int]*Entry  // Line numbers with ignore comments

func Build(fset *token.FileSet, file *ast.File) Map
func (m Map) ShouldIgnore(line int, checker CheckerName) bool  // Checks same line and previous line
func (m Map) GetUnusedIgnores(enabled EnabledCheckers) []UnusedIgnore
```

**Supported formats:**
- `//goroutinectx:ignore` - ignore all checkers
- `//goroutinectx:ignore goroutine` - ignore specific checker
- `//goroutinectx:ignore goroutine,errgroup` - ignore multiple checkers
- `//goroutinectx:ignore - reason` - ignore all with comment
- `//goroutinectx:ignore goroutine - reason` - ignore specific with comment

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

## Key Design Decisions

### 1. `inspector.WithStack` for Nested Function Support

**Decision:** Use [`inspector.WithStack`](https://pkg.go.dev/golang.org/x/tools/go/ast/inspector#Inspector.WithStack) instead of `ast.Inspect`.

**Rationale:** Proper tracking of context through nested functions and closures requires knowing the enclosing function at any point in the AST traversal. `inspector.WithStack` provides this stack context, enabling correct handling of:
- Nested functions at any depth
- Closures capturing context
- Shadowed context parameters
- Context introduced in middle layers

### 2. Type-Safe Analysis

**Decision:** Use [`go/types`](https://pkg.go.dev/go/types) for all type checking instead of name-based string matching.

**Rationale:** Name-based checking (e.g., `if sel.Sel.Name == "Info"`) is error-prone and breaks with:
- Package aliases
- Type embedding
- Interface satisfaction

Type-safe checking via `pass.TypesInfo` ensures accurate detection regardless of naming.

### 3. Interface Segregation

**Decision:** Separate `CallChecker` and `GoStmtChecker` interfaces rather than a single `Checker` interface.

**Rationale:** These are fundamentally different AST constructs:
- `go` is a statement (`*ast.GoStmt`)
- Function calls are expressions (`*ast.CallExpr`)

Separating interfaces allows checkers to implement only what they need, avoiding no-op method implementations.

### 4. Package Structure and Method Design

**Decision:** Split checker implementations into separate packages under `internal/checkers/`.

**Rationale:**
- Package namespace resolves naming conflicts naturally
- Internal names can be simpler (e.g., `Checker` instead of `errgroupChecker`)
- Methods must have meaningful receivers - receiver-less struct methods are a code smell indicating either:
  1. The function should be package-level, or
  2. The struct should hold some state

### 5. Multiple Context Parameter Support

**Decision:** Track ALL context parameters in a function, not just the first one.

**Rationale:** Functions may have multiple context parameters (e.g., `ctx1, ctx2 context.Context`). If any context variable is used, the check passes. Error messages report the first context name for consistency.

### 6. Nested Closure Context Rule

**Decision:** Context usage in nested closures does NOT satisfy the outer goroutine's requirement.

**Rationale:** This ensures every goroutine level explicitly acknowledges context propagation. Without this rule:
- Code readers can't tell if context propagation was intentional
- Refactoring (e.g., removing the inner goroutine) could silently break context propagation

The pattern `_ = ctx` serves as explicit acknowledgment.

### 7. Zero False Positives Philosophy

**Decision:** When uncertain, don't report.

**Rationale:** False positives erode trust in the linter. Users ignore warnings when too many are incorrect. Better to miss some issues than to report incorrectly. Users can always suppress with `//goroutinectx:ignore`.

## Future Considerations

Potential enhancements (not currently planned):
- golangci-lint integration
- More context carrier patterns
- Cross-function context tracking

## References

- [`go/analysis` package](https://pkg.go.dev/golang.org/x/tools/go/analysis)
- [Writing Go Analysis Tools](https://arslan.io/2019/06/13/using-go-analysis-to-write-a-custom-linter/)
- [golangci-lint custom linters](https://golangci-lint.run/contributing/new-linters/)
