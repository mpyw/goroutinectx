package ctxkind

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/mpyw/goroutinectx/internal/carrier"
	"github.com/mpyw/goroutinectx/internal/typeutil"
)

// Kind is a set of the ways a type is, or leads to, a context. A type can be
// several at once: an interface that embeds context.Context and is given in
// -context-carriers is a Context, a Carrier and an Implements.
type Kind uint8

// The kinds. A caller asks for a set of them, as one of the thresholds
// below.
const (
	// None is the empty set.
	None Kind = 0
	// Context is context.Context, through pointers and aliases, an interface
	// that embeds it, named or not, and a type parameter whose constraint is
	// or embeds it.
	Context Kind = 1 << (iota - 1)
	// Carrier is a type of -context-carriers, matched as [carrier.Carrier.Matches] does.
	Carrier
	// Implements is a type that, past its pointers, implements
	// context.Context, as a struct that embeds one does.
	Implements
	// Holds is a struct with a field the analyzed code can set that is
	// Captured or Holds, by value. A struct it points to is not looked into.
	// The field must be exported, or declared in the package given to
	// [Classifier.From], or in a package of the module.
	Holds
)

// The thresholds the callers pick. They do not agree yet (#210).
const (
	// Param is what a parameter, an argument or an operand counts as: a
	// context or a carrier.
	Param = Context | Carrier
	// Captured is what a value a closure captures or reads counts as, and
	// what a method called on one must return: also a type that implements
	// context.Context.
	Captured = Context | Carrier | Implements
)

const (
	contextPkgPath = "context"
	xnetPkgPath    = "golang.org/x/net/context"
)

// Classifier tells the kind of a type with what one pass configures. Build
// it once for each pass with [New].
type Classifier struct {
	carriers []carrier.Carrier
	// iface is context.Context, or nil when the program has no context
	// package, so that nothing implements it.
	iface *types.Interface
	// module is the path of the analyzed package's module, or "".
	module string
	// from is the package whose code Holds is judged for, or nil.
	from *types.Package
}

// New builds the Classifier of a pass. prog is its SSA program, carriers are
// -context-carriers, and module is the path of the analyzed package's
// module, or "" when the driver does not give it.
func New(prog *ssa.Program, carriers []carrier.Carrier, module string) *Classifier {
	return &Classifier{carriers: carriers, iface: contextInterface(prog), module: module}
}

// From gives c judging Holds for code in pkg, nil for code of no package, as
// a wrapper SSA makes.
func (c *Classifier) From(pkg *types.Package) *Classifier {
	from := *c
	from.from = pkg
	return &from
}

// Kind gives every kind t is.
func (c *Classifier) Kind(t types.Type) Kind {
	var k Kind
	for _, one := range []Kind{Context, Carrier, Implements, Holds} {
		if c.Is(t, one) {
			k |= one
		}
	}
	return k
}

// Is reports whether t is any of the kinds in k. Only the kinds in k are
// looked into.
func (c *Classifier) Is(t types.Type, k Kind) bool {
	return c.is(t, k, nil)
}

func (c *Classifier) is(t types.Type, k Kind, seen map[types.Type]bool) bool {
	return (k&Context != 0 && isContext(t)) ||
		(k&Carrier != 0 && carrier.IsCarrierType(t, c.carriers)) ||
		(k&Implements != 0 && c.implements(t)) ||
		(k&Holds != 0 && c.holds(t, seen))
}

// HasParam reports whether fn has a parameter of a kind in k. It reads the
// SSA parameters, so unlike [Classifier.ParamNames] it counts a blank
// parameter, as func(_ context.Context) has, and the receiver of a method.
func (c *Classifier) HasParam(fn *ssa.Function, k Kind) bool {
	return slices.ContainsFunc(fn.Params, func(p *ssa.Parameter) bool { return c.Is(p.Type(), k) })
}

// ParamNames gives the names of the parameters of fnType of a kind in k, in
// order. It reads the syntax, so unlike [Classifier.HasParam] it leaves out
// a blank parameter, which cannot be used, and a receiver, which is not in
// fnType.
func (c *Classifier) ParamNames(info *types.Info, fnType *ast.FuncType, k Kind) []string {
	if fnType == nil || fnType.Params == nil {
		return nil
	}
	var names []string
	for _, field := range fnType.Params.List {
		typ := info.TypeOf(field.Type)
		if typ == nil || !c.Is(typ, k) {
			continue
		}
		for _, name := range field.Names {
			if name.Name != "_" {
				names = append(names, name.Name)
			}
		}
	}
	return names
}

// Involves reports whether any of vs is built from a value of a kind in k:
// one is that value, or it is among the operands they are made from, however
// deep, as a closure that captures one.
func (c *Classifier) Involves(k Kind, vs ...ssa.Value) bool {
	seen := make(map[ssa.Value]bool)
	return slices.ContainsFunc(vs, func(v ssa.Value) bool { return c.involves(v, k, seen) })
}

