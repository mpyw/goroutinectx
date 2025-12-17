# Architecture Refactoring Plan: ctxrelay Simplification

## Executive Summary

Simplify ctxrelay to focus solely on **goroutine context propagation**.

- **Remove**: zerolog checker, slog checker
- **Keep**: go stmt, errgroup, waitgroup, gotask, goroutine_creator, goroutine-deriver

## Motivation

| Concern | Tool |
|---------|------|
| Goroutine spawning ctx propagation | **ctxrelay** (this project) |
| zerolog .Ctx(ctx) chain | zerologlint (PR) or zerologlintctx (separate) |
| slog.Info vs InfoContext | sloglint |
| context.Background() misuse | contextcheck |

**Why simplify?**
1. **Single responsibility**: Goroutine context propagation only
2. **Ecosystem collaboration**: Extend existing linters rather than duplicate
3. **Cleaner codebase**: Remove SSA complexity (zerolog), simpler maintenance

## Target Structure

```
ctxrelay/
├── go.mod
├── analyzer.go                     # Main analyzer entry point
├── analyzer_test.go
├── doc.go
├── internal/
│   ├── check/                      # CheckContext, ContextScope
│   │   ├── context.go
│   │   ├── closure.go
│   │   └── funcarg.go
│   ├── deriver/                    # DeriveMatcher (goroutine-deriver flag)
│   │   └── matcher.go
│   ├── ignore/                     # IgnoreMap (//ctxrelay:ignore)
│   │   └── ignore.go
│   ├── directive/                  # Directive parsing (//ctxrelay:goroutine_creator)
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
│   └── creator/                    # goroutine_creator directive checker
│       └── checker.go
├── cmd/
│   └── ctxrelay/
│       └── main.go
├── testdata/
│   └── src/
│       ├── goroutine/
│       ├── errgroup/
│       ├── waitgroup/
│       ├── gotask/
│       ├── goroutinecreator/
│       ├── goroutinederive/
│       └── stubs/                  # External package stubs
│           ├── golang.org/x/sync/errgroup/
│           ├── github.com/siketyan/gotask/
│           └── github.com/my-example-app/telemetry/apm/
├── CLAUDE.md
├── README.md
└── docs/
```

## What Gets Removed

| File/Directory | Reason |
|----------------|--------|
| `pkg/analyzer/checkers/zerologchecker/` | Delegate to zerologlint |
| `pkg/analyzer/checkers/slogchecker/` | Delegate to sloglint |
| `testdata/src/zerolog/` | No longer needed |
| `testdata/src/slog/` | No longer needed |
| `testdata/src/github.com/rs/zerolog/` | No longer needed |
| `-zerolog` flag | Removed |
| `-slog` flag | Removed |

## What Gets Kept (and Reorganized)

| Current | New Location |
|---------|--------------|
| `pkg/analyzer/analyzer.go` | `analyzer.go` |
| `pkg/analyzer/checkers/checker.go` | `internal/check/` |
| `pkg/analyzer/checkers/deriver.go` | `internal/deriver/` |
| `pkg/analyzer/checkers/ignore.go` | `internal/ignore/` |
| `pkg/analyzer/checkers/directive.go` | `internal/directive/` |
| `pkg/analyzer/checkers/typeutil.go` | `internal/typeutil/` |
| `pkg/analyzer/checkers/goroutinechecker/` | `internal/gostmt/` |
| `pkg/analyzer/checkers/goroutinederivechecker/` | merged into `internal/gostmt/` |
| `pkg/analyzer/checkers/errgroupchecker/` | `internal/errgroup/` |
| `pkg/analyzer/checkers/waitgroupchecker/` | `internal/waitgroup/` |
| `pkg/analyzer/checkers/gotaskchecker/` | `internal/gotask/` |
| `pkg/analyzer/checkers/goroutinecreatorchecker/` | `internal/creator/` |

## Flags (After Refactor)

