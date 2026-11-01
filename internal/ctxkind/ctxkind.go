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
	// Holds is a struct, or a pointer to one, with a field the analyzed code
	// can set that is Captured, or a struct that is Holds by value. A struct
	// a field points to is not looked into. The field must be exported, or
	// declared in the analyzed package, the package given to
	// [Classifier.From], or a package of the module.
	Holds
)

// The thresholds the callers pick (#210). A value a goroutine gets counts
// by the wide sets, Captured and Given, as when in doubt a goroutine is not
// reported. What puts a function under check is the narrow set, Param, as
// checking more functions means more reports.
const (
	// Param is what a parameter of a function must be for the goroutines in
	// it to be checked against it, and named in a report: a context or a
	// carrier. A type that only implements context.Context, as a
	// *gin.Context does, is left out, so a handler that takes one is not
	// checked.
	Param = Context | Carrier
	// Captured is what a value a closure captures or reads counts as, and
	// what a method called on one must return: also a type that implements
	// context.Context.
	Captured = Context | Carrier | Implements
	// Given is what a parameter of a spawned function or a factory, or an
	// argument given to one, counts as: what Captured counts, and a struct
	// that holds one, as a closure that gives a captured value to a call
	// counts it ([Holds]).
	Given = Captured | Holds
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
	// pkg is the analyzed package, or nil.
	pkg *types.Package
	// from is the package of the code Holds is judged for, or nil.
	from *types.Package
}

// New builds the Classifier of a pass. prog is its SSA program, carriers are
// -context-carriers, module is the path of the analyzed package's module,
// or "" when the driver does not give it, and pkg is the analyzed package.
func New(prog *ssa.Program, carriers []carrier.Carrier, module string, pkg *types.Package) *Classifier {
	return &Classifier{carriers: carriers, iface: contextInterface(prog), module: module, pkg: pkg}
}

// From gives c judging Holds for code in pkg too, as a closure of another
// package that a factory returns. pkg is nil for code of no package, as a
// wrapper SSA makes, which is judged for the analyzed package alone.
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
		(k&Holds != 0 && c.holds(typeutil.UnwrapPointer(t), seen))
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
// deep, as a closure that captures one. A value put into a variable, a
// struct, an array, a slice, a map or a channel they are made from counts
// too, as a context in the struct literal of ctxRunner{ctx} converted to an
// interface, or a closure sent on a channel or set in a map literal (#240).
func (c *Classifier) Involves(k Kind, vs ...ssa.Value) bool {
	return c.InvolvesUnless(k, nil, vs...)
}

// InvolvesUnless is [Classifier.Involves], but a value for which skip
// reports true does not count, and is not looked into, as a fresh
// context.Background() given to a goroutine as an any (#252). skip may be
// nil.
func (c *Classifier) InvolvesUnless(k Kind, skip func(ssa.Value) bool, vs ...ssa.Value) bool {
	in := involving{c: c, k: k, skip: skip, seen: make(map[ssa.Value]bool)}
	return slices.ContainsFunc(vs, in.involves)
}

// involving is one question [Classifier.InvolvesUnless] answers.
type involving struct {
	c    *Classifier
	k    Kind
	skip func(ssa.Value) bool
	seen map[ssa.Value]bool
}

func (in *involving) involves(v ssa.Value) bool {
	if v == nil || in.seen[v] {
		return false
	}
	in.seen[v] = true
	if in.skip != nil && in.skip(v) {
		return false
	}
	if in.c.Is(v.Type(), in.k) {
		return true
	}
	instr, ok := v.(ssa.Instruction)
	if !ok {
		return false
	}
	for _, op := range instr.Operands(nil) {
		if op != nil && in.involves(*op) {
			return true
		}
	}
	switch v.(type) {
	case *ssa.Alloc, *ssa.MakeSlice, *ssa.MakeMap, *ssa.MakeChan:
		return in.involvesPut(v)
	}
	return false
}

