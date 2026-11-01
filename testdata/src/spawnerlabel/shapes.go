package spawnerlabel

import (
	"golang.org/x/sync/errgroup"

	lib "spawnerfactlib"
)

// ===== WHAT IT PASSES TO A SPAWNER (#377, #378) =====
//
// A function needs the label only when a func value it runs in a goroutine
// comes from one of its own func parameters. Its own literal, named
// function or local variable does not count.

//goroutinectx:spawner
func shapeSpawn(f func()) { go f() }

func shapeWork() {}

// [GOOD]: Passes its own literal (#377, row 1)
func goodShapeLiteral() { shapeSpawn(func() {}) }

// [BAD]: Forwards its own param (row 2)
func badShapeForward(f func()) { shapeSpawn(f) } // want `function "badShapeForward" should have //goroutinectx:spawner directive \(calls shapeSpawn with func argument\)`

// [GOOD]: Passes a named function (row 3)
func goodShapeNamed() { shapeSpawn(shapeWork) }

// [GOOD]: Has a func param, but passes a named function
func goodShapeNamedWithParam(f func()) {
	shapeSpawn(shapeWork)
	f()
}

type shapeT struct{}

//goroutinectx:spawner
func (shapeT) Spawn(f func()) { go f() }

// [GOOD]: Method spawner, literal (row 4a)
func goodShapeMethodLit(t shapeT) { t.Spawn(func() {}) }

// [BAD]: Method spawner, forward (row 4b)
func badShapeMethodFwd(t shapeT, f func()) { t.Spawn(f) } // want `function "badShapeMethodFwd" should have //goroutinectx:spawner directive \(calls Spawn with func argument\)`

//goroutinectx:spawner
func shapeSpawnG[F ~func()](f F) { go f() }

// [GOOD]: Generic spawner, literal (row 5a)
func goodShapeGenericLit() { shapeSpawnG(func() {}) }

// [BAD]: Generic spawner, forward (row 5b)
func badShapeGenericFwd(f func()) { shapeSpawnG(f) } // want `function "badShapeGenericFwd" should have //goroutinectx:spawner directive \(calls shapeSpawnG with func argument\)`

// [GOOD]: Spawner of another package, literal (row 6a)
func goodShapeCrossLit() { lib.Spawn(func() {}) }

// [BAD]: Spawner of another package, forward (row 6b)
func badShapeCrossFwd(f func()) { lib.Spawn(f) } // want `function "badShapeCrossFwd" should have //goroutinectx:spawner directive \(calls Spawn with func argument\)`

// [GOOD]: Forwards to a function of another package that is not a spawner
func goodShapeCrossNotSpawner(f func()) { lib.Run(f) }

// [GOOD]: Spawner through a variable, literal (row 7a)
func goodShapeVarLit() {
	s := shapeSpawn
	s(func() {})
}

// [BAD]: Spawner through a variable, forward (row 7b)
func badShapeVarFwd(f func()) { // want `function "badShapeVarFwd" should have //goroutinectx:spawner directive \(calls shapeSpawn with func argument\)`
	s := shapeSpawn
	s(f)
}

// The external spawner shapes (rows 8a and 8b) are in spawnerlabelexternal.

// [GOOD]: errgroup, literal (row 9)
func goodShapeErrgroupLit() {
	g := new(errgroup.Group)
	g.Go(func() error { return nil })
	_ = g.Wait()
}

// [BAD]: errgroup, forward (row 10, the README example)
func badShapeErrgroupFwd(task func() error) { // want `function "badShapeErrgroupFwd" should have //goroutinectx:spawner directive \(calls errgroup\.Group\.Go with func argument\)`
	g := new(errgroup.Group)
	g.Go(task)
	_ = g.Wait()
}

// [GOOD]: Has a func param, but passes an unrelated literal (row 11)
func goodShapeParamUnrelated(cb func()) {
	shapeSpawn(func() {})
	cb()
}

// [BAD]: Calls its param inside the literal it passes (row 12)
func badShapeParamInLiteral(cb func()) { shapeSpawn(func() { cb() }) } // want `function "badShapeParamInLiteral" should have //goroutinectx:spawner directive \(calls shapeSpawn with func argument\)`

// [BAD]: Captures its param in the literal it passes, without calling it
func badShapeParamCaptured(cb func()) { // want `function "badShapeParamCaptured" should have //goroutinectx:spawner directive \(calls shapeSpawn with func argument\)`
	shapeSpawn(func() {
		g := cb
		_ = g
	})
}

