// Forms of struct fields and map values that SSA follows (#116).

package patterns

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ===== Struct field - var declaration =====

// [BAD]: Struct field - var declaration
//
// A struct literal in a var declaration is followed (#116).
func badStructFieldVarDeclarationGoroutine(ctx context.Context) {
	var s = funcHolder{fn: func() { fmt.Println("no ctx") }}
	go s.fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Struct field - var declaration
//
// A struct literal in a var declaration is followed (#116).
func badStructFieldVarDeclarationErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var s = errFuncHolder{fn: func() error { return nil }}
	g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Struct field - var declaration
//
// A struct literal in a var declaration is followed (#116).
func badStructFieldVarDeclarationWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var s = funcHolder{fn: func() {}}
	wg.Go(s.fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Struct field - var declaration
//
// A struct literal in a var declaration is followed (#116).
func badStructFieldVarDeclarationSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var s = errFuncHolder{fn: func() error { return nil }}
	runWithGroup(g, s.fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Struct field - field assignment =====

// [BAD]: Struct field - field assignment
//
// A func assigned to the field is followed (#116).
func badStructFieldFieldAssignmentGoroutine(ctx context.Context) {
	var s funcHolder
	s.fn = func() { fmt.Println("no ctx") }
	go s.fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Struct field - field assignment
//
// A func assigned to the field is followed (#116).
func badStructFieldFieldAssignmentErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var s errFuncHolder
	s.fn = func() error { return nil }
	g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Struct field - field assignment
//
// A func assigned to the field is followed (#116).
func badStructFieldFieldAssignmentWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var s funcHolder
	s.fn = func() {}
	wg.Go(s.fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Struct field - field assignment
//
// A func assigned to the field is followed (#116).
func badStructFieldFieldAssignmentSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var s errFuncHolder
	s.fn = func() error { return nil }
	runWithGroup(g, s.fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Struct field - pointer literal =====

// [BAD]: Struct field - pointer literal
//
// A field of a struct made with &T{} is followed (#116).
func badStructFieldPointerLiteralGoroutine(ctx context.Context) {
	s := &funcHolder{fn: func() { fmt.Println("no ctx") }}
	go s.fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Struct field - pointer literal
//
// A field of a struct made with &T{} is followed (#116).
func badStructFieldPointerLiteralErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := &errFuncHolder{fn: func() error { return nil }}
	g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Struct field - pointer literal
//
// A field of a struct made with &T{} is followed (#116).
func badStructFieldPointerLiteralWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := &funcHolder{fn: func() {}}
	wg.Go(s.fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Struct field - pointer literal
//
// A field of a struct made with &T{} is followed (#116).
func badStructFieldPointerLiteralSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := &errFuncHolder{fn: func() error { return nil }}
	runWithGroup(g, s.fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Struct field - assigned on one branch =====

// [BAD]: Struct field - assigned on one branch
//
// Each func the field can hold on the way to the spawn is checked (#116).
func badStructFieldAssignedOnOneBranchGoroutine(ctx context.Context, b bool) {
	s := funcHolder{fn: func() { _ = ctx }}
	if b {
		s.fn = func() { fmt.Println("no ctx") }
	}
	go s.fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Struct field - assigned on one branch
//
// Each func the field can hold on the way to the spawn is checked (#116).
func badStructFieldAssignedOnOneBranchErrgroup(ctx context.Context, b bool) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { _ = ctx; return nil }}
	if b {
		s.fn = func() error { return nil }
	}
	g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Struct field - assigned on one branch
//
// Each func the field can hold on the way to the spawn is checked (#116).
func badStructFieldAssignedOnOneBranchWaitGroup(ctx context.Context, b bool) {
	var wg sync.WaitGroup
	s := funcHolder{fn: func() { _ = ctx }}
	if b {
		s.fn = func() {}
	}
	wg.Go(s.fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Struct field - assigned on one branch
//
// Each func the field can hold on the way to the spawn is checked (#116).
func badStructFieldAssignedOnOneBranchSpawner(ctx context.Context, b bool) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { _ = ctx; return nil }}
	if b {
		s.fn = func() error { return nil }
	}
	runWithGroup(g, s.fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Map value - var declaration =====

// [BAD]: Map value - var declaration
//
// A map literal in a var declaration is followed (#116).
func badMapValueVarDeclarationGoroutine(ctx context.Context) {
	var m = map[string]func(){"a": func() { fmt.Println("no ctx") }}
	go m["a"]() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Map value - var declaration
//
// A map literal in a var declaration is followed (#116).
func badMapValueVarDeclarationErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	var m = map[string]func() error{"a": func() error { return nil }}
	g.Go(m["a"]) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Map value - var declaration
//
// A map literal in a var declaration is followed (#116).
func badMapValueVarDeclarationWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	var m = map[string]func(){"a": func() {}}
	wg.Go(m["a"]) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Map value - var declaration
//
// A map literal in a var declaration is followed (#116).
func badMapValueVarDeclarationSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	var m = map[string]func() error{"a": func() error { return nil }}
	runWithGroup(g, m["a"]) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Struct field - only a fresh context =====

// [BAD]: Struct field - only a fresh context
//
// The func is judged by SSA, so a context it makes itself does not count (#116).
func badStructFieldOnlyAFreshContextGoroutine(ctx context.Context) {
	s := funcHolder{fn: func() { use(context.Background()) }}
	go s.fn() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: Struct field - only a fresh context
//
// The func is judged by SSA, so a context it makes itself does not count (#116).
func badStructFieldOnlyAFreshContextErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { use(context.Background()); return nil }}
	g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	_ = g.Wait()
}

// [BAD]: Struct field - only a fresh context
//
// The func is judged by SSA, so a context it makes itself does not count (#116).
func badStructFieldOnlyAFreshContextWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := funcHolder{fn: func() { use(context.Background()) }}
	wg.Go(s.fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	wg.Wait()
}

// [BAD]: Struct field - only a fresh context
//
// The func is judged by SSA, so a context it makes itself does not count (#116).
func badStructFieldOnlyAFreshContextSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { use(context.Background()); return nil }}
	runWithGroup(g, s.fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	_ = g.Wait()
}

// ===== Struct field - replaced before the spawn =====

// [GOOD]: Struct field - replaced before the spawn
//
// Only the func the field holds at the spawn counts (#116).
func goodStructFieldReplacedBeforeTheSpawnGoroutine(ctx context.Context) {
	s := funcHolder{fn: func() { fmt.Println("no ctx") }}
	s = funcHolder{fn: func() { _ = ctx }}
	go s.fn()
}

// [GOOD]: Struct field - replaced before the spawn
//
// Only the func the field holds at the spawn counts (#116).
func goodStructFieldReplacedBeforeTheSpawnErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { return nil }}
	s = errFuncHolder{fn: func() error { _ = ctx; return nil }}
	g.Go(s.fn)
	_ = g.Wait()
}

// [GOOD]: Struct field - replaced before the spawn
//
// Only the func the field holds at the spawn counts (#116).
func goodStructFieldReplacedBeforeTheSpawnWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := funcHolder{fn: func() {}}
	s = funcHolder{fn: func() { _ = ctx }}
	wg.Go(s.fn)
	wg.Wait()
}

// [GOOD]: Struct field - replaced before the spawn
//
// Only the func the field holds at the spawn counts (#116).
func goodStructFieldReplacedBeforeTheSpawnSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { return nil }}
	s = errFuncHolder{fn: func() error { _ = ctx; return nil }}
	runWithGroup(g, s.fn)
	_ = g.Wait()
}

// ===== Struct field - read in a closure =====

// [BAD]: Struct field - read in a closure
//
// A struct captured by the goroutine is followed into it (#116).
func badStructFieldReadInAClosureGoroutine(ctx context.Context) {
	s := funcHolder{fn: func() { fmt.Println("no ctx") }}
	go func() {
		_ = ctx
		go s.fn() // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Struct field - read in a closure
//
// A struct captured by the goroutine is followed into it (#116).
func badStructFieldReadInAClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { return nil }}
	func() {
		g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	}()
	_ = g.Wait()
}

// [BAD]: Struct field - read in a closure
//
// A struct captured by the goroutine is followed into it (#116).
func badStructFieldReadInAClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := funcHolder{fn: func() {}}
	func() {
		wg.Go(s.fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	}()
	wg.Wait()
}

// [BAD]: Struct field - read in a closure
//
// A struct captured by the goroutine is followed into it (#116).
func badStructFieldReadInAClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { return nil }}
	func() {
		runWithGroup(g, s.fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	}()
	_ = g.Wait()
}

// ===== Map value - read in a closure =====

// [GOOD]: Map value - read in a closure
//
// A map captured by a closure is followed into it (#302).
func goodMapValueReadInAClosureGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() { use(ctx) }}
	func() {
		go m["a"]()
	}()
}

// [BAD]: Map value - read in a closure
//
// A map captured by a closure is followed into it (#302).
func badMapValueReadInAClosureGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() { fmt.Println("no ctx") }}
	func() {
		go m["a"]() // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Map value - read in a closure
//
// A map captured by a closure is followed into it (#302).
func badMapValueReadInAClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	m := map[string]func() error{"a": func() error { return nil }}
	func() {
		g.Go(m["a"]) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	}()
	_ = g.Wait()
}

// [BAD]: Map value - read in a closure
//
// A map captured by a closure is followed into it (#302).
func badMapValueReadInAClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	m := map[string]func(){"a": func() {}}
	func() {
		wg.Go(m["a"]) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	}()
	wg.Wait()
}

