package checkers

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/checkername"
	"github.com/mpyw/goroutinectx/internal/directive/spawner"
	"github.com/mpyw/goroutinectx/internal/flow"
	"github.com/mpyw/goroutinectx/internal/registry"
)

// SpawnerlabelChecker validates that functions are properly labeled with
// //goroutinectx:spawner.
type SpawnerlabelChecker struct {
	spawners *spawner.Map
	ssaProg  *flow.Program
	flow     *flow.Flow
}

// newSpawnerlabelChecker creates a new spawnerlabel checker. f follows the
// values called, to the closures the calls of factories return.
//
//declscope:shared // Defs in checkers.go builds the checker with it
func newSpawnerlabelChecker(spawners *spawner.Map, ssaProg *flow.Program, f *flow.Flow) *SpawnerlabelChecker {
	return &SpawnerlabelChecker{spawners: spawners, ssaProg: ssaProg, flow: f}
}

// Name returns the checker name.
func (*SpawnerlabelChecker) Name() checkername.Name {
	return checkername.Spawnerlabel
}

// spawnerlabelCall is a call that runs a func parameter of the function
// checked in a goroutine.
type spawnerlabelCall struct {
	// methodName is the API called, as "errgroup.Group.Go", or "" for a go
	// statement.
	methodName string
}

// message gives the report of the function named name.
func (c *spawnerlabelCall) message(name string) string {
	if c.methodName == "" {
		return fmt.Sprintf(messageLabelMissingGo, name)
	}
	return fmt.Sprintf(messageLabelMissing, name, c.methodName)
}

// spawnerlabelIsFuncType reports whether t is a func type, or a type
// parameter whose constraint allows only func types, as F ~func() does.
func spawnerlabelIsFuncType(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Signature:
		return true
	case *types.Interface:
		if _, isParam := t.(*types.TypeParam); !isParam {
			return false
		}
		return spawnerlabelConstraintIsFunc(u)
	}
	return false
}

// spawnerlabelConstraintIsFunc reports whether the constraint iface allows
// only func types: one of its embedded types or unions does.
func spawnerlabelConstraintIsFunc(iface *types.Interface) bool {
	for i := range iface.NumEmbeddeds() {
		switch e := iface.EmbeddedType(i).(type) {
		case *types.Union:
			allFunc := e.Len() > 0
			for j := range e.Len() {
				if _, isFunc := e.Term(j).Type().Underlying().(*types.Signature); !isFunc {
					allFunc = false
				}
			}
			if allFunc {
				return true
			}
		default:
			if inner, ok := e.Underlying().(*types.Interface); ok {
				if spawnerlabelConstraintIsFunc(inner) {
					return true
				}
			} else if _, isFunc := e.Underlying().(*types.Signature); isFunc {
				return true
			}
		}
	}
	return false
}

// Check runs the spawnerlabel analysis on the given pass, and gives its
// reports.
func (c *SpawnerlabelChecker) Check(pass *analysis.Pass, skipFiles map[string]bool) []analysis.Diagnostic {
	var diags []analysis.Diagnostic
	for _, file := range pass.Files {
		filename := pass.Fset.PositionFor(file.Pos(), false).Filename
		if skipFiles[filename] {
			continue
		}

		for _, decl := range file.Decls {
			fnDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Skip functions without body (interface methods, external)
			if fnDecl.Body == nil {
				continue
			}

			diags = append(diags, c.checkFunction(pass, fnDecl)...)
		}
	}
	return diags
}

