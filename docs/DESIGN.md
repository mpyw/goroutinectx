# Design Document

This document explains **why** goroutinectx was designed the way it is. For technical specifications and implementation details, see [ARCHITECTURE.md](./ARCHITECTURE.md).

## Problem Statement

In Go applications, `context.Context` is used for:
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
4. **Type-safe analysis** - use `go/types` for accurate detection

## Non-Goals

1. Auto-fixing (may be added later)
2. Runtime checking
3. Configuration files (flags only for v1)
4. Custom rule definition (future version)

---

## Key Design Decisions

### 1. `inspector.WithStack` for Nested Function Support

**Decision:** Use `inspector.WithStack` instead of `ast.Inspect`.

**Rationale:** Proper tracking of context through nested functions and closures requires knowing the enclosing function at any point in the AST traversal. `inspector.WithStack` provides this stack context, enabling correct handling of:
- Nested functions at any depth
- Closures capturing context
- Shadowed context parameters
- Context introduced in middle layers

### 2. Type-Safe Analysis

**Decision:** Use `go/types` for all type checking instead of name-based string matching.

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

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2024-12-15 | No config files for v1.0 | Keep it simple. Add later if needed |
| 2024-12-15 | Checker interface pattern | Extensibility and testability |
| 2024-12-15 | Type info over names | Name-based matching is error-prone |
| 2024-12-15 | Use `inspector.WithStack` | Accurate tracking of nested functions |
| 2024-12-17 | Interface segregation | CallChecker vs GoStmtChecker separation |
| 2024-12-17 | Nested closure rule | Explicit context acknowledgment at each level |

## References

- [go/analysis package](https://pkg.go.dev/golang.org/x/tools/go/analysis)
- [Writing Go Analysis Tools](https://arslan.io/2019/06/13/using-go-analysis-to-write-a-custom-linter/)
- [golangci-lint custom linters](https://golangci-lint.run/contributing/new-linters/)
