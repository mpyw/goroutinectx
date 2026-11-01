// Stub package for testing
package context

import "context"

// Context is an alias of context.Context, as in the real package.
type Context = context.Context

func Background() Context { return context.Background() }

func TODO() Context { return context.TODO() }

// CancelFunc is an alias of context.CancelFunc, as in the real package.
type CancelFunc = context.CancelFunc

func WithCancel(parent Context) (Context, CancelFunc) { return context.WithCancel(parent) }

func WithValue(parent Context, key, val any) Context { return context.WithValue(parent, key, val) }
