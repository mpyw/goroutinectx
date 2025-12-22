# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**goroutinectx** is a Go linter that enforces context propagation best practices in goroutines. It detects cases where a `context.Context` is available in function parameters but not properly passed to goroutines and related APIs.

### Supported Checkers

- **goroutine**: Detect `go func()` that doesn't capture/use context
- **errgroup**: Detect `errgroup.Group.Go()` closures without context
- **waitgroup**: Detect `sync.WaitGroup.Go()` closures without context (Go 1.25+)
- **conc**: Detect `github.com/sourcegraph/conc` pool closures without context
- **spawner**: Detect calls to spawner functions that pass closures without context
  - Directive: `//goroutinectx:spawner` marks local functions
  - Flag: `-external-spawner=pkg/path.Func` or `-external-spawner=pkg/path.Type.Method` for external functions
- **goroutine-derive**: Detect goroutines that don't call a specified context-derivation function (e.g., `apm.NewGoroutineContext`)
  - Activated via flag: `-goroutine-deriver=pkg/path.Func` or `-goroutine-deriver=pkg/path.Type.Method`
  - OR (comma): `-goroutine-deriver=pkg1.Func1,pkg2.Func2` - at least one must be called
  - AND (plus): `-goroutine-deriver=pkg1.Func1+pkg2.Func2` - all must be called
  - Mixed: `-goroutine-deriver=pkg1.Func1+pkg2.Func2,pkg3.Func3` - (Func1 AND Func2) OR Func3
- **gotask**: Detect `github.com/siketyan/gotask` task functions without context derivation (requires `-goroutine-deriver`)
  - `Do*` functions: checks that task arguments call the deriver
  - `Task.DoAsync` / `CancelableTask.DoAsync`: checks that ctx argument is derived

### Directives

- `//goroutinectx:ignore` - Suppress warnings for the next line or same line
  - Checker-specific: `//goroutinectx:ignore goroutine` or `//goroutinectx:ignore goroutine,errgroup`
  - Valid checker names: `goroutine`, `goroutinederive`, `waitgroup`, `errgroup`, `spawner`, `spawnerlabel`, `gotask`
  - Unused ignore detection: reports unused ignore directives
- `//goroutinectx:spawner` - Mark a function as spawning goroutines with its func arguments

## Architecture

```
goroutinectx/
├── analyzer.go                # Main analyzer (orchestration, flags, run function)
├── analyzer_test.go           # Integration tests using analysistest
├── waitgroup_test.go          # Waitgroup-specific tests (Go 1.25+ build tag)
├── internal/
│   ├── checkers/              # Individual checker implementations
│   │   ├── checker.go         # CallChecker, GoStmtChecker interfaces
│   │   ├── errgroup/          # errgroup.Group.Go() checker
│   │   ├── waitgroup/         # sync.WaitGroup.Go() checker (Go 1.25+)
│   │   ├── goroutine/         # go statement checker
│   │   ├── spawner/           # spawner directive checker
│   │   ├── goroutinederive/   # goroutine_derive checker
│   │   └── gotask/            # gotask library checker
│   ├── context/               # Context scope detection
│   ├── directives/            # Directive parsing
│   │   ├── ignore/            # //goroutinectx:ignore
│   │   ├── spawner/           # //goroutinectx:spawner
│   │   ├── carrier/           # Context carrier types
│   │   └── deriver/           # DeriveMatcher for OR/AND deriver matching
│   └── typeutil/              # Type checking utilities
├── testdata/
│   ├── metatest/              # Test metadata validation (structure.json)
│   └── src/                   # Test fixtures
│       ├── goroutine/         # goroutine checker tests
│       ├── errgroup/          # errgroup checker tests
│       ├── waitgroup/         # waitgroup checker tests
│       ├── spawner/           # spawner directive tests
│       ├── goroutinederive/   # Single deriver tests
│       ├── goroutinederiveand/    # AND (all must be called) tests
│       ├── goroutinederivemixed/  # Mixed AND/OR tests
│       ├── gotask/            # gotask checker tests
│       └── carrier/           # Context carrier tests
├── docs/                      # Documentation
├── .github/workflows/         # CI configuration
├── .golangci.yaml             # golangci-lint configuration
└── README.md
```

### Key Design Decisions

