# Architecture Refactoring Plan: ctxrelay Monorepo Split

## Executive Summary

Split ctxrelay into two independent analyzers while maintaining a monorepo structure:
- `ctxrelay/goroutine` (crgoroutine) - Goroutine context propagation
- `ctxrelay/zerolog` (crzerolog) - zerolog chain context injection

Remove slog checker (delegate to [sloglint](https://github.com/go-simpler/sloglint)).

## Motivation

| Concern | Current | After Refactor |
|---------|---------|----------------|
| Goroutine spawning | go, errgroup, waitgroup, gotask, creator | `crgoroutine` |
| Struct chain ctx injection | zerolog | `crzerolog` |
| Non-context variant ban | slog.Info vs InfoContext | **Remove** (use sloglint) |

**Why split?**
1. **golangci-lint integration**: Each analyzer becomes independently configurable
2. **Separation of concerns**: SSA-based zerolog vs AST-based goroutine checks are fundamentally different
3. **Smaller attack surface**: Users only include what they need

## Target Structure

```
ctxrelay/
├── go.mod                          # Single module (monorepo)
├── goroutine/                      # Package: crgoroutine
│   ├── analyzer.go                 # Analyzer entry point
│   ├── analyzer_test.go
│   └── doc.go
├── zerolog/                        # Package: crzerolog
│   ├── analyzer.go
│   ├── analyzer_test.go
│   └── doc.go
├── internal/                       # Shared utilities (not exported)
│   ├── check/                      # CheckContext, ContextScope
│   │   ├── context.go
│   │   ├── closure.go
│   │   └── funcarg.go
│   ├── deriver/                    # DeriveMatcher (goroutine-deriver flag)
│   │   └── matcher.go
│   ├── ignore/                     # IgnoreMap (//ctxrelay:ignore)
│   │   └── ignore.go
│   ├── directive/                  # Directive parsing (//ctxrelay:*)
│   │   └── directive.go
│   ├── typeutil/                   # Type checking utilities
│   │   └── typeutil.go
│   ├── gostmt/                     # go statement checker
│   │   └── checker.go
│   ├── errgroup/                   # errgroup.Group.Go checker
│   │   └── checker.go
│   ├── waitgroup/                  # sync.WaitGroup.Go checker
│   │   └── checker.go
│   ├── gotask/                     # siketyan/gotask checker
│   │   └── checker.go
│   ├── creator/                    # goroutine_creator directive checker
│   │   └── checker.go
│   └── zerolog/                    # zerolog SSA tracer
│       ├── checker.go
│       ├── tracer.go
│       ├── trace.go
│       └── types.go
├── cmd/
│   ├── crgoroutine/                # Standalone CLI
│   │   └── main.go
│   ├── crzerolog/                  # Standalone CLI
│   │   └── main.go
│   └── ctxrelay/                   # Legacy combined CLI (optional, for migration)
│       └── main.go
├── testdata/                       # Shared test fixtures
│   └── src/
│       ├── goroutine/
│       ├── errgroup/
│       ├── waitgroup/
│       ├── gotask/
│       ├── goroutinecreator/
│       ├── goroutinederive/
│       ├── zerolog/
│       └── stubs/                  # External package stubs
│           ├── golang.org/
│           ├── github.com/rs/zerolog/
│           └── github.com/siketyan/gotask/
├── CLAUDE.md
├── README.md
└── docs/
```

## Package Design

### `goroutine/` (crgoroutine)

```go
package crgoroutine

import "golang.org/x/tools/go/analysis"

var Analyzer = &analysis.Analyzer{
    Name: "crgoroutine",
    Doc:  "checks goroutine context propagation",
    // ...
}
```

**Flags:**
- `-goroutine-deriver` - Require deriver function in goroutines
- `-context-carriers` - Additional context carrier types
- `-errgroup` (default: true)
- `-waitgroup` (default: true)
- `-gotask` (default: true)
- `-goroutine-creator` (default: true)

### `zerolog/` (crzerolog)

```go
package crzerolog

import "golang.org/x/tools/go/analysis"

var Analyzer = &analysis.Analyzer{
    Name: "crzerolog",
    Doc:  "checks zerolog chains for .Ctx(ctx) calls",
    // ...
}
```

**Flags:**
- (none currently, may add custom logger type support later)

### `internal/` Structure

| Package | Contents | Used By |
|---------|----------|---------|
| `internal/check` | CheckContext, ContextScope, ClosureChecker | goroutine, zerolog |
| `internal/deriver` | DeriveMatcher, DeriveFuncSpec | goroutine |
| `internal/ignore` | IgnoreMap | goroutine, zerolog |
| `internal/directive` | ParseDirective, GoroutineCreatorDirective | goroutine |
| `internal/typeutil` | IsContextType, IsContextOrCarrierType | goroutine, zerolog |
| `internal/gostmt` | GoStmtChecker | goroutine |
| `internal/errgroup` | ErrgroupChecker | goroutine |
| `internal/waitgroup` | WaitgroupChecker | goroutine |
| `internal/gotask` | GotaskChecker | goroutine |
| `internal/creator` | CreatorChecker | goroutine |
| `internal/zerolog` | SSA tracers, Event/Logger/Context tracers | zerolog |

## Migration Steps

### Phase 1: Restructure Internal Packages (Non-Breaking)

1. Create `internal/` directory structure
2. Move shared utilities:
   - `pkg/analyzer/checkers/checker.go` → `internal/check/`
   - `pkg/analyzer/checkers/deriver.go` → `internal/deriver/`
   - `pkg/analyzer/checkers/ignore.go` → `internal/ignore/`
   - `pkg/analyzer/checkers/directive.go` → `internal/directive/`
   - `pkg/analyzer/checkers/typeutil.go` → `internal/typeutil/`
3. Move checker implementations:
   - `pkg/analyzer/checkers/goroutinechecker/` → `internal/gostmt/`
   - `pkg/analyzer/checkers/errgroupchecker/` → `internal/errgroup/`
   - `pkg/analyzer/checkers/waitgroupchecker/` → `internal/waitgroup/`
   - `pkg/analyzer/checkers/gotaskchecker/` → `internal/gotask/`
   - `pkg/analyzer/checkers/goroutinecreatorchecker/` → `internal/creator/`
   - `pkg/analyzer/checkers/goroutinederivechecker/` → merged into `internal/gostmt/`
   - `pkg/analyzer/checkers/zerologchecker/` → `internal/zerolog/`
4. Update all import paths
5. Tests should still pass with old structure

### Phase 2: Create New Entry Points

1. Create `goroutine/analyzer.go`:
   - Combine gostmt, errgroup, waitgroup, gotask, creator, goroutinederive
   - Register flags
   - Export `Analyzer` variable
2. Create `zerolog/analyzer.go`:
   - Wrap internal/zerolog
   - Export `Analyzer` variable
3. Create new CLI entry points:
   - `cmd/crgoroutine/main.go`
   - `cmd/crzerolog/main.go`
4. Keep `cmd/ctxrelay/main.go` as combined CLI for migration

### Phase 3: Remove slog Checker

1. Remove `pkg/analyzer/checkers/slogchecker/`
2. Remove slog-related test fixtures
3. Update documentation to recommend sloglint

### Phase 4: Cleanup Old Structure

1. Remove `pkg/analyzer/` directory
2. Move test fixtures to `testdata/`
3. Update `go.mod` if needed
4. Update all documentation

### Phase 5: Documentation & Release

1. Update README.md with new usage
2. Update CLAUDE.md
3. Create migration guide for existing users
4. Tag new version

## golangci-lint Integration

After refactoring, users can configure in `.golangci.yml`:

```yaml
linters:
  enable:
    - crgoroutine
    - crzerolog

linters-settings:
  crgoroutine:
    goroutine-deriver: "github.com/my-app/apm.NewGoroutineContext"
    context-carriers:
      - "github.com/labstack/echo/v4.Context"
    errgroup: true
    waitgroup: true
    gotask: true
    goroutine-creator: true
```

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Breaking existing users | Keep `cmd/ctxrelay` as combined CLI during transition |
| Import path changes | Internal packages only, no public API change |
| Test coverage loss | Move tests alongside code, run full suite after each phase |
| golangci-lint compatibility | Test with golangci-lint before release |

## Success Criteria

- [ ] All existing tests pass
- [ ] `crgoroutine` analyzer works standalone
- [ ] `crzerolog` analyzer works standalone
- [ ] golangci-lint can load each analyzer independently
- [ ] Combined CLI still works for migration
- [ ] Documentation updated

## Timeline Estimate

| Phase | Effort |
|-------|--------|
| Phase 1: Restructure | 2-3 hours |
| Phase 2: Entry Points | 1-2 hours |
| Phase 3: Remove slog | 30 min |
| Phase 4: Cleanup | 1 hour |
| Phase 5: Documentation | 1 hour |
| **Total** | **~6-8 hours** |

## Open Questions

1. **Package naming**: `crgoroutine` vs `ctxgoroutine` vs just `goroutine`?
   - Current choice: `crgoroutine` (cr = ctxrelay prefix, avoids collision)

2. **goroutine-derive merger**: Merge `goroutinederivechecker` into `gostmt` or keep separate?
   - Current choice: Merge (it's just an additional check on go statements)

3. **Legacy CLI**: Keep `cmd/ctxrelay` permanently or deprecate?
   - Current choice: Keep for now, decide based on adoption

4. **Testdata location**: Per-package or shared?
   - Current choice: Shared `testdata/` at root (reuse stubs)