func (c *Classifier) involves(v ssa.Value, k Kind, seen map[ssa.Value]bool) bool {
	if v == nil || seen[v] {
		return false
	}
	seen[v] = true
	if c.Is(v.Type(), k) {
		return true
	}
	instr, ok := v.(ssa.Instruction)
	if !ok {
		return false
	}
	for _, op := range instr.Operands(nil) {
		if op != nil && c.involves(*op, k, seen) {
			return true
		}
	}
	return false
}

// MayHold reports whether v may hold a value of a kind in k. A function
// holds what it captures: nothing for a function that is not a closure, and
// its bindings for a closure or a bound method. A function value that cannot
// be followed may hold one. Any other value may hold one unless its type
// cannot: a basic type holds none, and a pointer, a slice, an array, a
// channel, a map or a struct holds what its elements or fields may. An
// interface, a function, a type parameter or an unsafe.Pointer may hold one
// (#192).
func (c *Classifier) MayHold(v ssa.Value, k Kind) bool {
	if _, isFunc := v.Type().Underlying().(*types.Signature); !isFunc {
		return c.typeMayHold(v.Type(), k, make(map[types.Type]bool))
	}
	switch v := v.(type) {
	case *ssa.Function, *ssa.Const:
		return false
	case *ssa.MakeClosure:
		return slices.ContainsFunc(v.Bindings, func(b ssa.Value) bool { return c.MayHold(b, k) })
	}
	return true
}

func (c *Classifier) typeMayHold(t types.Type, k Kind, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	if c.Is(t, k) {
		return true
	}
	elem := func(e types.Type) bool { return c.typeMayHold(e, k, seen) }
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Kind() == types.UnsafePointer
	case *types.Pointer:
		return elem(u.Elem())
	case *types.Slice:
		return elem(u.Elem())
	case *types.Array:
		return elem(u.Elem())
	case *types.Chan:
		return elem(u.Elem())
	case *types.Map:
		return elem(u.Key()) || elem(u.Elem())
	case *types.Struct:
		for f := range u.Fields() {
			if elem(f.Type()) {
				return true
			}
		}
		return false
	}
	return true
}

// implements reports whether t, past its pointers, implements
// context.Context.
func (c *Classifier) implements(t types.Type) bool {
	if c.iface == nil {
		return false
	}
	t = typeutil.UnwrapPointer(t)
	return types.Implements(t, c.iface) || types.Implements(types.NewPointer(t), c.iface)
}

// holds reports whether t is a struct with a field the analyzed code can
// set that is Captured or Holds. Only such a field counts, so the unexported
// context of a *testing.T or an *http.Request does not (#191).
func (c *Classifier) holds(t types.Type, seen map[types.Type]bool) bool {
	if seen == nil {
		seen = make(map[types.Type]bool)
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for f := range st.Fields() {
		if c.canSet(f) && c.is(f.Type(), Captured|Holds, seen) {
			return true
		}
	}
	return false
}

// canSet reports whether the analyzed code can set the field f: it is
// exported, or declared in the package of From or a package of the module.
// When the module is not known, a field outside the standard library may be
// of the module, so it counts, as a report there could be false.
func (c *Classifier) canSet(f *types.Var) bool {
	if f.Exported() || f.Pkg() == c.from {
		return true
	}
	// An unexported field always has a package.
	path := f.Pkg().Path()
	if c.module == "" {
		return !isStdPath(path)
	}
	return path == c.module || strings.HasPrefix(path, c.module+"/")
}

// IsFreshCall reports whether fn is Background or TODO of package context or
// golang.org/x/net/context: a call of it gives a context that carries
// nothing of the caller's.
func IsFreshCall(fn *types.Func) bool {
	return fn != nil && fn.Pkg() != nil &&
		(fn.Pkg().Path() == contextPkgPath || fn.Pkg().Path() == xnetPkgPath) &&
		(fn.Name() == "Background" || fn.Name() == "TODO")
}

// IsWithCall reports whether fn is a function of package context, not
// golang.org/x/net/context, that makes a context from the Context it is
// given first, as context.WithCancel and context.WithValue do.
func IsWithCall(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == contextPkgPath && strings.HasPrefix(fn.Name(), "With") &&
		sig.Recv() == nil && sig.Params().Len() > 0 && isContext(sig.Params().At(0).Type())
}

// isContext reports whether t is a Context: context.Context, through
// pointers and aliases, an interface that embeds it, named or not, or a type
// parameter whose constraint is or embeds it (see [typeutil.Embeds]).
func isContext(t types.Type) bool {
	return typeutil.Embeds(t, isContextNamed)
}

func isContextNamed(t types.Type) bool {
	named, ok := typeutil.UnwrapPointer(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == contextPkgPath && obj.Name() == "Context"
}

// contextInterface gives the context.Context interface, or nil when prog
// has no context package.
func contextInterface(prog *ssa.Program) *types.Interface {
	pkg := prog.ImportedPackage(contextPkgPath)
	if pkg == nil {
		return nil
	}
	obj := pkg.Pkg.Scope().Lookup("Context")
	if obj == nil {
		return nil
	}
	iface, _ := obj.Type().Underlying().(*types.Interface)
	return iface
}

// isStdPath reports whether path is of the standard library: its first
// element has no dot, as "testing" and "net/http".
func isStdPath(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