1. **Type-safe analysis**: Uses `go/types` for accurate detection (not just name-based)
2. **Nested function support**: Uses `inspector.WithStack` to track context through closures
3. **Shadowing support**: `UsesContext` uses type identity, not name matching
4. **Interface segregation**: `CallChecker` and `GoStmtChecker` interfaces (no BaseChecker)
5. **Minimal exports**: Only necessary types/functions are exported from `checkers` package
6. **Zero false positives**: Prefer missing issues over false alarms
7. **Multiple context tracking**: Tracks ALL context parameters, not just the first one. If ANY context variable is used, the check passes. Error messages report the first context name for consistency.

### Checker Interface Design

```go
// Separated interfaces - checkers implement only what they need
type CallChecker interface { CheckCall(cctx *CheckContext, call *ast.CallExpr) }
type GoStmtChecker interface { CheckGoStmt(cctx *CheckContext, stmt *ast.GoStmt) }

type Checkers struct {
    Call   []CallChecker   // errgroup, waitgroup, spawner, gotask
    GoStmt []GoStmtChecker // goroutine, goroutine_derive
}
```

**Why two interfaces?**
- `GoStmtChecker`: For `go` keyword statements (`go func() {}()`)
- `CallChecker`: For function calls (`g.Go()`, `wg.Go()`)

These are AST-level distinctions: `go` is a statement, function calls are expressions.

### Goroutine-Related Checkers

Four checkers handle goroutine context propagation:

| Checker | Target | AST Node | Higher-Order Support |
|---------|--------|----------|---------------------|
| goroutine | `go func(){}()` | `*ast.GoStmt` | Yes (`go fn()()`) |
| errgroup | `g.Go(func(){})` | `*ast.CallExpr` | Yes (`g.Go(fn)`, `g.Go(make())`) |
| waitgroup | `wg.Go(func(){})` | `*ast.CallExpr` | Yes (`wg.Go(fn)`, `wg.Go(make())`) |
| spawner | `//goroutinectx:spawner` marked funcs | `*ast.CallExpr` | Yes (func args checked) |

**Supported patterns:**
- Literal: `g.Go(func() { ... })`
- Variable: `g.Go(fn)` where `fn := func() { ... }`
- Call result: `g.Go(makeWorker())` where `makeWorker` returns a func
- Call with ctx: `g.Go(makeWorker(ctx))` - ctx passed to factory
- Directive: Functions marked with `//goroutinectx:spawner` check their func arguments

**spawner Directive:**
```go
//goroutinectx:spawner
func runWithGroup(g *errgroup.Group, fn func() error) {
    g.Go(fn)  // fn is spawned as goroutine
}

func caller(ctx context.Context) {
    runWithGroup(g, func() error {
        // Warning: should use ctx
        return nil
    })
}
```

**Known Limitations:**
- Channel receives - can't trace func from channel
- Nested closure ctx (e.g., `defer func() { _ = ctx }()`) - intentionally not counted
- `interface{}` type assertion - can't trace func through type assertion

### Derive Function Matching (DeriveMatcher)

`internal/directives/deriver/` provides shared OR/AND logic for matching derive functions. Used by:
- `goroutinederive`: checks `go func()` calls
- `gotask`: checks gotask task functions and DoAsync calls

```go
// DeriveMatcher supports OR (comma) and AND (plus) operators
type DeriveMatcher struct {
    OrGroups [][]DeriveFuncSpec  // Each group must have ALL specs satisfied
    Original string               // Original flag value for error messages
}

// SatisfiesAnyGroup checks if ANY OR group is fully satisfied
func (m *DeriveMatcher) SatisfiesAnyGroup(pass *analysis.Pass, node ast.Node) bool
```

**Parsing Logic:**
- `pkg.Func1,pkg.Func2` → OR: either Func1 or Func2
- `pkg.Func1+pkg.Func2` → AND: both Func1 and Func2
- `pkg.A+pkg.B,pkg.C` → Mixed: (A AND B) OR C

### gotask Checker

The gotask checker handles `github.com/siketyan/gotask` library:

| Target | Check |
|--------|-------|
| `gotask.Do*` functions | Task arguments (2nd+) must call deriver in their body |
| `Task.DoAsync` | Context argument (1st) must be derived |
| `CancelableTask.DoAsync` | Context argument (1st) must be derived |

**Key insight:** Since gotask tasks run as goroutines, they need to call the deriver function inside their body - there's no way to wrap the context at the call site.

