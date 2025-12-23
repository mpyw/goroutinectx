package echo

import "context"

// Context is the interface for echo's context.
type Context interface {
	Request() any
	Response() any
	Get(key string) any
	Set(key string, val any)
	RealContext() context.Context
	// Real echo.Context has many more methods...
}

// echoContext is a concrete implementation (internal to echo).
type echoContext struct{}

func (c *echoContext) Request() any                 { return nil }
func (c *echoContext) Response() any                { return nil }
func (c *echoContext) Get(key string) any           { return nil }
func (c *echoContext) Set(key string, val any)      {}
func (c *echoContext) RealContext() context.Context { return context.Background() }

// New creates a new echo context (stub).
func New() Context {
	return &echoContext{}
}
