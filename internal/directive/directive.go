package directive

import (
	"go/ast"
	"go/token"
	"strings"
	"unicode"
)

// tool is the tool part of every goroutinectx directive.
const tool = "goroutinectx"

// Parse parses a comment as a goroutinectx directive.
// Only the canonical form "//goroutinectx:name [args]" is a directive,
// as for any other Go directive. It reports false for any other comment,
// including another tool's directive and the forms [Malformed] reports.
func Parse(text string) (ast.Directive, bool) {
	d, ok := ast.ParseDirective(token.NoPos, text)
	if !ok || d.Tool != tool {
		return ast.Directive{}, false
	}

	return d, true
}

// Malformed reports whether a comment is addressed to goroutinectx but is not
// a directive. A comment is addressed when its body, after "//" or "/*",
// starts with "goroutinectx:" once leading whitespace is skipped. Examples are
// "// goroutinectx:ignore", "//goroutinectx: ignore", "/* goroutinectx:ignore */",
// "//goroutinectx:Ignore" and "//goroutinectx:".
//
// It also returns the canonical form to write instead, built from the name
// as written. The suggestion is empty when that form is not a directive
// either, such as for an uppercase name or no name.
func Malformed(text string) (string, bool) {
	if _, ok := Parse(text); ok {
		return "", false
	}

	body, ok := strings.CutPrefix(text, "//")
	if !ok {
		if body, ok = strings.CutPrefix(text, "/*"); !ok {
			return "", false
		}
		body = strings.TrimSuffix(body, "*/")
	}

	rest, ok := strings.CutPrefix(strings.TrimLeftFunc(body, unicode.IsSpace), tool+":")
	if !ok {
		return "", false
	}

	name := strings.TrimLeftFunc(rest, unicode.IsSpace)
	if i := strings.IndexFunc(name, unicode.IsSpace); i >= 0 {
		name = name[:i]
	}

	want := "//" + tool + ":" + name
	if _, ok := Parse(want); !ok {
		return "", true
	}

	return want, true
}
