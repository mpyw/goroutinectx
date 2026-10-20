// Package spawnerlabeldoc tests -spawnerlabel with the directive above
// another doc comment line (#85).
package spawnerlabeldoc

//goroutinectx:spawner
func plain(fn func()) { go fn() }

//goroutinectx:spawner
// docAfter has the directive above another doc line.
func docAfter(fn func()) {
	plain(fn)
}

// docBefore has the directive below another doc line.
//
//goroutinectx:spawner
func docBefore(fn func()) {
	plain(fn)
}

func missing(fn func()) { // want `function "missing" should have //goroutinectx:spawner directive \(calls plain with func argument\)`
	plain(fn)
}
