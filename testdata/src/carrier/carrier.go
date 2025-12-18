package carrier

import (
	"github.com/labstack/echo/v4"

	"log/slog"
)

// Test that echo.Context is treated as context carrier when configured.

// [BAD]: Basic echo handler context usage
//
// HTTP handler does not use context from request.
func badEchoHandler(c echo.Context) {
	// Note: slog checker has been removed (delegated to sloglint)
	// This test now only verifies echo.Context is recognized as a context carrier
	slog.Info("missing context")
}

// [GOOD]: Basic echo handler context usage
//
// HTTP handler properly uses context from request.
func goodEchoHandler(c echo.Context) {
	// When using echo.Context, the user would typically extract context.Context
	// and pass it. But the point is that goroutinectx recognizes `c` as a context carrier.
	_ = c // use context carrier
}

// [BAD]: Goroutine in echo handler
//
// Goroutine in Echo handler ignores request context.
func badGoroutineInEchoHandler(c echo.Context) {
	go func() { // want `goroutine does not propagate context "c"`
		// Note: slog checker has been removed (delegated to sloglint)
		println("in goroutine")
	}()
}

// [GOOD]: Goroutine in echo handler
//
// Goroutine in Echo handler properly uses request context.
func goodGoroutineInEchoHandler(c echo.Context) {
	go func() {
		_ = c // captures echo.Context
		println("in goroutine")
	}()
}

// ===== MULTIPLE CONTEXT/CARRIER COMBINATIONS =====

// [GOOD]: Mixed context and carrier - uses carrier
//
// Carrier parameter is properly used in the function body.
func goodMixedCtxAndCarrierUsesCarrier(c echo.Context, prefix string) {
	go func() {
		_ = c // uses carrier
	}()
}

// [BAD]: Mixed context and carrier - uses carrier
//
// Mixed context and carrier - uses neither
func badMixedCtxAndCarrierUsesNeither(c echo.Context, prefix string) {
	go func() { // want `goroutine does not propagate context "c"`
		_ = prefix
	}()
}

// [GOOD]: Carrier as second param - uses it
//
// Carrier parameter is properly used in the function body.
func goodCarrierAsSecondParam(prefix string, c echo.Context) {
	go func() {
		_ = c
	}()
}

// [BAD]: Carrier as second param - uses it
//
// Carrier as second param - does not use it
func badCarrierAsSecondParam(prefix string, c echo.Context) {
	go func() { // want `goroutine does not propagate context "c"`
		_ = prefix
	}()
}
