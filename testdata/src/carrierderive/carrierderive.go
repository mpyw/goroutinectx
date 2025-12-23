package carrierderive

import (
	"github.com/labstack/echo/v4"

	"github.com/my-example-app/telemetry/apm"
)

// Test cases for goroutine-derive checker with context carriers.
// Tests context extracted from carrier with different reference patterns.

// ===== INLINE REFERENCE PATTERNS =====

// [GOOD]: Carrier context inline - calls deriver
//
// Context extracted inline from carrier, deriver called.
func goodCarrierContextInline(c echo.Context) {
	go func() {
		ctx := apm.NewGoroutineContext(c.RealContext())
		_ = ctx
	}()
}

// [BAD]: Carrier context inline - calls deriver
//
// Context extracted inline from carrier, but no deriver called.
func badCarrierContextInlineNoDeriver(c echo.Context) {
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		ctx := c.RealContext()
		_ = ctx
	}()
}

// ===== LOCAL VARIABLE PATTERNS =====

// [GOOD]: Carrier context local var - calls deriver
//
// Context extracted to local variable inside goroutine, deriver called.
func goodCarrierContextLocalVar(c echo.Context) {
	go func() {
		ctx := c.RealContext()
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
	}()
}

// [BAD]: Carrier context local var - calls deriver
//
// Context extracted to local variable inside goroutine, no deriver called.
func badCarrierContextLocalVarNoDeriver(c echo.Context) {
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		ctx := c.RealContext()
		_ = ctx
	}()
}

// ===== FREEVAR PATTERNS =====

// [GOOD]: Carrier context freevar - calls deriver
//
// Context extracted outside, captured as freevar by closure, deriver called.
func goodCarrierContextFreeVar(c echo.Context) {
	ctx := c.RealContext()
	go func() {
		newCtx := apm.NewGoroutineContext(ctx)
		_ = newCtx
	}()
}

// [BAD]: Carrier context freevar - calls deriver
//
// Context extracted outside, captured as freevar by closure, no deriver called.
func badCarrierContextFreeVarNoDeriver(c echo.Context) {
	ctx := c.RealContext()
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		_ = ctx
	}()
}

// ===== CARRIER DIRECTLY CAPTURED =====

// [GOOD]: Carrier captured directly - calls deriver
//
// Carrier itself captured by closure, context extracted inside, deriver called.
func goodCarrierCapturedDirect(c echo.Context) {
	go func() {
		ctx := apm.NewGoroutineContext(c.RealContext())
		_ = ctx
	}()
}

// [BAD]: Carrier captured directly - calls deriver
//
// Carrier itself captured by closure, but deriver not called.
func badCarrierCapturedDirectNoDeriver(c echo.Context) {
	go func() { // want "goroutine should call github.com/my-example-app/telemetry/apm.NewGoroutineContext to derive context"
		_ = c // captures carrier but doesn't call deriver
	}()
}