// [BAD]: Captures its param through a reassigned local variable
func badShapeParamCapturedByRef(cb func()) { // want `function "badShapeParamCapturedByRef" should have //goroutinectx:spawner directive \(calls shapeSpawn with func argument\)`
	h := shapeWork
	h = cb
	shapeSpawn(func() { h() })
}

// [GOOD]: The literal it passes captures a func param of another literal
func goodShapeOtherLiteralParam() {
	run := func(cb func()) { cb() }
	shapeSpawn(func() { run(shapeWork) })
}

// [BAD]: go statement on its own param (#378, row 13)
func badShapeGoStmtParam(cb func()) { go cb() } // want `function "badShapeGoStmtParam" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`

// [BAD]: go statement on a literal that calls its param (#378)
func badShapeGoStmtLiteral(cb func()) { go func() { cb() }() } // want `function "badShapeGoStmtLiteral" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`

// [BAD]: go statement that passes its param to the goroutine (#378)
func badShapeGoStmtArg(cb func()) { go func(f func()) { f() }(cb) } // want `function "badShapeGoStmtArg" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`

// [BAD]: go statement on each element of its variadic param (#378)
func badShapeGoStmtVariadic(fns ...func()) { // want `function "badShapeGoStmtVariadic" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	for _, fn := range fns {
		go fn()
	}
}

// [BAD]: go statement on what its factory param returns (#378)
func badShapeGoStmtFactory(factory func() func()) { // want `function "badShapeGoStmtFactory" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	fn := factory()
	go fn()
}

// [GOOD]: go statement on a literal that does not touch its param
func goodShapeGoStmtUnrelated(cb func()) {
	go func() {}()
	cb()
}

// [GOOD]: go statement on its own named function
func goodShapeGoStmtNamed() { go shapeWork() }

// [GOOD]: A func-typed receiver is not a func param
type shapeJob func()

func (j shapeJob) start() { go j() }

// [GOOD]: Local func variable holding a literal (row 14)
func goodShapeLocalVar() {
	f := func() {}
	shapeSpawn(f)
}

// [BAD]: Local func variable holding its param
func badShapeLocalVarParam(cb func()) { // want `function "badShapeLocalVarParam" should have //goroutinectx:spawner directive \(calls shapeSpawn with func argument\)`
	f := cb
	shapeSpawn(f)
}

//goroutinectx:spawner
func shapeSpawnAll(fns ...func()) {
	for _, f := range fns {
		go f()
	}
}

// [GOOD]: Variadic spawner, literals
func goodShapeVariadicLit() { shapeSpawnAll(func() {}, shapeWork) }

// [BAD]: Variadic spawner, one of the elements is its param
func badShapeVariadicElem(cb func()) { shapeSpawnAll(shapeWork, cb) } // want `function "badShapeVariadicElem" should have //goroutinectx:spawner directive \(calls shapeSpawnAll with func argument\)`

// [BAD]: Variadic spawner, its own variadic param spread
func badShapeVariadicSpread(fns ...func()) { shapeSpawnAll(fns...) } // want `function "badShapeVariadicSpread" should have //goroutinectx:spawner directive \(calls shapeSpawnAll with func argument\)`

// shapeWrap gives a closure that calls f.
func shapeWrap(f func()) func() { return func() { f() } }

// [BAD]: Passes what a call made of its param gives
func badShapeWrapped(cb func()) { shapeSpawn(shapeWrap(cb)) } // want `function "badShapeWrapped" should have //goroutinectx:spawner directive \(calls shapeSpawn with func argument\)`

// [GOOD]: Passes what a call made of its own function gives
func goodShapeWrapped() { shapeSpawn(shapeWrap(shapeWork)) }

// ===== A LABEL ADDED AS THE OLD RULE ASKED (#377) =====

// [BAD]: Labeled, but cb only runs in place. The spawner checker would check
// the func arguments of its callers for nothing.
//
//goroutinectx:spawner
func unnecessaryShapeWithCallback(cb func()) { // want `function "unnecessaryShapeWithCallback" has unnecessary //goroutinectx:spawner directive`
	shapeSpawn(func() {})
	cb()
}

// [GOOD]: Labeled, and forwards its param
//
//goroutinectx:spawner
func goodShapeLabeledForward(f func()) { shapeSpawn(f) }

// [GOOD]: Labeled, and its param escapes where the checker cannot follow it
//
//goroutinectx:spawner
func goodShapeLabeledEscapes(ch chan<- func(), f func()) { ch <- f }
