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
// starts with "goroutinectx:" once leading whitespace is skipped.
func Malformed(text string) bool {
	if _, ok := Parse(text); ok {
		return false
	}

	body, ok := strings.CutPrefix(text, "//")
	if !ok {
		if body, ok = strings.CutPrefix(text, "/*"); !ok {
			return false
		}
	}

	return strings.HasPrefix(strings.TrimLeftFunc(body, unicode.IsSpace), tool+":")
}
