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
//	| Holds      | a struct, or a pointer to one, with a field of one of these   |
//	|            | kinds that the analyzed code can set                          |
//
// A [Classifier] knows the carriers, the context.Context interface, the
// analyzed package and its module of one pass. env.Env builds it once for
// each pass.
//
// # Thresholds
//
// Each caller asks for the set of kinds it counts, by name:
//
//	if kinds.HasParam(fn, ctxkind.Given) { ... }       // a value a goroutine gets
//	if kinds.Is(t, ctxkind.Captured) { ... }           // what a closure captures
//	names := kinds.ParamNames(info, ft, ctxkind.Param) // what puts a function under check
//	if kinds.Is(t, ctxkind.Context) { ... }            // context.Context only
//
// A value a goroutine gets counts by the wide sets, Captured and Given, so
// that when in doubt nothing is reported. A function is put under check by
// the narrow set, Param, as checking more functions means more reports
// (#210). These differences are kept on purpose:
//
//   - Param leaves out Implements and Holds. A handler that takes a
//     *gin.Context, which implements context.Context, or a struct with a
//     context field, is not checked.
//   - Captured leaves out Holds. A captured struct that holds a context
//     counts only when it is given to a call, which the caller tells.
//   - A parameter that exempts a closure from the deriver counts by
//     Captured, not Given. A struct that holds a context is not a context
//     of the closure's own (#256).
//   - The fresh argument of a call is tested with Context. Only a
//     context.Context can be a fresh Background() or TODO().
//   - [Classifier.HasParam] counts a blank parameter and a receiver, which
//     are given a value. [Classifier.ParamNames] leaves them out, as a
//     report cannot name them.
//
// A field counts for Holds when it is exported, or declared in the analyzed
// package or its module. [Classifier.From] adds the package of a closure of
// another package. A wrapper SSA makes has no package of its own, and is
// judged for the analyzed package.
//
// # Fresh and With Calls
//
// [IsFreshCall] tells Background and TODO, which give a context that
// carries nothing of the caller's. [IsWithCall] tells WithCancel, WithValue
// and the like, which make a context from the one they are given. Both take
// the functions of package context and of golang.org/x/net/context, whose
// With functions call those of context.
package ctxkind