**Known Limitations:**
- Variable references can't be traced (e.g., `task := NewTask(fn); DoAll(ctx, task)`)
- Nested function literals aren't traversed (e.g., deriver in `defer func(){}()` inside task)
- Higher-order function returns can't be traced (e.g., `DoAll(ctx, makeTask())`)

## Development Commands

```bash
# Run ALL tests (ALWAYS use this before committing)
./test_all.sh

# Run golangci-lint
golangci-lint run ./...

# Format code
go fmt ./...
```

> [!IMPORTANT]
> Always use `./test_all.sh` before committing. This script runs:
> 1. JSON schema validation for `structure.json`
> 2. Test metadata validation (ensures all test functions are in `structure.json`)
> 3. All analyzer tests
>
> Running only `go test ./...` will miss structure validation failures.

## Adding a New Checker

1. Create `internal/checkers/<name>/checker.go`:
```go
package myname

import (
    "go/ast"
    "github.com/mpyw/goroutinectx/internal/context"
)

type Checker struct{}

func New() *Checker { return &Checker{} }

// Implement CallChecker for call expression checks
func (c *Checker) CheckCall(cctx *context.CheckContext, call *ast.CallExpr) {
    // Implementation using cctx.Pass, cctx.Scope, cctx.IgnoreMap
}

// OR implement GoStmtChecker for go statement checks
func (c *Checker) CheckGoStmt(cctx *context.CheckContext, stmt *ast.GoStmt) {
    // Implementation
}
```

2. Register in `analyzer.go` under `runASTChecks()`

3. Add test fixtures in `testdata/src/<name>/`

4. Add test case in `analyzer_test.go`

5. Add test metadata in `testdata/metatest/structure.json`

## Testing Strategy

- Use `analysistest` for all analyzer tests
- Test fixtures use `// want` comments for expected diagnostics
- Test metadata is managed in `testdata/metatest/structure.json` (JSON-based)
- Test structure per checker:
  - `basic.go` - Simple good/bad cases
  - `advanced.go` - Complex patterns (defer, loops, channels)
  - `evil.go` - Adversarial patterns (nesting, IIFE, limitations)

## Code Style

- Follow standard Go conventions
- Use `go/analysis` framework
- Prefer `inspector.WithStack` over `ast.Inspect` for traversal
- Type utilities go in `internal/typeutil/` (unexported)
- Checker types are unexported; only interface and registry are public
- Prefix file-specific variables with checker name (e.g., `errgroupGoMethod`)

### Comment Guidelines

**Comments should inform newcomers, not document history.**

- Bad: `// moved from evil.go - this is higher-order function`
- Bad: `// refactored in session 5`
- Good: `// tests basic go fn()() pattern`
- Good: `// cross-function tracking not supported`

**When NOT to comment:**
- Refactoring moves (where something came from)
- Session/date information
- Obvious code behavior

**When to comment:**
- WHY something exists (design rationale)
- `[LIMITATION]` markers for known gaps
- Non-obvious behavior that would confuse readers

**Exception:** Major architectural changes that affect understanding may warrant brief explanation, but prefer updating documentation (CLAUDE.md, ARCHITECTURE.md) over inline comments.

### LIMITATION Comment Format

Test cases that document known analyzer limitations use the `[LIMITATION]:` format:

```go
// [LIMITATION]: Variable reassignment not tracked - uses first assignment only
func limitationReassignedFn(ctx context.Context) {
    fn := func() { doSomething(ctx) }
    fn = func() { doNothing() }  // Reassigned!
    go fn()()  // Currently passes - should fail
}
```

## Documentation Strategy

| File | Purpose | Git Tracked |
|------|---------|-------------|
| `CLAUDE.md` | AI assistant guidance, architecture overview, coding conventions | Yes |
| `docs/` | Detailed design docs, API references, user-facing documentation | Yes |
| `TASKS.md` | Temporary session notes, in-progress work, handoff context | No (gitignored) |

**Principle**: Design decisions, architecture diagrams, and anything useful for future development goes in git-tracked files. `TASKS.md` is ephemeral scratch space for the current session only.

## File Organization

**Proactive file organization is mandatory.** When creating new files or adding symbols to existing files, always evaluate:

1. **Naming consistency**: Does the name follow existing conventions in the directory?
2. **Location appropriateness**: Is this the right directory/package for this content?
3. **File consolidation**: Should this be merged with an existing file?
4. **File splitting**: Is this file getting too large or handling too many concerns?

### Testdata Naming Conventions