// [BAD]: Map value - read in a closure
//
// A map captured by a closure is followed into it (#302).
func badMapValueReadInAClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	m := map[string]func() error{"a": func() error { return nil }}
	func() {
		runWithGroup(g, m["a"]) // want `runWithGroup\(\) func argument should use context "ctx"`
	}()
	_ = g.Wait()
}

// [BAD]: Map value - read in a goroutine
//
// The goroutine runs a value of a map it captures, and the value does not
// use ctx (#302).
func badMapValueReadInAGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() { fmt.Println("no ctx") }}
	go func() { // want `goroutine does not propagate context "ctx"`
		m["a"]()
	}()
}

// [GOOD]: Map value - read in a goroutine
func goodMapValueReadInAGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() { use(ctx) }}
	go func() {
		m["a"]()
	}()
}

// ===== Map value - written past the literal =====

// [NOTCHECKED]: Map value - updated in a closure
//
// A map a closure updates is not followed, so it passes.
func notCheckedMapValueUpdatedInAClosureGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	func() {
		m["a"] = func() { use(ctx) }
	}()
	func() {
		go m["a"]()
	}()
}

// [NOTCHECKED]: Map value - updated after a closure captures it
//
// A map updated through the variable a closure captures is not followed,
// so it passes.
func notCheckedMapValueUpdatedAfterCaptureGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	run := func() {
		go m["a"]()
	}
	m["a"] = func() { use(ctx) }
	run()
}

