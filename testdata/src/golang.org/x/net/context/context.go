// Stub package for testing
package context

import "context"

// Context is an alias of context.Context, as in the real package.
type Context = context.Context

func Background() Context { return context.Background() }

func TODO() Context { return context.TODO() }