// involvesPut reports whether a value put into container, or into a field
// or an element of it, however deep, involves a value of a kind in k.
func (in *involving) involvesPut(container ssa.Value) bool {
	return slices.ContainsFunc(*container.Referrers(), func(ref ssa.Instruction) bool {
		switch x := ref.(type) {
		case *ssa.Store:
			return x.Addr == container && in.involves(x.Val)
		case *ssa.FieldAddr:
			return x.X == container && in.involvesPut(x)
		case *ssa.IndexAddr:
			return x.X == container && in.involvesPut(x)
		case *ssa.MapUpdate:
			return x.Map == container && (in.involves(x.Key) || in.involves(x.Value))
		case *ssa.Send:
			return x.Chan == container && in.involves(x.X)
		}
		return false
	})
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

// HasContextField reports whether t, past its pointers, is a struct with a
// field of [Captured], of any package and exported or not, or a field of a
// struct by value that has one. Unlike [Holds], a field the analyzed code
// cannot set counts too, as the unexported context of an *exec.Cmd or an
// *http.Request: a call given ctx that makes such a value has put ctx there
// (#257). A struct a field points to is not looked into.
func (c *Classifier) HasContextField(t types.Type) bool {
	return c.hasContextField(typeutil.UnwrapPointer(t), make(map[types.Type]bool))
}

func (c *Classifier) hasContextField(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for f := range st.Fields() {
		if c.Is(f.Type(), Captured) || c.hasContextField(types.Unalias(f.Type()), seen) {
			return true
		}
	}
	return false
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
// set that is Captured, or a struct that holds one. Only such a field
// counts, so the unexported context of a *testing.T or an *http.Request
// does not (#191).
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
		if c.canSet(f) && (c.Is(f.Type(), Captured) || c.holds(f.Type(), seen)) {
			return true
		}
	}
	return false
}

// canSet reports whether the analyzed code can set the field f: it is
// exported, or declared in the analyzed package, the package of From or a
// package of the module.
// When the module is not known, a field outside the standard library may be
// of the module, so it counts, as a report there could be false.
func (c *Classifier) canSet(f *types.Var) bool {
	// An unexported field always has a package.
	return f.Exported() || c.ofModule(f.Pkg())
}

// DeclaredInModule reports whether t is a named type, or an alias of one,
// declared in the analyzed package, the package of From or a package of
// the module, as a field
// is judged for Holds (#191). When the module is not known, a type outside
// the standard library may be of the module, so it counts. A type with no
// package, as error or any, does not. An interface of such a type that
// cannot be followed may hold a value the analyzed code made with a
// context, and one of the standard library, as net.Conn, is taken to hold
// none (#240).
func (c *Classifier) DeclaredInModule(t types.Type) bool {
	var obj *types.TypeName
	switch n := t.(type) {
	case *types.Named:
		obj = n.Obj()
	case *types.Alias:
		obj = n.Obj()
	default:
		return false
	}
	return obj.Pkg() != nil && c.ofModule(obj.Pkg())
}

// ofModule reports whether pkg is the analyzed package, the package of From
// or a package of the module, or, when the module is not known, outside the
// standard library. pkg is not nil.
func (c *Classifier) ofModule(pkg *types.Package) bool {
	if pkg == c.pkg || pkg == c.from {
		return true
	}
	path := pkg.Path()
	if c.module == "" {
		return !isStdPath(path)
	}
	return path == c.module || strings.HasPrefix(path, c.module+"/")
}

// IsFreshCall reports whether fn is Background or TODO of package context or
// golang.org/x/net/context: a call of it gives a context that carries
// nothing of the caller's.
func IsFreshCall(fn *types.Func) bool {
	return fn != nil && isContextPkg(fn.Pkg()) && (fn.Name() == "Background" || fn.Name() == "TODO")
}

// isContextPkg reports whether pkg is context or golang.org/x/net/context.
func isContextPkg(pkg *types.Package) bool {
	return pkg != nil && (pkg.Path() == contextPkgPath || pkg.Path() == xnetPkgPath)
}

// IsWithCall reports whether fn is a function of package context or
// golang.org/x/net/context that makes a context from the Context it is
// given first, as context.WithCancel and context.WithValue do. The With
// functions of x/net/context call those of context, as [IsFreshCall] takes
// its Background and TODO.
func IsWithCall(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	return ok && isContextPkg(fn.Pkg()) && strings.HasPrefix(fn.Name(), "With") &&
		sig.Recv() == nil && sig.Params().Len() > 0 && isContext(sig.Params().At(0).Type())
}

// IsCancelCall reports whether fn is a function of package context or
// golang.org/x/net/context that makes a context from the Context it is
// given first, as [IsWithCall] tells, and gives a func that cancels it
// second: WithCancel, WithCancelCause, WithTimeout, WithDeadline and their
// Cause forms.
func IsCancelCall(fn *types.Func) bool {
	if fn == nil || !IsWithCall(fn) {
		return false
	}
	results := fn.Type().(*types.Signature).Results()
	if results.Len() != 2 {
		return false
	}
	_, isFunc := results.At(1).Type().Underlying().(*types.Signature)
	return isFunc
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
