//declscope:shared // the checkers read their report formats from this table

package checkers

import (
	"fmt"

	"github.com/mpyw/goroutinectx/internal/deriver"
)

// The formats of the checkers' reports, one for each way a check fails.
const (
	// messageGoroutine takes the context's name.
	messageGoroutine = `goroutine does not propagate context "%s"`
	// messageGoroutineDerive and messageGoroutineDeriveDefer take the
	// -goroutine-deriver value.
	messageGoroutineDerive      = "goroutine should call %s to derive context"
	messageGoroutineDeriveDefer = "goroutine calls %s in defer, but it should be called at goroutine start"
	// messageCallback and messageSpawner take the API's name and the
	// context's name, and messageCallbackDeriver and messageSpawnerDeriver
	// are the same with -goroutine-deriver set.
	messageCallback        = "%s() closure should use context %q"
	messageCallbackDeriver = "%s() closure should use context %q or call goroutine deriver"
	messageSpawner         = "%s() func argument should use context %q"
	messageSpawnerDeriver  = "%s() func argument should use context %q or call goroutine deriver"
	// messageTaskCtx and messageTaskVariadic take the API's name, and
	// messageTaskArg the API's name and the argument's ordinal.
	messageTaskCtx      = "%s() 1st argument should call goroutine deriver"
	messageTaskVariadic = "%s() variadic argument should call goroutine deriver"
	messageTaskArg      = "%s() %s argument should call goroutine deriver"
	// messageLabelMissing takes the function's name and the API it calls,
	// and messageLabelUnneeded the function's name.
	messageLabelMissing  = "function %q should have //goroutinectx:spawner directive (calls %s with func argument)"
	messageLabelUnneeded = "function %q has unnecessary //goroutinectx:spawner directive"
)

// messageContextFormat gives format, or withDeriver when derivers is not
// empty.
func messageContextFormat(derivers *deriver.Matcher, format, withDeriver string) string {
	if !derivers.IsEmpty() {
		return withDeriver
	}
	return format
}

// messageOrdinal converts an argument number to its ordinal string (1st,
// 2nd, 3rd, etc.).
func messageOrdinal(n int) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}