// checkFunction checks a single function declaration. A function needs the
// label when a func value it runs in a goroutine, or passes to a spawner,
// comes from one of its own func parameters. A marked function that runs
// none of them so, and only calls them in place or leaves them unused,
// does not.
func (c *SpawnerlabelChecker) checkFunction(pass *analysis.Pass, fnDecl *ast.FuncDecl) []analysis.Diagnostic {
	fn := c.getFuncObject(pass, fnDecl)
	ssaFn := spawnerlabelSSAFunc(c.ssaProg, fnDecl)
	if fn == nil || ssaFn == nil {
		return nil
	}

	isMarked := c.spawners.IsSpawner(fn)
	params := spawnerlabelFuncParams(ssaFn)
	spawnInfo := c.findSpawnCall(ssaFn, params)

	// Check for missing label
	if !isMarked && spawnInfo != nil {
		return []analysis.Diagnostic{{
			Pos:     fnDecl.Name.Pos(),
			Message: spawnInfo.message(fnDecl.Name.Name),
		}}
	}

	// Check for unnecessary label
	if isMarked && spawnInfo == nil && spawnerlabelOnlyCalled(params) {
		return []analysis.Diagnostic{{
			Pos:     fnDecl.Name.Pos(),
			Message: fmt.Sprintf(messageLabelUnneeded, fnDecl.Name.Name),
		}}
	}

	return nil
}

// getFuncObject gets the *types.Func for a function declaration.
func (c *SpawnerlabelChecker) getFuncObject(pass *analysis.Pass, fnDecl *ast.FuncDecl) *types.Func {
	fn, _ := pass.TypesInfo.ObjectOf(fnDecl.Name).(*types.Func)
	return fn
}

// findSpawnCall searches fn for a call that runs one of params in a
// goroutine. It goes into the closures the function runs: IIFEs, closures
// called through a variable, the goroutines it starts, and the closures a
// call returns when that result is called. A closure only made, and never
// run here, does not count.
func (c *SpawnerlabelChecker) findSpawnCall(fn *ssa.Function, params map[ssa.Value]bool) *spawnerlabelCall {
	if len(params) == 0 {
		return nil
	}
	o := &spawnerlabelOrigin{flow: c.flow, params: params, seen: make(map[ssa.Value]bool)}
	for site := range flow.Runs(fn, flow.RunOptions{Goroutines: true, Returned: true, Flow: c.flow}) {
		if info := c.spawnCallInfoOf(site, o); info != nil {
			return info
		}
	}
	return nil
}

// spawnCallInfoOf reports the API a call runs a func parameter in a
// goroutine through, or nil. A go statement counts with the value it runs
// and the arguments it passes.
func (c *SpawnerlabelChecker) spawnCallInfoOf(site flow.RunCall, o *spawnerlabelOrigin) *spawnerlabelCall {
	call := site.Call
	if site.IsGo() {
		if (!call.IsInvoke() && o.fromParam(call.Value)) || o.anyArg(call, 0) {
			return &spawnerlabelCall{}
		}
		return nil
	}

	// Get the called function
	calledFn := flow.ExtractCalledFunc(call)
	if calledFn == nil {
		return nil
	}

	// Check against registry. A task API, as DoAsync, runs its receiver,
	// so every argument counts.
	if entry, ok := registry.Match(calledFn); ok {
		start := entry.CallbackArgIdx
		if entry.AlwaysSpawns {
			start = 0
		}
		if o.anyArg(call, start) {
			return &spawnerlabelCall{methodName: entry.Spec.FullName()}
		}
	}

	// Check if calling a spawner-marked function
	if c.spawners.IsSpawner(calledFn) && o.anyArg(call, 0) {
		return &spawnerlabelCall{methodName: calledFn.Name()}
	}

	return nil
}

// spawnerlabelSSAFunc finds the SSA function of decl in p, or nil.
func spawnerlabelSSAFunc(p *flow.Program, decl *ast.FuncDecl) *ssa.Function {
	if p == nil || decl == nil {
		return nil
	}

	for _, fn := range p.SrcFuncs {
		syntax := fn.Syntax()
		if syntax == nil {
			continue
		}
		if fnDecl, ok := syntax.(*ast.FuncDecl); ok && fnDecl.Pos() == decl.Pos() {
			return fn
		}
	}
	return nil
}

