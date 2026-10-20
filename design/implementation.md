# Implementation notes

This file records design decisions and implementation details. The short, always-loaded instructions are in [AGENTS.md](../AGENTS.md).

## Project Overview

**goroutinectx** is a Go linter that enforces context propagation best practices in goroutines. It detects cases where a [`context.Context`](https://pkg.go.dev/context#Context) is available in function parameters but not properly passed to goroutines and related APIs.

### Supported Checkers

- **goroutine**: Detect `go func()` that doesn't capture/use context
- **errgroup**: Detect [`errgroup.Group.Go()`](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.Go) closures without context
- **waitgroup**: Detect [`sync.WaitGroup.Go()`](https://pkg.go.dev/sync#WaitGroup.Go) closures without context (Go 1.25+)
- **conc**: Detect [conc](https://pkg.go.dev/github.com/sourcegraph/conc) pool closures without context
- **spawner**: Detect calls to spawner functions that pass closures without context
  - Directive: `//goroutinectx:spawner` marks local functions
  - Flag: `-external-spawner=pkg/path.Func` or `-external-spawner=pkg/path.Type.Method` for external functions
- **goroutine-derive**: Detect goroutines that don't call a specified context-derivation function (e.g., `apm.NewGoroutineContext`)
  - Activated via flag: `-goroutine-deriver=pkg/path.Func` or `-goroutine-deriver=pkg/path.Type.Method`
  - OR (comma): `-goroutine-deriver=pkg1.Func1,pkg2.Func2` - at least one must be called
  - AND (plus): `-goroutine-deriver=pkg1.Func1+pkg2.Func2` - all must be called
  - Mixed: `-goroutine-deriver=pkg1.Func1+pkg2.Func2,pkg3.Func3` - (Func1 AND Func2) OR Func3
- **gotask**: Detect [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2) task functions without context derivation (requires `-goroutine-deriver`)
  - `Do*` functions: checks that task arguments call the deriver
  - [`Task.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#Task.DoAsync) / [`CancelableTask.DoAsync`](https://pkg.go.dev/github.com/siketyan/gotask/v2#CancelableTask.DoAsync): checks that ctx argument is derived

### Directives

Only `//goroutinectx:name [args]` (a line comment, a lowercase name, no spaces) is a directive (`go/ast.ParseDirective`). Any other comment starting with `goroutinectx:` is reported as malformed, and a directive with a name goroutinectx does not read is reported as unknown. A trailing `// ...` is a reason and is dropped before parsing, as declscope does.

- `//goroutinectx:ignore` - Suppress warnings on its own line, or, written on a line of its own, on the next line. A report on an argument of a call is also silenced by an ignore for the line the call starts
  - Checker-specific: `//goroutinectx:ignore goroutine` or `//goroutinectx:ignore goroutine,errgroup`
  - Valid checker names: `goroutine`, `goroutinederive`, `waitgroup`, `errgroup`, `conc`, `spawner`, `spawnerlabel`, `gotask`
  - A reason follows `//`, or ` - ` for compatibility.
  - Unused ignore detection: reports unused ignore directives

**An unknown checker name is reported, and the directive silences nothing** (#65). Names used to be accepted unchecked. A reason written without a separator, a misspelled name, or `// reason` with no checker became a checker that never reports. The ignore then did nothing, and its only report said "unused ... for checker(s): <the reason>". The whole directive is dropped on one unknown name, as `//declscope:ignore` is. Guessing which part was meant would silence a report nobody asked to silence.

**An ignore is asked only when a report would be made.** The runner used to ask it before running the check. That marked the ignore used on a line with nothing to report, so an ignore that silenced nothing was never reported as unused. `badUnusedIgnore` in `testdata/src/goroutine` passed only because the parse bug above turned its `// want` into a checker name. Checkers return their diagnostics, and the runner asks the ignore at each diagnostic's own line, so a report on an argument is silenced by an ignore at that argument.

**`conc` is a checker name of its own.** The conc checker used to report under `errgroup`, so `//goroutinectx:ignore conc`, listed in the README, never worked. An `errgroup` ignore no longer silences a conc report. Keeping it would leave two names for one checker, and compatibility was not a goal here. The one form kept for compatibility is the ` - ` reason, which the README documented.
- `//goroutinectx:spawner` - Mark a function as spawning goroutines with its func arguments

Directive matching and file filters use unadjusted positions (`fset.PositionFor(pos, false)`), so a `//line` directive does not move them. Only diagnostics show adjusted positions.

## Architecture

```
goroutinectx/
├── analyzer.go                # Main analyzer: flags resolved into run.Config
├── plugin/                    # golangci-lint module plugin: settings resolved into run.Config
├── analyzer_test.go           # Integration tests using analysistest
├── internal/
│   ├── run/                   # Run: every checker over one package, with a Config;
│   │                          # the checker interfaces and the runner that applies ignores
│   ├── checker/               # Checker names
│   ├── checkers/              # Checker implementations
│   │   ├── goroutine.go       # go statement checkers (goroutine, goroutinederive)
│   │   ├── spawn.go           # errgroup, waitgroup, conc and spawner checkers
│   │   ├── gotask.go          # gotask library checker
│   │   └── spawnerlabel/      # spawnerlabel checker
│   ├── carrier/               # Context carrier types (-context-carriers)
│   ├── deriver/               # Matcher for OR/AND deriver matching
│   ├── directive/             # Directive parsing
│   │   ├── ignore/            # //goroutinectx:ignore
│   │   └── spawner/           # //goroutinectx:spawner
│   ├── funcspec/              # pkg/path.Func and pkg/path.Type.Method specs
│   ├── probe/                 # AST lookups of struct fields and indexed elements
│   ├── registry/              # The one table of supported spawning APIs
│   ├── scope/                 # Context scope detection
│   ├── ssa/                   # SSA program, closure checks, value resolution, call walker
│   └── typeutil/              # Type checking utilities
├── testdata/
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

1. **Type-safe analysis**: Uses [`go/types`](https://pkg.go.dev/go/types) for accurate detection (not just name-based)
2. **Nested function support**: Uses [`inspector.WithStack`](https://pkg.go.dev/golang.org/x/tools/go/ast/inspector#Inspector.WithStack) to track context through closures
3. **Shadowing support**: `UsesContext` uses type identity, not name matching
4. **Interface segregation**: call and go statement checker interfaces (no BaseChecker)
5. **Minimal exports**: Only necessary types/functions are exported from `checkers` package
6. **Zero false positives**: Prefer missing issues over false alarms
7. **Multiple context tracking**: Tracks ALL context parameters, not just the first one. If ANY context variable is used, the check passes. Error messages report the first context name for consistency.

### Checker Interface Design

```go
// Separated interfaces, in internal/run. Each returns its reports, and the
// runner applies //goroutinectx:ignore at each report's position.
type goStmtChecker interface {
    Name() checker.Name
    CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) []analysis.Diagnostic
}

type callChecker interface {
    Name() checker.Name
    CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic
}
```

The set of checkers named by an unused-ignore report is the set of checkers built: their `Name()`s.

**Why two interfaces?**
- `goStmtChecker`: For `go` keyword statements (`go func() {}()`)
- `callChecker`: For function calls (`g.Go()`, `wg.Go()`)

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

**How a spawned value is resolved.** The checkers find the `ssa.Go` or `ssa.Call` of the AST node, and take the SSA value of the spawned function or callback argument. `ssa.FuncResolver` follows it back through Phi nodes, loads of local variables and calls of factories to each function literal, and runs the same SSA check an inline literal gets. So a variable, a `var` declaration, a branch and a factory are all checked like the literal itself, and only the value that reaches the call counts: a later assignment does not. A value it cannot follow passes: a parameter, an interface method, a variable a closure writes. A struct field or an indexed element is still looked up on the AST.

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
- `interface{}` type assertion - can't trace func through type assertion
- A variable written by a closure - not followed, so it passes
- A parenthesized function, as `go (fn)()` - not traced

### Derive Function Matching (DeriveMatcher)

`internal/deriver/` provides shared OR/AND logic for matching derive functions. Used by:
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

The gotask checker handles the [gotask](https://pkg.go.dev/github.com/siketyan/gotask/v2) library:

| Target | Check |
|--------|-------|
| `gotask.Do*` functions | Task arguments (2nd+) must call deriver in their body |
| `Task.DoAsync` | Context argument (1st) must be derived |
| `CancelableTask.DoAsync` | Context argument (1st) must be derived |

**Key insight:** Since gotask tasks run as goroutines, they need to call the deriver function inside their body - there's no way to wrap the context at the call site.

**Known Limitations:**
- The elements of a slice variable expanded with `...` can't be traced, so it is reported
- A task from an interface method can't be traced, so it is reported

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
> Always use `./test_all.sh` before committing. It runs all tests and golangci-lint.

## Releasing

Releases are cut from GitHub Actions, never locally:

- **Tag and Release** (`tag_and_release.yml`) — the normal path: creates the `vX.Y.Z` tag, then builds and publishes.
- `tag.yml` / `release.yml` are the two halves, dispatchable on their own when a tag already exists.

`release.yml` runs goreleaser (`.goreleaser.yaml`) once for all six targets — darwin/linux/windows x amd64/arm64, `CGO_ENABLED=0` — and uploads the archives plus `checksums.txt` to the release. Those assets are what makes `mise use -g "github:mpyw/goroutinectx"` work, so a release with no assets breaks the documented install path. goreleaser is pinned to 2.17.1: 2.18.0 attests every build artifact, which fails the job.

Validate config changes with `goreleaser check`, and dry-run with `goreleaser release --snapshot --clean --skip=publish`.

## Adding a New Checker

1. Add the checker to `internal/checkers`, and its name to `internal/checker`:
```go
type MyChecker struct{}

func (*MyChecker) Name() checker.Name { return checker.My }

// For call expression checks. A spawning API goes in the table in
// internal/registry, with this checker's name.
func (c *MyChecker) CheckCall(cctx *probe.Context, call *ast.CallExpr) []analysis.Diagnostic {
    // Return nil for a call it does not check, else one diagnostic per
    // report, at its own position
}

// OR for go statement checks
func (c *MyChecker) CheckGoStmt(cctx *probe.Context, stmt *ast.GoStmt) []analysis.Diagnostic { ... }
```

2. Build it in `buildCheckers` in `internal/run/run.go`

3. Add test fixtures in `testdata/src/<name>/`

4. Add test case in `analyzer_test.go`

## Testing Strategy

- Use `analysistest` for all analyzer tests
- Test fixtures use `// want` comments for expected diagnostics
- Each fixture function's doc comment starts with its kind and a title (see [Test Case Comments](#test-case-comments))
- Test structure per checker:
  - `basic.go` - Simple good/bad cases
  - `advanced.go` - Complex patterns (defer, loops, channels)
  - `evil.go` - Adversarial patterns (nesting, IIFE, limitations)

## Code Style

- Follow standard Go conventions
- Use [`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis) framework
- Prefer [`inspector.WithStack`](https://pkg.go.dev/golang.org/x/tools/go/ast/inspector#Inspector.WithStack) over [`ast.Inspect`](https://pkg.go.dev/go/ast#Inspect) for traversal
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

**Exception:** Major architectural changes that affect understanding may warrant brief explanation, but prefer updating documentation (AGENTS.md, design/implementation.md, ARCHITECTURE.md) over inline comments.

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
| `AGENTS.md` | Short repository instructions | Yes |
| `design/implementation.md` | Detailed implementation decisions | Yes |
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

## Test Case Comments

The `// want` comments are the assertions. The doc comment of each fixture function says what kind of case it is, on its first line:

| First line | Meaning |
|------------|---------|
| `// [BAD]: <title>` | Should report. Has a `// want` |
| `// [GOOD]: <title>` | Should not report |
| `// [LIMITATION]: <title>` | The analyzer gets this case wrong. The comment says how, and the `// want` comments record what it does today |
| `// [NOTCHECKED]: <title>` | The analysis cannot follow the value. The `// want` comments record what it does instead |

A helper function needs no such line.

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

## Related Projects

- [zerologlintctx](https://github.com/mpyw/zerologlintctx) - Zerolog context propagation linter
- [ctxweaver](https://github.com/mpyw/ctxweaver) - Code generator for context-aware instrumentation
- [gormreuse](https://github.com/mpyw/gormreuse) - GORM instance reuse linter
- [contextcheck](https://github.com/kkHAIKE/contextcheck) - Detects [`context.Background`](https://pkg.go.dev/context#Background)/[`context.TODO`](https://pkg.go.dev/context#TODO) misuse
