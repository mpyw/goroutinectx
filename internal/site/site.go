package site

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/flow"
	"github.com/mpyw/goroutinectx/internal/funcspec"
)

// Index maps the nodes of one package to their SSA form.
type Index struct {
	pass *analysis.Pass
	// flow follows a function value a call calls, as [Index.called] does.
	flow *flow.Flow
	// calls are the Call, Go and Defer instructions by the position of
	// their call's left parenthesis.
	calls map[token.Pos]ssa.CallInstruction
	// gos are the go instructions by the position of their go keyword.
	gos map[token.Pos]*ssa.Go
	// lits are the SSA functions of the function literals.
	lits map[*ast.FuncLit]*ssa.Function
}

// NewIndex builds the Index of pass, whose SSA program is prog. f is the
// pass's Flow.
//
// An instruction is indexed in the innermost function whose syntax holds its
// position. The syntax of the yield function of a range-over-func loop is
// only the loop's body: see [siteSyntax].
func NewIndex(pass *analysis.Pass, prog *flow.Program, f *flow.Flow) *Index {
	x := &Index{
		pass:  pass,
		flow:  f,
		calls: make(map[token.Pos]ssa.CallInstruction),
		gos:   make(map[token.Pos]*ssa.Go),
		lits:  make(map[*ast.FuncLit]*ssa.Function),
	}
	seen := make(map[*ssa.Function]bool)
	for _, fn := range prog.SrcFuncs {
		if !seen[fn] {
			x.add(fn, fn.Pos(), seen)
		}
	}
	return x
}

// add indexes fn and the closures it makes. lo is where its syntax starts,
// as a node must start to be in it.
func (x *Index) add(fn *ssa.Function, lo token.Pos, seen map[*ssa.Function]bool) {
	seen[fn] = true
	syntax := siteSyntax(fn)
	if lit, ok := syntax.(*ast.FuncLit); ok {
		if _, found := x.lits[lit]; !found {
			x.lits[lit] = fn
		}
	}
	var anons []ast.Node
	for _, anon := range fn.AnonFuncs {
		if s := siteSyntax(anon); s != nil {
			anons = append(anons, s)
			x.add(anon, s.Pos(), seen)
		}
	}
	// own reports whether pos is in fn's syntax, and in none of its
	// closures'.
	own := func(pos token.Pos) bool {
		if syntax == nil || pos < lo || syntax.End() < pos {
			return false
		}
		for _, s := range anons {
			if s.Pos() <= pos && pos <= s.End() {
				return false
			}
		}
		return true
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if g, ok := instr.(*ssa.Go); ok && own(g.Pos()) {
				if _, found := x.gos[g.Pos()]; !found {
					x.gos[g.Pos()] = g
				}
			}
			if c, ok := instr.(ssa.CallInstruction); ok && own(c.Common().Pos()) {
				if _, found := x.calls[c.Common().Pos()]; !found {
					x.calls[c.Common().Pos()] = c
				}
			}
		}
	}
}

// siteSyntax gives the syntax fn's instructions are made from. SSA gives the
// yield function of a range-over-func loop the whole *ast.RangeStmt as its
// syntax, but the function around the loop evaluates the range expression
// (#254). So the yield function covers only the loop's body.
func siteSyntax(fn *ssa.Function) ast.Node {
	syntax := fn.Syntax()
	if rng, ok := syntax.(*ast.RangeStmt); ok {
		return rng.Body
	}
	return syntax
}

// FuncLit gives the SSA function of lit, or nil.
func (x *Index) FuncLit(lit *ast.FuncLit) *ssa.Function {
	return x.lits[lit]
}

// Site is one go statement or call.
type Site struct {
	// Go is the go statement, or nil for a call.
	Go *ast.GoStmt
	// Call is the call: the one the go statement makes, or the call itself.
	Call *ast.CallExpr
	// Instr is the instruction: the *ssa.Go of a go statement, or the
	// instruction of a call, which is a Go or a Defer for one a go or defer
	// statement makes. It is nil when SSA has none, as for unreachable
	// code.
	Instr ssa.CallInstruction
	// Fn is the function a go statement runs. Its value is nil for an
	// interface method.
	Fn Spawned
	// Callee is the function a call calls, or nil. It is nil for a go
	// statement.
	Callee *types.Func
	// RecvArgs is the number of arguments a call gives before Callee's
	// parameters: 1 for a method given its receiver first, as by a method
	// expression, (*errgroup.Group).Go(g, fn), and 0 otherwise.
	RecvArgs int
	// Args are a call's arguments, each as a value it may spawn. Their
	// values are nil when SSA's do not line up with them.
	Args []Spawned
	// Recv is the receiver a call of a method gives, or nil: its first
	// argument when RecvArgs is 1, or else the receiver of a static method
	// call or of a method value ([Receiver]).
	Recv ssa.Value
}

// Spawned is a value a site may spawn.
type Spawned struct {
	// Expr is its expression.
	Expr ast.Expr
	// Value is its SSA value, or nil.
	Value ssa.Value
	// Ordinal is its position among the call's arguments, from 1, or 0 for
	// the function a go statement runs.
	Ordinal int
}