// [NOTCHECKED]: Map value - variable replaced
//
// A variable a closure captures that is given another map is not
// followed, so it passes.
func notCheckedMapValueReplacedGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	run := func() {
		go m["a"]()
	}
	m = map[string]func(){"a": func() { use(ctx) }}
	run()
}

// [NOTCHECKED]: Map value - replaced in a closure
//
// A closure that stores another map to the variable is not followed, so it
// passes.
func notCheckedMapValueReplacedInAClosureGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	func() {
		m = map[string]func(){"a": func() { use(ctx) }}
	}()
	func() {
		go m["a"]()
	}()
}

// [NOTCHECKED]: Map value - address taken
//
// A map variable whose address is passed on may be written there, so it
// passes.
func notCheckedMapValueAddressTakenGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	fill(&m)
	func() {
		go m["a"]()
	}()
}

// [NOTCHECKED]: Map value - copied to a variable read past the copy
//
// A copy of the map that is passed on may be updated there, so it passes.
func notCheckedMapValueCopyLoadedGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	alias := m
	fill(&alias)
	func() {
		go m["a"]()
	}()
}

// [NOTCHECKED]: Map value - copied to a variable passed on
//
// A copy of the map whose address is passed on may be updated there, so
// it passes.
func notCheckedMapValueCopyAddressTakenGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	alias := m
	fill(&alias)
	held := m
	func() {
		go held["a"]()
	}()
}

// [NOTCHECKED]: Map value - copied to a variable a closure replaces
//
// A copy of the map in a variable a closure stores to is not followed, so
// it passes.
func notCheckedMapValueCopyReplacedInAClosureGoroutine(ctx context.Context) {
	m := map[string]func(){"a": func() {}}
	alias := m
	func() {
		alias = nil
	}()
	held := m
	func() {
		go held["a"]()
	}()
	_ = alias
}

// ===== Struct field - pointer literal read in a closure =====

// [BAD]: Struct field - pointer literal read in a closure
//
// The struct is followed through the variable that holds its address.
func badStructFieldPointerLiteralReadInAClosureGoroutine(ctx context.Context) {
	s := &funcHolder{fn: func() { fmt.Println("no ctx") }}
	go func() {
		_ = ctx
		go s.fn() // want `goroutine does not propagate context "ctx"`
	}()
}

