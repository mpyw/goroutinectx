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

All modern Go linters use [`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis). Benefits:
- Integration with `go vet`
- Integration with `golangci-lint`
- Standard fact mechanism for cross-package analysis
- Built-in testing via [`analysistest`](https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest)

### 3. Library-First Design

goroutinectx is designed as an importable library:
- `analyzer.go` - Main analyzer definition
- `internal/` - Implementation packages
- No standalone CLI (use with singlechecker or multichecker)

This enables both standalone usage and programmatic integration.

## Directory Structure

```
goroutinectx/
├── analyzer.go                # Analyzer (flags) and New (a Config), both resolved into run.Config
├── plugin/                    # golangci-lint module plugin: settings passed to New
├── analyzer_test.go           # Integration tests using analysistest
├── cmd/goroutinectx/          # The command, its end-to-end tests, and the golden check
├── internal/
│   ├── run/                   # Run: every checker over one package, with a Config;
│   │                          # the checker interfaces and the runner that applies ignores
│   ├── checker/               # Checker names
│   ├── checkerdef/            # The one table of checkers: flags, defaults, builders
│   ├── checkers/              # Checker implementations
│   │   ├── goroutine.go       # go statement checkers (goroutine, goroutinederive)
│   │   ├── spawn.go           # errgroup, waitgroup, conc and spawner checkers
│   │   ├── gotask.go          # gotask library checker
│   │   └── spawnerlabel/      # spawnerlabel checker
│   ├── carrier/               # Context carrier types (-context-carriers)
│   ├── ctxkind/               # Classifier: what counts as a context, built once per pass
│   ├── deriver/               # Matcher for OR/AND deriver matching
│   ├── directive/             # Directive parsing
│   │   ├── ignore/            # //goroutinectx:ignore
│   │   └── spawner/           # //goroutinectx:spawner
│   ├── env/                   # Env: what the checkers read about the pass, built once per pass
│   ├── flow/                  # SSA program, fresh contexts, closure checks,
│   │                          # value resolution, call walker
│   ├── funcspec/              # pkg/path.Func and pkg/path.Type.Method specs
│   ├── registry/              # The one table of supported spawning APIs
│   └── typeutil/              # Pointers, embedded interfaces, package paths
├── testdata/
│   └── src/                   # Test fixtures for analysistest
│       ├── patterns/          # Cases the goroutine, errgroup, waitgroup and spawner checkers share
│       ├── patternsderive/    # The same, with -goroutine-deriver
│       ├── goroutine/         # goroutine-only cases
│       ├── errgroup/          # errgroup-only cases
│       ├── waitgroup/         # waitgroup-only cases
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
├── AGENTS.md                  # Repository instructions
├── design/                    # Implementation notes
└── README.md
```

## Core Components

### analyzer.go

Main entry point. `Analyzer` defines the flags (`-goroutine-deriver`, `-context-carriers`, and a toggle for each checker of `checkerdef.Defs` that has a flag) and resolves them into a `run.Config`. `New` builds an analyzer of its own, with no flags, from a `Config` resolved the same way. `internal/run` does the rest, so the golangci-lint plugin in `plugin/` runs the same code through `New`, without touching the flags. `run.Run`:
1. Builds the ignore maps, spawners and derivers from the `run.Config`, and from them one `env.Env` for the pass
2. Builds each checker that is on from the `env.Env`
3. Maps each function with a context parameter to the name of the first (`Kinds.ParamNames`)
4. Uses `inspector.WithStack` to traverse the AST, and for each go statement and call finds the nearest enclosing function with a context
5. Dispatches to the go statement or call checkers, with the `env.Env` and that context's name
6. Reports each returned diagnostic unless an ignore for that checker sits at its line

### Checker interfaces (internal/run)

Core interfaces:

```go
// Separated interfaces, in internal/run. Each returns its reports, and the
// runner applies //goroutinectx:ignore at each report's position.
type goStmtChecker interface {
    Name() checker.Name
    CheckGoStmt(e *env.Env, ctxName string, stmt *ast.GoStmt) []analysis.Diagnostic
}

type callChecker interface {
    Name() checker.Name
    CheckCall(e *env.Env, ctxName string, call *ast.CallExpr) []analysis.Diagnostic
}

type packageChecker interface {
    Name() checker.Name
    Check(pass *analysis.Pass, skipFiles map[string]bool) []analysis.Diagnostic
}
```

### internal/env/

What the checkers read about the pass, built once per pass by `run.Run`:

```go
type Env struct {
    Pass     *analysis.Pass
    SSA      *flow.Program
    Kinds    *ctxkind.Classifier // what counts as a context: carriers, module
    Derivers *deriver.Matcher    // nil when -goroutine-deriver is not set
    Spawners *spawner.Map
    Flow     *flow.Flow          // follows values, one bound per question, remembered for the pass
}
```

### internal/ctxkind/

Tells what counts as a context, with the carriers, the `context.Context` interface and the module of one pass. `Kind` is a set, since a type can be several at once:

| Kind | A type of this kind is |
|------|------------------------|
| `Context` | `context.Context`, an interface that embeds it, or a type parameter constrained by one |
| `Carrier` | A type of `-context-carriers` |
| `Implements` | A type that implements `context.Context`, as a struct that embeds one does |
| `Holds` | A struct with a field of one of these kinds that the analyzed code can set (#191) |

Each caller names the set it counts. The sets do not agree yet (#210 step 2):

| Threshold | Kinds | Used by |
|-----------|-------|---------|
| `Param` | `Context`, `Carrier` | `HasParam`, `ParamNames`, `Involves`, `MayHold`; the goroutine checker's test of a closure's parameters |
| `Captured` | `Context`, `Carrier`, `Implements` | `Flow.ClosureCapturesContext`: free variables, the parts read from them, the results of a method called on them; `Holds` for its fields |
| `Context` | `Context` | `flow.GivesOnlyFreshContext`, for the fresh argument; `IsWithCall`, for the first parameter |

It also holds both tests for a context parameter. They differ on purpose:

| Method | Reads | Blank parameter (`_ context.Context`) | Receiver | Used by |
|--------|-------|---------------------------------------|----------|---------|
| `HasParam` | SSA parameters | Counts | Counts | The checkers, for spawned closures and factories |
| `ParamNames` | Syntax of a declaration or literal | Left out | Left out | The runner, to find the enclosing context and its name |

`IsFreshCall` tells `Background` and `TODO` of `context` and `golang.org/x/net/context`. `IsWithCall` tells the `With*` functions of `context`, which the gotask checker follows back to a deriver.

### internal/flow/

Follows values through the package's SSA form: the instruction of a go statement or a call (`Program.GoAt`, `Program.CallAt`), what a closure captures (`ClosureCapturesContext`), what it runs (`ClosureCallsDeriver`, `Runs`), what kind of function it is (`Classify`), and where a spawned function comes from (`FuncResolver`). It wraps `golang.org/x/tools/go/ssa`, which is imported as `ssa`.

`Classify` is the one place that tells a function literal, a range-over-func body, a named function, a method, and the synthetic wrappers (bound method, thunk, promoted method) apart. It classifies an instantiation as its generic origin. A function or method of another package has no syntax, so it is of the external kind. The rule for a named function given only a fresh context takes an external function with no receiver too (#223). `Runs` is the one walk of what a function runs. Its options decide whether it goes into goroutines and into the closures a called factory returns:

| Caller | Goroutines | Returned closures |
|--------|------------|-------------------|
| Deriver check (`ClosureCallsDeriver`) | No | No |
| spawnerlabel | Yes | Yes |

The returns of a function are read in one place, in two ways. A factory is followed through the first result of each return only, as `Flow.Returns` gives it. spawnerlabel takes a closure returned as any result.

`Flow` is the one engine that follows values, built once per pass and held by `env.Env`. `Flow.Sources(v)` gives the values `v` can come from, each with its kind (function literal, named function, method, call result, parameter, global, unknown), and whether the list is complete. It goes through Phi nodes, type changes, loads of local variables and of their parts, and map lookups at constant keys. It never looks through a call. Two other questions read what it does not:

| Question | Gives | For |
|----------|-------|-----|
| `Flow.CapturedSources(v)` | What a variable a closure captures, or a part of a captured value, can hold: every value stored to it, as the closure may run after any store | The closure checks |
| `Flow.Returns(fn)` | Each value `fn` returns as its first result, with its sources | `FuncResolver`, `ResolvedSliceElems` |

Every check that follows a value is a fold over these:

| Check | Fold |
|-------|------|
| `FuncResolver.Passes` | Every source passes: a literal by `Closure`, a call by the factory rules. Of a factory, one of the values `Returns` gives passing is enough |
| `FuncResolver.CapturedPasses` | `Passes` of each value `CapturedSources` gives, or true when they cannot all be followed |
| `Flow.IsFreshContext` | Every source is a `Background()` or `TODO()` call, and the list is complete |
| `Flow.SignalsContext` | A source is `Done()` of a context, or the cancel func of a `context.With*` call, and that context is not fresh |
| `Flow.ClosureCapturesContext` | A captured context, or one read from a part of a captured value, counts unless `IsFreshContext` of each value `CapturedSources` gives. A captured func variable counts when one of them is a closure that captures a context. A func value the closure calls, read from what it captures, counts when `FuncResolver.CapturedPasses` passes it, as if it were spawned directly |
| `Flow.ResolvedSliceElems` | The elements of a slice literal, or of each slice a factory `Returns` |
| gotask `ctxIsDerived`, `taskDerives` | Every source is a deriver call, a `With*` of one, or a task that derives |

**One bound per question.** A question to `Flow` (`Sources`, `CapturedSources`, `Returns`, `IsFreshContext`, `ClosureCapturesContext`, `FuncResolver.Passes`, ...) may follow 4096 values, parts, loads and lookups in all, and the questions it asks in turn share that bound: a resolver judging a closure that asks whether a context is fresh does not start a new one. So nested checks do not multiply the work (#169). What a question finds is remembered for the rest of the pass, unless a bound cut it short, so a value asked about again costs nothing.

It also tells whether a call gives only a fresh context (`GivesOnlyFreshContext`).

### internal/directive/

`directive.Scan` reads every directive of a file in one pass over its comments. The runner and `spawner.Analyzer` both use it. The runner skips a generated file whole, and reads the rest once each.

```go
type Found struct {
    Ignores  []Ignore         // Each //goroutinectx:ignore, its checker names unchecked
    Spawners []*ast.FuncDecl  // Each function whose doc comment holds //goroutinectx:spawner
    Problems []Problem        // Each comment addressed to goroutinectx that does nothing
}

func Scan(file *ast.File) Found
```

### internal/directive/ignore/

Checker-specific ignores and unused detection:

```go
type Entry struct {
    pos      token.Pos
    trailing bool                  // The comment follows code on its line
    checkers []checker.Name        // Empty = ignore all
    used     map[checker.Name]bool // Track usage per checker
}

type Map map[int]*Entry  // Line numbers with ignore comments

func Build(fset *token.FileSet, file *ast.File, ignores []directive.Ignore) (Map, []directive.Problem)  // Reports an unknown checker name
func (m Map) Covering(line int, name checker.Name) *Entry  // Same line, and previous line unless that ignore trails code. Marks nothing
func (e *Entry) MarkUsed(name checker.Name)                // The runner marks the ignore that silenced a report
func (m Map) GetUnusedIgnores(enabled EnabledCheckers) []UnusedIgnore
```

**Supported formats:**
- `//goroutinectx:ignore` - ignore all checkers
- `//goroutinectx:ignore goroutine` - ignore specific checker
- `//goroutinectx:ignore goroutine,errgroup` - ignore multiple checkers
- `//goroutinectx:ignore // reason` - ignore all with comment
- `//goroutinectx:ignore goroutine // reason` - ignore specific with comment
- `//goroutinectx:ignore goroutine - reason` - the same, with ` - ` kept for compatibility
- An unknown checker name is reported, and the directive ignores nothing

Only `//goroutinectx:name` (a line comment, a lowercase name, no spaces) is a
directive, as parsed by `go/ast.ParseDirective`. Any other comment starting
with `goroutinectx:` is reported as a malformed directive.

### internal/deriver/

Deriver function matching with OR/AND logic:

```go
type Matcher struct {
    OrGroups [][]funcspec.Spec // Each group must have ALL specs satisfied
    Original string            // Original flag value for error messages
}

func NewMatcher(deriveFuncsStr string) (*Matcher, error)
func (m *Matcher) IsEmpty() bool
func (m *Matcher) MatchesFunc(fn *types.Func) bool
```

Whether a goroutine calls them is judged by `flow.ClosureCallsDeriver`.

## Checker Implementations

| Checker | File | Interface | Checks |
|---------|------|-----------|--------|
| goroutine | internal/checkers/goroutine.go | goStmtChecker | Context in `go func()` |
| goroutinederive | internal/checkers/goroutine.go | goStmtChecker | Specific function call in `go func()` |
| errgroup | internal/checkers/spawn.go | callChecker | Context in `g.Go()` |
| waitgroup | internal/checkers/spawn.go | callChecker | Context in `wg.Go()` |
| conc | internal/checkers/spawn.go | callChecker | Context in sourcegraph/conc pool callbacks |
| spawner | internal/checkers/spawn.go | callChecker | Context in `//goroutinectx:spawner` marked function calls |
| gotask | internal/checkers/gotask.go | callChecker | Deriver in gotask task functions |
| spawnerlabel | internal/checkers/spawnerlabel/ | packageChecker | `//goroutinectx:spawner` missing or unneeded |

## Analysis Flow

```go
insp.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
    ctxName, ok := runEnclosingContext(ctxNames, stack)
    if !ok {
        return true // No context in scope
    }

    switch node := n.(type) {
    case *ast.GoStmt:
        for _, c := range r.goStmtCheckers {
            r.report(c.Name(), c.CheckGoStmt(r.env, ctxName, node), token.NoPos)
        }
    case *ast.CallExpr:
        for _, c := range r.callCheckers {
            r.report(c.Name(), c.CheckCall(r.env, ctxName, node), node.Pos())
        }
    }
    return true
})
```

## Goroutine Checker Details

### Context Usage Detection

The goroutine checker detects whether a goroutine's function captures a context, from its SSA free variables:

```go
func (f *Flow) ClosureCapturesContext(closure *ssa.Function, kinds *ctxkind.Classifier) bool // in internal/flow
```

**Key behaviors:**
- Uses SSA values, not name matching (handles shadowing correctly)
- Checks ALL context parameters, not just the first one
- Context usage in nested closures counts: a closure captures what its inner closures capture
- A context read from a field or an element of a captured value counts, unless it is only ever fresh; a captured value that implements `context.Context`, or a method called on a captured value that returns a context or holds one the analyzed module can set (an exported field, or a field of a type in the module, or outside the standard library when the module is not known), counts too

### Higher-Order Function Support

For patterns like `g.Go(fn)`, `g.Go(s.fn)` or `g.Go(fns["a"])`, the SSA value of the argument is followed back to each function literal it can hold, by `flow.FuncResolver`:

```go
r := &flow.FuncResolver{Flow: e.Flow, Closure: func(fn *ssa.Function) bool { ... }}
ok := r.Passes(value)
```

It takes the sources of the value from `Flow.Sources`, which goes through Phi nodes, loads of local variables and of their struct fields and constant-index elements, and map lookups at constant keys, and follows calls of factories through what `Flow.Returns` gives. Only the stores that reach the spawn count. A value it cannot follow passes, and so does a parameter, unless it is of a factory followed from a call that gives it no func that passes.

### Deriver Matching

When `-goroutine-deriver` is set, goroutines must call the specified function(s):

- **OR (comma)**: `pkg1.Func1,pkg2.Func2` - at least one must be called
- **AND (plus)**: `pkg1.Func1+pkg2.Func2` - all must be called
- **Mixed**: `pkg1.A+pkg1.B,pkg2.C` - (A AND B) OR C

## Testing

### analysistest

All checker tests use [`analysistest`](https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest):

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

A type is matched against `context.Context` or a carrier only after `typeutil.UnwrapPointer`, which strips pointers and aliases. An alias such as `type Ctx = context.Context`, or `golang.org/x/net/context.Context`, is a `*types.Alias`, not the `*types.Named` it names (#181). An interface is matched by the types it embeds, named or not, so `type MyCtx interface{ context.Context }` is a context (#194). A type parameter is matched by its constraint in the same way (`typeutil.Embeds`, #186). Only embedded types are looked into: an interface that declares the methods of `context.Context` itself is not a context, and a struct that embeds one is not matched here.

### 3. Interface Segregation

**Decision:** Separate `CallChecker` and `GoStmtChecker` interfaces rather than a single `Checker` interface.

**Rationale:** These are fundamentally different AST constructs:
- `go` is a statement (`*ast.GoStmt`)
- Function calls are expressions (`*ast.CallExpr`)

Separating interfaces allows checkers to implement only what they need, avoiding no-op method implementations.

### 4. Package Structure

**Decision:** Keep the go statement and call checkers in one package, `internal/checkers`, one file per kind (`goroutine.go`, `spawn.go`, `gotask.go`). Only spawnerlabel, a package checker that shares nothing with them, has a package of its own.

**Rationale:** The checkers judge spawned functions with the same helpers (`spawned.go`): how a callee and its arguments are found, and how a closure is resolved. Split into packages, each would need those exported from a shared package. In one package, declscope keeps each file's helpers to that file, and the names carry the file (`gotaskIsConstructor`, `spawnedCallee`).

### 5. Multiple Context Parameter Support

**Decision:** Track ALL context parameters in a function, not just the first one.

**Rationale:** Functions may have multiple context parameters (e.g., `ctx1, ctx2 context.Context`). If any context variable is used, the check passes. Error messages report the first context name for consistency.

### 6. Nested Closure Context Rule

**Decision:** Context usage in nested closures satisfies the outer goroutine's requirement.

**Rationale:** The check reads the SSA free variables of the spawned closure. A closure gets a free variable for each one its inner closures capture, so a context used in a deferred or recovery closure, or in an inner goroutine, is captured by the outer goroutine too. That goroutine does carry the context, so reporting it would be a false positive (see 7). Each `go` statement and spawn call is still checked on its own, so an inner goroutine that does not use the context is reported.

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
