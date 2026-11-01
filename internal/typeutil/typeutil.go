package typeutil

import (
	"fmt"
	"go/build"
	"go/types"
	"slices"
	"strings"
)

// Embeds reports whether t, through pointers and aliases, is a type match
// accepts, or an interface that embeds one, as type MyCtx interface{
// context.Context } does. A type parameter is judged by its constraint.
//
// Only an embedded type counts. An interface that declares the same
// methods itself is not looked into, and neither is a struct that embeds
// one: it is not an interface.
func Embeds(t types.Type, match func(types.Type) bool) bool {
	t = UnwrapPointer(t)
	if tp, ok := t.(*types.TypeParam); ok {
		t = tp.Constraint()
	}
	return embeds(t, match)
}

func embeds(t types.Type, match func(types.Type) bool) bool {
	if match(t) {
		return true
	}
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for i := range iface.NumEmbeddeds() {
		if embeds(iface.EmbeddedType(i), match) {
			return true
		}
	}
	return false
}

// UnwrapPointer recursively unwraps all pointer layers, and the aliases
// around and between them.
//
// This is critical for SSA-based carrier type matching. When a closure captures
// a pointer variable, SSA represents it with an additional level of indirection:
//
//	func handler(ctx *CarrierType) {
//	    go func() {
//	        _ = ctx  // SSA FreeVars: **CarrierType (not *CarrierType)
//	    }()
//	}
//
// Therefore, we must unwrap ALL pointer layers to match against the registered
// carrier type (CarrierType, no pointer). Single-layer unwrapping would leave
// *CarrierType, which wouldn't match.
//
// An alias, such as type Ctx = context.Context, is a *types.Alias and not
// the type it names, so it is unaliased at each layer.
func UnwrapPointer(t types.Type) types.Type {
	for {
		t = types.Unalias(t)
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = ptr.Elem()
	}
}

// PkgPathMatches checks if pkgPath matches targetPkg, allowing a major
// version suffix such as /v2. Only the suffix may follow: neither
// /v1alpha1 nor /v2/sub matches.
func PkgPathMatches(pkgPath, targetPkg string) bool {
	if pkgPath == targetPkg {
		return true
	}
	rest, ok := strings.CutPrefix(pkgPath, targetPkg+"/v")
	return ok && rest != "" && strings.Trim(rest, "0123456789") == ""
}

// CheckPkgPath rejects pkgPath, the package of the name s, when it is a bare
// name that is not a standard library package. Such a name never matches,
// since a package is matched by its full import path.
func CheckPkgPath(s, pkgPath string) error {
	if !strings.Contains(pkgPath, "/") && !isStdPackage(pkgPath) {
		return fmt.Errorf("%q: %q is not an import path; write the full path of the package, as github.com/org/%s",
			s, pkgPath, s)
	}
	return nil
}

// CheckItem rejects item, the i-th item (from 0) of the list setting flag,
// when it is blank or holds a comma. noun is what one item names, as
// "function". The command line splits a flag at commas and drops blank
// parts first, so only a list given as a list, as the plugin's settings
// and goroutinectx.Config are, can hold such an item.
func CheckItem(flag string, i int, item, noun string) error {
	if strings.TrimSpace(item) == "" {
		return fmt.Errorf("%s: item %d is empty", flag, i+1)
	}
	if strings.Contains(item, ",") {
		return fmt.Errorf("%s: %q: each item is one %s, and commas separate %ss only on the command line",
			flag, item, noun, noun)
	}
	return nil
}

// stdTopLevel lists the standard library packages whose path has no slash.
var stdTopLevel = []string{
	"bufio", "bytes", "cmp", "context", "crypto", "embed", "encoding", "errors", "expvar", "flag", "fmt",
	"hash", "html", "image", "io", "iter", "log", "maps", "math", "mime", "net", "os", "path", "plugin",
	"reflect", "regexp", "runtime", "slices", "sort", "strconv", "strings", "structs", "sync", "syscall",
	"testing", "time", "unicode", "unique", "unsafe", "uuid", "weak",
}

// isStdPackage reports whether path, which has no slash, is a standard
// library package: one listed, or one GOROOT holds, for a package newer than
// the list.
func isStdPackage(path string) bool {
	if slices.Contains(stdTopLevel, path) {
		return true
	}
	pkg, err := build.Default.Import(path, "", build.FindOnly)
	return err == nil && pkg.Goroot
}