Test files are organized by complexity and purpose:

```
testdata/src/<checker>/
├── basic.go           # Core functionality - simple good/bad cases
├── advanced.go        # Complex patterns - higher-order functions, deep nesting
└── evil.go            # Evil edge cases - adversarial/unusual patterns
```

**File Classification Principle:**

Classification is based on **human intuition** - "would a developer write this daily?"

| File | Criterion | Content |
|------|-----------|---------|
| `basic.go` | Daily patterns | Patterns you write and see every day |
| `advanced.go` | Real-world but not daily | Production patterns that are common but not routine |
| `evil.go` | Adversarial | Unusual patterns that test analyzer limits |

**Classification Guidelines:**

1. **basic.go** - Daily patterns (1-level nesting max)
   - Simple good/bad cases (direct context use vs. no use)
   - 1-level goroutine (`go func() { ... }()`)
   - Variable shadowing
   - Ignore directives (`//goroutinectx:ignore`)
   - Multiple context parameters
   - Direct function calls (`go doSomething(ctx)`)

2. **advanced.go** - Real-world complex patterns (production code, but not daily)
   - Defer patterns (deferred cleanup, recovery)
   - Loop patterns (for, range with goroutines)
   - Channel operations (send/receive, select)
   - WaitGroup patterns
   - Method calls on captured objects
   - Control flow (switch, conditional goroutines)

3. **evil.go** - Adversarial patterns (tests analyzer limits)
   - 2+ level goroutine nesting
   - Higher-order functions (`go fn()()`, `go fn()()()`)
   - IIFE (Immediately Invoked Function Expression)
   - Interface method calls
   - `[LIMITATION]` cases documenting analyzer boundaries
   - Goroutines in expressions, deferred functions

**Decision Tree:**

```
Is it 1-level goroutine with straightforward code?
├─ Yes → basic.go
└─ No → Is it a production pattern (defer, loops, channels, WaitGroup)?
         ├─ Yes → advanced.go
         └─ No → evil.go (nesting 2+, go fn()(), IIFE, [LIMITATION])
```

### Trigger Points for Reorganization
- **New file creation**: Consider if existing file should be renamed/split
- **Symbol addition**: Check if file is growing beyond single responsibility
- **Test addition**: Verify test file naming matches pattern

## Test Metadata Management

Test cases are documented in `testdata/metatest/structure.json`:

- JSON Schema validation ensures structure consistency (`structure.schema.json`)
- `validation_test.go` verifies function comments match metadata
- Supports good/bad/limitation/notChecked variants per target

**Adding a new test case:**
1. Add the test function to the appropriate testdata file
2. Add metadata entry in `structure.json` with title, targets, variants
3. Run `go test ./testdata/metatest/...` to validate

## Quality Improvement Cycle

When improving code quality, follow this iterative cycle:

### Phase 1: QA Engineer - Evil Edge Case Testing
- Add thorough, adversarial test cases that push the analyzer to its limits
- Cover edge cases: deep nesting, closures, loops, conditionals, type conversions
- Mark failing cases with `[LIMITATION]:` comments explaining the gap
- Document what the ideal behavior should be vs current behavior

### Phase 2: Implementation Engineer - Address Limitations
- Review all `[LIMITATION]` comments and attempt to resolve them
- Prioritize fixes that improve real-world detection accuracy
- When a limitation is resolved, remove the comment and update the test expectation
- Document truly unfixable limitations

### Phase 3: Code Style Engineer - Refactoring Review
- Review code for clarity, maintainability, and consistency
- Categorize each suggestion:
  - **Should not do**: Would harm readability or add unnecessary complexity
  - **Either way**: Neutral impact, matter of preference
  - **Should do**: Clear improvement to code quality
- Implement all "Should do" items first, then "Either way" items
- Only skip "Should not do" items

**Code Style Engineer Principles:**

1. **Namespace Pollution Intolerance**: The code style engineer strongly opposes "ad-hoc namespace pollution" common in Go's conventional compromises. When a package handles multiple concerns, generic names that only reflect one concern risk collisions. Solutions:
   - Use prefixes to disambiguate
   - Split into separate packages when concerns are distinct enough

2. **Design Pattern Advocate**: The code style engineer loves design patterns and actively proposes their application when encountering ad-hoc code. Particularly favors:
   - **Strategy Pattern** for AST traversal with pluggable behaviors
   - **Visitor Pattern** for tree-structured data processing
   - **Factory Pattern** for creating checker instances

