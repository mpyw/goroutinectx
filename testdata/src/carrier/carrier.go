package carrier

import (
	"github.com/labstack/echo/v4"
	"golang.org/x/sync/errgroup"

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

// [GOOD]: Goroutine takes the carrier as a param
//
// The closure takes the carrier as a param, as it would a context (#115).
func goodGoroutineCarrierParam(c echo.Context) {
	go func(c echo.Context) {
		_ = c
	}(c)
}

//goroutinectx:spawner
func runWithCarrier(fn func(echo.Context)) {
	go fn(echo.New())
}

// [GOOD]: Spawner func argument takes the carrier as a param
//
// The func argument takes the carrier as a param, as it would a context (#115).
func goodSpawnerCarrierParam(c echo.Context) {
	runWithCarrier(func(c echo.Context) {
		_ = c
	})
}

// [GOOD]: Errgroup closure from a factory that takes the carrier
//
// The factory takes the carrier as a param, so what it makes passes.
func goodErrgroupCarrierFactory(c echo.Context) {
	g := new(errgroup.Group)
	g.Go(carrierWorker(c))
	_ = g.Wait()
}

func carrierWorker(c echo.Context) func() error {
	return func() error {
		_ = c
		return nil
	}
}
