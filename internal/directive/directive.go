package directive

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"unicode"
)

// tool is the tool part of every goroutinectx directive.
const tool = "goroutinectx"

// names lists the directives goroutinectx reads.
var names = []string{"ignore", "spawner"}

// Known reports whether name is a directive goroutinectx reads. A directive
// with any other name does nothing, so it is reported rather than silently
// skipped: //goroutinectx:ignre would otherwise leave the report it was
// written for, with no hint why.
func Known(name string) bool {
	return slices.Contains(names, name)
}

// Parse parses a comment as a goroutinectx directive.
// Only the canonical form "//goroutinectx:name [args]" is a directive,
// as for any other Go directive. It reports false for any other comment,
// including another tool's directive and the forms [Malformed] reports.
//
// A trailing comment explains the directive and is dropped first:
// "//goroutinectx:ignore // reason" is a bare ignore.
func Parse(text string) (ast.Directive, bool) {
	if body, ok := strings.CutPrefix(text, "//"); ok {
		if i := strings.Index(body, "//"); i >= 0 {
			text = "//" + body[:i]
		}
	}
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