3. **Responsibility Boundary Enforcement**: The code style engineer is strict about clear responsibility boundaries. Each function/type should have a single, well-defined purpose. Cross-cutting concerns should be handled through composition or dependency injection, not ad-hoc parameter passing.

4. **Export/Unexport Discipline**: The code style engineer is particular about visibility. Everything should be unexported by default; only export what is genuinely needed by external packages. Internal helpers, implementation details, and intermediate types must remain unexported to maintain encapsulation.

5. **Method vs Function Discipline**: For packages focused on a single concern, avoid unnecessary methods. Functions are preferred unless:
   - The method genuinely operates on the receiver's state
   - Grouping as methods provides clearer semantic organization
   - Namespace protection is needed (defensive programming)

   If a method doesn't use its receiver, either:
   - Convert it to a function if appropriate
   - Omit the receiver name (use `_`) to signal intentional non-use
   - Keep as method if semantic grouping justifies it (document why)

   **Function → Method Conversion Guideline:**
   When the first argument is clearly the "subject" of the operation, convert to a method:
   - `func doSomething(cctx *CheckContext, target *ast.Expr)` → `func (cctx *CheckContext) doSomething(target *ast.Expr)`
   - The "subject" is the entity performing the action, not just providing context

   **Keep as function** when:
   - There are 2+ arguments and it's unclear which is the "subject"
   - Example: `closureFindFieldInAssignment(cctx, assign, v, fieldName)` - multiple subjects makes method conversion unclear

### Phase 4: Newbie - Naive Questions
Become a complete beginner who has never seen the code. Ask genuinely confused questions:
- "Why are there two checker interfaces? Can't we just use one?"
- "What does 'higher-order function support' mean?"
- "I don't understand why nested closure context doesn't count"
- "What's the flow when I call the analyzer?"

The goal is to identify knowledge gaps and unclear abstractions. Don't pretend to understand - if something is confusing, it needs better documentation.

### Phase 5: Teacher Duo - Explanation & Documentation
The **Implementation Engineer** and **Design Pattern Advocate** collaborate to answer the Newbie's questions:
- Explain concepts step-by-step, building from fundamentals
- Use analogies and diagrams where helpful
- Identify which explanations belong in which document

**Documentation Outputs:**

| Document | Purpose | Style |
|----------|---------|-------|
| `docs/ARCHITECTURE.md` | Precise technical specification | Reference-oriented, complete |
| `docs/TUTORIAL.md` | Step-by-step learning guide | Beginner-friendly, progressive |

Both documents must be kept in sync - when code changes, update both:
- ARCHITECTURE.md: What it is (accurate specification)
- TUTORIAL.md: How to understand it (pedagogical progression)

### Repeat
Continue the cycle until:
- No new meaningful edge cases can be found
- All addressable limitations are resolved
- Code style meets quality standards
- Newbie questions are answered in documentation
- Both reference and tutorial docs are current

## Serena MCP Server Usage Guidelines

When using Serena for code analysis, avoid excessive parallel searches to prevent server freezing.

**Best Practices:**
- Use sequential symbol searches when analyzing broad code areas
- Start with `get_symbols_overview` before diving into `find_symbol` calls
- Prefer single `find_symbol` calls over parallel searches for the same file
- When exploring multiple checkers, analyze them one at a time

**Sequential Pattern (Recommended):**
```
1. get_symbols_overview for file A
2. find_symbol for specific symbol in A
3. get_symbols_overview for file B
4. find_symbol for specific symbol in B
```

**Avoid:**
- Launching multiple parallel `find_symbol` calls across many files
- Running broad searches (e.g., searching entire codebase) in parallel
- Using `search_for_pattern` with very broad patterns in parallel

**Throughput vs Latency:**
When search scope is large, prioritize reliability over speed by executing searches sequentially rather than in parallel.

## Related Projects

- [zerologlintctx](https://github.com/mpyw/zerologlintctx) - Zerolog context propagation linter
- [ctxweaver](https://github.com/mpyw/ctxweaver) - Code generator for context-aware instrumentation
- [gormreuse](https://github.com/mpyw/gormreuse) - GORM instance reuse linter
- [contextcheck](https://github.com/kkHAIKE/contextcheck) - Detects [`context.Background()`](https://pkg.go.dev/context#Background)/[`context.TODO()`](https://pkg.go.dev/context#TODO) misuse