```
-goroutine-deriver    Require deriver function call in goroutines
-context-carriers     Additional context carrier types (comma-separated)
-errgroup            Enable errgroup.Group.Go checker (default: true)
-waitgroup           Enable sync.WaitGroup.Go checker (default: true)
-gotask              Enable gotask checker (default: true)
-goroutine-creator   Enable goroutine_creator directive (default: true)
```

## Migration Steps

### Phase 1: Remove zerolog & slog

1. Delete `pkg/analyzer/checkers/zerologchecker/`
2. Delete `pkg/analyzer/checkers/slogchecker/`
3. Delete related test fixtures
4. Remove flags from `analyzer.go`
5. Update imports
6. Run tests

### Phase 2: Flatten Structure

1. Move `pkg/analyzer/analyzer.go` → `analyzer.go`
2. Move `pkg/analyzer/analyzer_test.go` → `analyzer_test.go`
3. Move `pkg/analyzer/checkers/` → `internal/`
4. Move `pkg/analyzer/testdata/` → `testdata/`
5. Update all import paths
6. Delete empty `pkg/` directory

### Phase 3: Reorganize Internal

1. Split `internal/checker.go` into focused files:
   - `internal/check/context.go` - ContextScope, FindContextScope
   - `internal/check/closure.go` - CheckClosureUsesContext
   - `internal/check/funcarg.go` - CheckFuncArgUsesContext, tracing helpers
2. Move deriver to `internal/deriver/`
3. Move ignore to `internal/ignore/`
4. Move directive to `internal/directive/`
5. Move typeutil to `internal/typeutil/`
6. Rename checker directories (drop "checker" suffix)

### Phase 4: Merge goroutine-derive into gostmt

1. `goroutinederivechecker` logic merges into `gostmt/checker.go`
2. Both check `*ast.GoStmt`, just different aspects
3. Delete separate `goroutinederivechecker/` directory

### Phase 5: Documentation & Cleanup

1. Update README.md - remove zerolog/slog sections
2. Update CLAUDE.md - reflect new structure
3. Delete TASK_ARCH_REFACTOR.md (this file)
4. Clean up any remaining references

## Package API (After Refactor)

```go
package ctxrelay

import "golang.org/x/tools/go/analysis"

// Analyzer checks goroutine context propagation.
var Analyzer *analysis.Analyzer
```

## Usage (After Refactor)

```bash
# Standalone
ctxrelay ./...

# With go vet
go vet -vettool=$(which ctxrelay) ./...

# With goroutine-deriver
ctxrelay -goroutine-deriver=github.com/my-app/apm.NewGoroutineContext ./...
```

## Recommended Companion Linters

Document in README that users should combine with:

```yaml
# .golangci.yml
linters:
  enable:
    - ctxrelay        # goroutine context propagation
    - contextcheck    # context.Background() misuse
    - sloglint        # slog.Info vs InfoContext
    # - zerologlint   # zerolog .Ctx() (if/when available)
```

## Timeline Estimate

| Phase | Effort |
|-------|--------|
| Phase 1: Remove zerolog & slog | 30 min |
| Phase 2: Flatten structure | 1 hour |
| Phase 3: Reorganize internal | 1-2 hours |
| Phase 4: Merge goroutine-derive | 30 min |
| Phase 5: Documentation | 30 min |
| **Total** | **~4 hours** |

## Success Criteria

- [ ] zerolog/slog code completely removed
- [ ] All goroutine-related tests pass
- [ ] Structure matches target layout
- [ ] Single `Analyzer` export at package root
- [ ] README updated with companion linter recommendations
- [ ] CLAUDE.md reflects new architecture

## Future Consideration: Rename to goroutinectx

| Current | Proposed |
|---------|----------|
| `ctxrelay` | `goroutinectx` |
| `github.com/mpyw/ctxrelay` | `github.com/mpyw/goroutinectx` |

**Rationale:**
- `ctxrelay` is abstract ("relay context" - to where?)
- `goroutinectx` is direct ("goroutine context" - exactly what it checks)

**When to rename:**
- After architecture refactor is complete
- Before first public release
- Single commit: rename repo + update all references
