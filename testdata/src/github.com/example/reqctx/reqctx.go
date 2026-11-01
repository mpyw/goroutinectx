// Package reqctx has a carrier that is a context.Context too, for testing
// that such a carrier is judged as a context.
package reqctx

import "context"

// Context is a context.Context, given in -context-carriers.
type Context interface{ context.Context }
