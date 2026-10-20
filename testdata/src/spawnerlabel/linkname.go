package spawnerlabel

import _ "unsafe"

// nanotime has no body: it is linked to the runtime's. A function with no
// body is not checked.
//
//go:linkname nanotime runtime.nanotime
func nanotime() int64
