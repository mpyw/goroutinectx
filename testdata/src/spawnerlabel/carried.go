package spawnerlabel

import (
	"context"
	"iter"

	"golang.org/x/sync/errgroup"

	gotask "github.com/siketyan/gotask/v2"
)

// ===== A VALUE A FUNC PARAM RETURNED IN PLACE (#381) =====
//
// A goroutine that captures a string or an int that a func parameter
// returned does not run the parameter. Only a captured func, a slice of
// funcs, or a task counts.

func carriedStore(context.Context, string) error { return nil }

// [GOOD]: errgroup goroutine captures a string its param returned
func goodCarriedString(ctx context.Context, items []string, keyFn func(string) string) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, it := range items {
		key := keyFn(it)
		g.Go(func() error { return carriedStore(ctx, key) })
	}
	return g.Wait()
}

// [GOOD]: go statement on a literal that captures an int its param returned
func goodCarriedInt(compute func() int) {
	n := compute()
	go func() { shapeUse(n) }()
}

// [GOOD]: The captured int is a variable captured by reference
func goodCarriedIntByRef(compute func() int) {
	n := compute()
	n++
	go func() { shapeUse(n) }()
}

// [BAD]: go statement on a literal that captures a func its param returned
func badCarriedFunc(factory func() func()) { // want `function "badCarriedFunc" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	fn := factory()
	go func() { fn() }()
}

// [BAD]: The captured func is a variable captured by reference
func badCarriedFuncByRef(factory func() func()) { // want `function "badCarriedFuncByRef" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	fn := shapeWork
	fn = factory()
	go func() { fn() }()
}

// [BAD]: go statement on a literal that captures funcs its param returned
func badCarriedFuncs(factory func() []func()) { // want `function "badCarriedFuncs" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	fns := factory()
	go func() {
		for _, fn := range fns {
			fn()
		}
	}()
}

// [BAD]: go statement on a literal that captures a task made of its param
func badCarriedTask(ctx context.Context, fn func(context.Context) error) { // want `function "badCarriedTask" should have //goroutinectx:spawner directive \(runs func argument in go statement\)`
	task := gotask.NewTask(fn)
	go func() { _ = task.Do(ctx) }()
}

// ===== A FUNC PARAM WHOSE CALL GIVES BACK A FUNC (#382) =====
//
// A labeled function keeps its label when a func that comes from its func
// parameter may run in a goroutine the checker cannot see.

type carriedJob struct{ run func() }

// [GOOD]: Runs a func its param gives back in a field
//
//goroutinectx:spawner
func goodCarriedField(newJob func() carriedJob) {
	go newJob().run()
}

// [GOOD]: Runs each func its range-over-func param yields
//
//goroutinectx:spawner
func goodCarriedRangeFunc(seq iter.Seq[func()]) {
	for f := range seq {
		go f()
	}
}

// [GOOD]: Runs a func its param gives back in a map
//
//goroutinectx:spawner
func goodCarriedMap(get func() map[string]func()) {
	go get()["a"]()
}

// [GOOD]: Runs a func its param gives back through a channel
//
//goroutinectx:spawner
func goodCarriedChan(get func() chan func()) {
	go (<-get())()
}

// [GOOD]: Gives its param a callback that the param may call with a func
//
//goroutinectx:spawner
func goodCarriedYield(each func(func(func()))) {
	each(func(f func()) { go f() })
}

// [BAD]: Labeled, but its param only gives back an int in place
//
//goroutinectx:spawner
func unnecessaryCarriedInt(compute func(int) int) { // want `function "unnecessaryCarriedInt" has unnecessary //goroutinectx:spawner directive`
	shapeUse(compute(1))
}

// [BAD]: Labeled, but its param gives back a struct without a func
//
//goroutinectx:spawner
func unnecessaryCarriedStruct(get func() struct{ n int }) { // want `function "unnecessaryCarriedStruct" has unnecessary //goroutinectx:spawner directive`
	shapeUse(get().n)
}

// [GOOD]: Its param gives back an any, which may hold a func
//
//goroutinectx:spawner
func goodCarriedAny(get func() any) {
	_ = get()
}

// [GOOD]: Its param gives back an array of funcs
//
//goroutinectx:spawner
func goodCarriedArray(get func() [2]func()) {
	_ = get()
}

// [BAD]: Labeled, but its param gives back an int and an error in place
//
//goroutinectx:spawner
func unnecessaryCarriedTuple(get func() (int, error)) { // want `function "unnecessaryCarriedTuple" has unnecessary //goroutinectx:spawner directive`
	n, _ := get()
	shapeUse(n)
}

type carriedNode struct {
	next *carriedNode
	ns   []int
	ids  [2]int
	tags map[string]int
	done chan int
}

// [BAD]: Labeled, but its param gives back a list of nodes without a func
//
//goroutinectx:spawner
func unnecessaryCarriedNode(get func() *carriedNode) { // want `function "unnecessaryCarriedNode" has unnecessary //goroutinectx:spawner directive`
	_ = get()
}

// [GOOD]: Its param gives back a func beside an error
//
//goroutinectx:spawner
func goodCarriedTupleFunc(get func() (func(), error)) {
	_, _ = get()
}
