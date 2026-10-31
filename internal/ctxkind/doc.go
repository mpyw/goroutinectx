// Package ctxkind tells what counts as a context.
//
// # Kinds
//
// A [Kind] is a set of the ways a type is, or leads to, a context:
//
//	| Kind       | A type of this kind is                                        |
//	| ---------- | ------------------------------------------------------------- |
//	| Context    | context.Context, an interface that embeds it, or a type       |
//	|            | parameter constrained by one                                  |
//	| Carrier    | a type of -context-carriers                                   |
//	| Implements | a type that implements context.Context, as a struct that      |
//	|            | embeds one does                                               |
//	| Holds      | a struct with a field of one of these kinds that the analyzed |
//	|            | code can set                                                  |
//
// A [Classifier] knows the carriers, the context.Context interface and the
// module of one pass. env.Env builds it once for each pass.
//
// # Thresholds
//
// Each caller asks for the set of kinds it counts, by name:
//
//	if kinds.HasParam(fn, ctxkind.Param) { ... }      // a context or a carrier
//	if kinds.Is(t, ctxkind.Captured) { ... }          // also a type that implements one
//	if kinds.Is(t, ctxkind.Context) { ... }           // context.Context only
//
// The callers do not agree yet on which set counts. That is #210, step 2.
//
// # Fresh and With Calls
//
// [IsFreshCall] tells context.Background and context.TODO, which give a
// context that carries nothing of the caller's. [IsWithCall] tells
// context.WithCancel, context.WithValue and the like, which make a context
// from the one they are given.
package ctxkind
