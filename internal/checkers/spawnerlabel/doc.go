// Package spawnerlabel checks that //goroutinectx:spawner labels the
// functions that spawn, and only those. It runs with -spawnerlabel.
//
// # Missing Label
//
// A function needs the label when it calls a supported API, such as
// errgroup.Group.Go, with a func argument, or calls another spawner with one,
// or calls gotask's DoAsync:
//
//	func runTask(task func() error) { // should have //goroutinectx:spawner
//	    g := new(errgroup.Group)
//	    g.Go(task)
//	    _ = g.Wait()
//	}
//
// The calls made in the closures the function runs count too: IIFEs, a
// closure called through a variable, the goroutines it starts, and the
// closures a call returns when that result is called. A closure it only
// makes, and does not run, does not count. A go statement on its own, as
// go fn(), is not such a call.
//
// # Unnecessary Label
//
// A labeled function needs it only while it makes such a call or takes a
// func parameter, the arguments the spawner checker checks. A type parameter
// whose constraint allows only func types, as F ~func() does, is one:
//
//	//goroutinectx:spawner // unnecessary
//	func simpleHelper() {
//	    fmt.Println("hello")
//	}
//
// # Integration
//
// The checker runs over the whole pass, after the per-node checkers:
//
//	diags := spawnerlabel.New(spawners, ssaProg, f).Check(pass, skipFiles)
package spawnerlabel