// [BAD]: Struct field - pointer literal read in a closure
//
// The struct is followed through the variable that holds its address.
func badStructFieldPointerLiteralReadInAClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := &errFuncHolder{fn: func() error { return nil }}
	func() {
		g.Go(s.fn) // want `errgroup.Group.Go\(\) closure should use context "ctx"`
	}()
	_ = g.Wait()
}

// [BAD]: Struct field - pointer literal read in a closure
//
// The struct is followed through the variable that holds its address.
func badStructFieldPointerLiteralReadInAClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := &funcHolder{fn: func() {}}
	func() {
		wg.Go(s.fn) // want `sync.WaitGroup.Go\(\) closure should use context "ctx"`
	}()
	wg.Wait()
}

// [BAD]: Struct field - pointer literal read in a closure
//
// The struct is followed through the variable that holds its address.
func badStructFieldPointerLiteralReadInAClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := &errFuncHolder{fn: func() error { return nil }}
	func() {
		runWithGroup(g, s.fn) // want `runWithGroup\(\) func argument should use context "ctx"`
	}()
	_ = g.Wait()
}

// ===== Struct field - pointer literal written in a closure =====

// [NOTCHECKED]: Struct field - pointer literal written in a closure
//
// A closure may write the struct through its address, so it passes.
func notCheckedStructFieldPointerLiteralWrittenInAClosureGoroutine(ctx context.Context) {
	s := &funcHolder{fn: func() { fmt.Println("no ctx") }}
	func() { s.fn = func() { _ = ctx } }()
	go func() {
		_ = ctx
		go s.fn()
	}()
}

// [NOTCHECKED]: Struct field - pointer literal written in a closure
//
// A closure may write the struct through its address, so it passes.
func notCheckedStructFieldPointerLiteralWrittenInAClosureErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := &errFuncHolder{fn: func() error { return nil }}
	func() { s.fn = func() error { _ = ctx; return nil } }()
	func() {
		g.Go(s.fn)
	}()
	_ = g.Wait()
}

// [NOTCHECKED]: Struct field - pointer literal written in a closure
//
// A closure may write the struct through its address, so it passes.
func notCheckedStructFieldPointerLiteralWrittenInAClosureWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := &funcHolder{fn: func() {}}
	func() { s.fn = func() { _ = ctx } }()
	func() {
		wg.Go(s.fn)
	}()
	wg.Wait()
}

// [NOTCHECKED]: Struct field - pointer literal written in a closure
//
// A closure may write the struct through its address, so it passes.
func notCheckedStructFieldPointerLiteralWrittenInAClosureSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := &errFuncHolder{fn: func() error { return nil }}
	func() { s.fn = func() error { _ = ctx; return nil } }()
	func() {
		runWithGroup(g, s.fn)
	}()
	_ = g.Wait()
}

// ===== Struct field - passed by pointer =====

// [NOTCHECKED]: Struct field - passed by pointer
//
// A struct whose address is passed on may be written there, so it passes.
func notCheckedStructFieldPassedByPointerGoroutine(ctx context.Context) {
	s := funcHolder{fn: func() { fmt.Println("no ctx") }}
	fill(&s)
	go s.fn()
}

// [NOTCHECKED]: Struct field - passed by pointer
//
// A struct whose address is passed on may be written there, so it passes.
func notCheckedStructFieldPassedByPointerErrgroup(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { return nil }}
	fill(&s)
	g.Go(s.fn)
	_ = g.Wait()
}

// [NOTCHECKED]: Struct field - passed by pointer
//
// A struct whose address is passed on may be written there, so it passes.
func notCheckedStructFieldPassedByPointerWaitGroup(ctx context.Context) {
	var wg sync.WaitGroup
	s := funcHolder{fn: func() {}}
	fill(&s)
	wg.Go(s.fn)
	wg.Wait()
}

// [NOTCHECKED]: Struct field - passed by pointer
//
// A struct whose address is passed on may be written there, so it passes.
func notCheckedStructFieldPassedByPointerSpawner(ctx context.Context) {
	g := new(errgroup.Group)
	s := errFuncHolder{fn: func() error { return nil }}
	fill(&s)
	runWithGroup(g, s.fn)
	_ = g.Wait()
}

// funcHolder holds a func in a field.
type funcHolder struct {
	fn func()
}

// errFuncHolder holds a func that returns an error in a field.
type errFuncHolder struct {
	fn func() error
}

func use(ctx context.Context) { _ = ctx }

// fill may write the fields of s.
func fill(s any) { _ = s }
