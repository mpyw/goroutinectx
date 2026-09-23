package directive

import (
	"go/ast"
	"go/token"
	"strings"
)

// tool is the tool part of every goroutinectx directive.
const tool = "goroutinectx"

// Parse parses a comment as a goroutinectx directive.
// It accepts both "//goroutinectx:name" and "// goroutinectx:name".
// It reports false for any other comment, including another tool's directive.
func Parse(text string) (ast.Directive, bool) {
	// go/ast only recognizes the canonical form with no space after "//",
	// so re-attach the comment marker to the trimmed body first.
	if body, ok := strings.CutPrefix(text, "//"); ok {
		text = "//" + strings.TrimSpace(body)
	}

	d, ok := ast.ParseDirective(token.NoPos, text)
	if !ok || d.Tool != tool {
		return ast.Directive{}, false
	}

	return d, true
}