// spawnerlabelFuncParams gives the func parameters of fn: those of a func
// type, of a slice of one as a variadic ...func() is, or of a type
// parameter that allows only func types. The receiver is not one.
func spawnerlabelFuncParams(fn *ssa.Function) map[ssa.Value]bool {
	params := fn.Params
	if fn.Signature.Recv() != nil && len(params) > 0 {
		params = params[1:]
	}
	funcs := make(map[ssa.Value]bool)
	for _, p := range params {
		t := p.Type()
		if slice, ok := t.Underlying().(*types.Slice); ok {
			t = slice.Elem()
		}
		if spawnerlabelIsFuncType(t) {
			funcs[p] = true
		}
	}
	return funcs
}

// spawnerlabelOnlyCalled reports whether each of params is only called in
// place, or not used at all. Any other use, as storing it or passing it on,
// may run it in a goroutine the checker cannot see.
func spawnerlabelOnlyCalled(params map[ssa.Value]bool) bool {
	for p := range params {
		for _, ref := range *p.Referrers() {
			switch r := ref.(type) {
			case *ssa.DebugRef:
				continue
			case *ssa.Call:
				if r.Call.Value == p && !slices.Contains(r.Call.Args, p) {
					continue
				}
			}
			return false
		}
	}
	return true
}

// spawnerlabelOrigin tells whether a value comes from one of the func
// parameters of the function checked.
type spawnerlabelOrigin struct {
	flow   *flow.Flow
	params map[ssa.Value]bool
	// seen are the values asked of, so that a cycle of closures ends.
	seen map[ssa.Value]bool
}

// anyArg reports whether an argument of call, at start or after, comes from
// a func parameter, or an element of one of a slice.
func (o *spawnerlabelOrigin) anyArg(call *ssa.CallCommon, start int) bool {
	for i := start; i < len(call.Args); i++ {
		arg := call.Args[i]
		if o.fromParam(arg) {
			return true
		}
		if _, isSlice := arg.Type().Underlying().(*types.Slice); !isSlice {
			continue
		}
		elems, _ := o.flow.ResolvedSliceElems(arg)
		if slices.ContainsFunc(elems, o.fromParam) {
			return true
		}
	}
	return false
}

// fromParam reports whether v comes from a func parameter: it is one, or an
// element of one, or a closure that captures one, or the result of a call
// of one or given one. It follows v as [flow.Flow.CapturedValues] and
// [flow.Flow.Sources] do.
func (o *spawnerlabelOrigin) fromParam(v ssa.Value) bool {
	if o.seen[v] {
		return false
	}
	o.seen[v] = true
	for _, x := range o.flow.CapturedValues(v) {
		if o.params[x] {
			return true
		}
		// An element of a slice parameter, as range over fns reads.
		if load, ok := x.(*ssa.UnOp); ok && load.Op == token.MUL {
			if elem, ok := load.X.(*ssa.IndexAddr); ok && o.fromParam(elem.X) {
				return true
			}
		}
		sources, _ := o.flow.Sources(x)
		if slices.ContainsFunc(sources, o.fromSource) {
			return true
		}
	}
	return false
}

// fromSource reports whether s comes from a func parameter, as fromParam
// tells it.
func (o *spawnerlabelOrigin) fromSource(s flow.Source) bool {
	switch s.Kind {
	case flow.SourceParam:
		return o.params[s.Value]
	case flow.SourceLiteral:
		// A literal that captures nothing is a function, not a closure.
		mc, isClosure := s.Value.(*ssa.MakeClosure)
		if !isClosure {
			return false
		}
		return slices.ContainsFunc(mc.Fn.(*ssa.Function).FreeVars, func(fv *ssa.FreeVar) bool { return o.fromParam(fv) })
	case flow.SourceCall:
		call := &s.Call().Call
		return o.fromParam(call.Value) || o.anyArg(call, 0)
	}
	return false
}