// Params gives the arguments of s after the receiver: those of Callee's
// parameters.
func (s *Site) Params() []Spawned {
	return s.Args[min(s.RecvArgs, len(s.Args)):]
}

// Common gives the call of s's instruction, or nil.
func (s *Site) Common() *ssa.CallCommon {
	if s.Instr == nil {
		return nil
	}
	return s.Instr.Common()
}

// Site gives the site of n, a go statement or a call, or nil for any other
// node.
func (x *Index) Site(n ast.Node) *Site {
	switch n := n.(type) {
	case *ast.GoStmt:
		return x.goSite(n)
	case *ast.CallExpr:
		return x.callSite(n)
	}
	return nil
}

func (x *Index) goSite(stmt *ast.GoStmt) *Site {
	s := &Site{Go: stmt, Call: stmt.Call, Fn: Spawned{Expr: stmt.Call.Fun}}
	if g := x.gos[stmt.Go]; g != nil {
		s.Instr = g
		if !g.Call.IsInvoke() {
			s.Fn.Value = g.Call.Value
		}
	}
	return s
}

func (x *Index) callSite(call *ast.CallExpr) *Site {
	s := &Site{Call: call, Args: make([]Spawned, len(call.Args))}
	if instr := x.calls[call.Lparen]; instr != nil {
		s.Instr = instr
	}
	common := x.called(s.Common())
	s.Callee, s.RecvArgs = x.callee(s, common)
	var values []ssa.Value
	if common != nil {
		values = flow.ResolvedArgs(common, call)
	}
	for i, arg := range call.Args {
		s.Args[i] = Spawned{Expr: arg, Ordinal: i + 1}
		if values != nil {
			s.Args[i].Value = values[i]
		}
	}
	if common != nil {
		s.Recv = Receiver(common)
		if s.RecvArgs == 1 && len(s.Args) > 0 && s.Args[0].Value != nil {
			s.Recv = s.Args[0].Value
		}
	}
	return s
}

// called gives common, or, when it calls a value that is not static, as a
// variable a closure captures, a copy of it that calls the one value the
// variable can hold, when that value is static: goFn of goFn := g.Go called
// in a closure calls the method value g.Go (#275). It gives nil for nil.
func (x *Index) called(common *ssa.CallCommon) *ssa.CallCommon {
	if common == nil || common.IsInvoke() || common.StaticCallee() != nil {
		return common
	}
	sources, complete := x.flow.Sources(common.Value)
	if !complete || len(sources) != 1 {
		return common
	}
	c := *common
	c.Value = sources[0].Value
	if c.StaticCallee() == nil {
		return common
	}
	return &c
}

// callee gives the function common, the call of s, calls, and its
// RecvArgs. A call through a function or method value, as gofn := wg.Go,
// or through a method expression in a variable, as gofn :=
// (*errgroup.Group).Go, is found through SSA.
func (x *Index) callee(s *Site, common *ssa.CallCommon) (*types.Func, int) {
	if fn := funcspec.ExtractFunc(x.pass, s.Call); fn != nil {
		if sel, ok := ast.Unparen(s.Call.Fun).(*ast.SelectorExpr); ok {
			if sel := x.pass.TypesInfo.Selections[sel]; sel != nil && sel.Kind() == types.MethodExpr {
				return fn, 1
			}
		}
		return fn, 0
	}
	if common == nil || common.IsInvoke() || common.StaticCallee() == nil {
		return nil, 0
	}
	fn := flow.ExtractCalledFunc(common)
	// A method value is a bound wrapper that holds the receiver. A method
	// called any other way is given the receiver first.
	if flow.Classify(common.StaticCallee()) != flow.ClassifiedBound && fn != nil && fn.Signature().Recv() != nil {
		return fn, 1
	}
	return fn, 0
}

// Receiver gives the receiver of a static method call, or of a call of a
// method value, which a bound method wrapper holds. It gives nil for any
// other call.
func Receiver(common *ssa.CallCommon) ssa.Value {
	if common.IsInvoke() {
		return nil
	}
	if mc, ok := common.Value.(*ssa.MakeClosure); ok {
		if fn, ok := mc.Fn.(*ssa.Function); ok && flow.Classify(fn) == flow.ClassifiedBound && len(mc.Bindings) == 1 {
			return mc.Bindings[0]
		}
		return nil
	}
	if common.Signature().Recv() == nil || len(common.Args) == 0 {
		return nil
	}
	return common.Args[0]
}

// Judge judges the function s can hold by p, as [flow.Judge] does. A
// function literal SSA has no value for is found by its syntax.
func (x *Index) Judge(f *flow.Flow, p flow.Policy, s Spawned) flow.Verdict {
	if lit, ok := ast.Unparen(s.Expr).(*ast.FuncLit); ok && s.Value == nil {
		if fn := x.FuncLit(lit); fn != nil {
			return p.Literal(fn, flow.SiteSpawned)
		}
	}
	if s.Value == nil {
		return flow.Unknown
	}
	return flow.Judge(f, p, s.Value)
}
