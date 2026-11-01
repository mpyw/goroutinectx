package flow

import (
	"golang.org/x/tools/go/ssa"
)

// returnedResults is which results of a return returns gives: the one at
// that index, or every result.
//
//declscope:shared // flow.go asks for the result at an index
type returnedResults int

//declscope:shared // flow.go gives the result at an index, run.go every result
const (
	// returnedFirst gives the first result of each return. A factory is
	// followed this way, as func() (func(), error) gives the function first.
	returnedFirst returnedResults = 0
	// returnedEvery gives every result of each return. spawnerlabel finds
	// the closures a call returns this way, so a closure returned second
	// counts too.
	returnedEvery returnedResults = -1
)

// returns gives the values fn returns, in block order: the results of each
// Return instruction that results picks.
//
//declscope:shared // flow.go gives what a factory returns, run.go returned closures
func returns(fn *ssa.Function, results returnedResults) []ssa.Value {
	var returned []ssa.Value
	for _, block := range fn.Blocks {
		// A return ends its block.
		ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return)
		switch {
		case !ok || len(ret.Results) == 0:
		case results == returnedEvery:
			returned = append(returned, ret.Results...)
		default:
			returned = append(returned, ret.Results[results])
		}
	}
	return returned
}
