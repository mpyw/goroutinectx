package flow

import (
	"golang.org/x/tools/go/ssa"
)

// returnedResults is which results of a return returns gives.
type returnedResults int

//declscope:shared // flow.go gives the first result, run.go every result
const (
	// returnedFirst gives the first result of each return. A factory is
	// followed this way, as func() (func(), error) gives the function first.
	returnedFirst returnedResults = iota
	// returnedEvery gives every result of each return. spawnerlabel finds
	// the closures a call returns this way, so a closure returned second
	// counts too.
	returnedEvery
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
		case results == returnedFirst:
			returned = append(returned, ret.Results[0])
		default:
			returned = append(returned, ret.Results...)
		}
	}
	return returned
}
