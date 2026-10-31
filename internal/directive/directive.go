package directive

import (
	"go/ast"
	"go/token"
	"strings"
	"unicode"

	"github.com/mpyw/goroutinectx/internal/checker"
)

// tool is the tool part of every goroutinectx directive.
const tool = "goroutinectx"

// Found holds the goroutinectx directives of one file, as [Scan] gives them.
type Found struct {
	// Ignores holds each //goroutinectx:ignore, in source order. The
	// checker names are not checked: ignore.Build reports an unknown one.
	Ignores []Ignore
	// Spawners holds each function whose doc comment holds a
	// //goroutinectx:spawner, once.
	Spawners []*ast.FuncDecl
	// Problems holds each comment addressed to goroutinectx that does
	// nothing, in source order.
	Problems []Problem
}

// Ignore is a //goroutinectx:ignore directive.
type Ignore struct {
	Comment  *ast.Comment
	Checkers []checker.Name // The checkers it silences: empty for all
}

// Problem is a comment addressed to goroutinectx that does nothing. It is
// reported, since it would otherwise be dropped with no sign: a spawner
// would not be checked, and an ignore would not silence anything.
type Problem struct {
	Pos     token.Pos
	Message string
}

// Scan reads every goroutinectx directive of a file in one pass over its
// comments.
//
// A comment addressed to goroutinectx that is not a directive is a problem,
// and so is a directive with a name goroutinectx does not read:
// //goroutinectx:ignre would otherwise leave the report it was written for,
// with no hint why. A spawner directive outside the doc comment of a
// function is a problem too.
func Scan(file *ast.File) Found {
	var found Found
	var funcDocs map[*ast.CommentGroup]*ast.FuncDecl // Made at the first spawner directive
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if !addressed(c.Text) {
				continue // Most comments, and cheap to tell
			}
			d, ok := parse(c.Text)
			switch {
			case !ok:
				found.problem(c, "malformed goroutinectx directive: write it as //goroutinectx:name")
			case d.Name == "ignore":
				found.Ignores = append(found.Ignores, Ignore{Comment: c, Checkers: ignoreCheckers(d.Args)})
			case d.Name == "spawner":
				if funcDocs == nil {
					funcDocs = funcDocsOf(file)
				}
				fn := funcDocs[cg]
				switch {
				case fn == nil:
					found.problem(c, "goroutinectx:spawner has no effect here: write it in the doc comment of a function")
				case len(found.Spawners) == 0 || found.Spawners[len(found.Spawners)-1] != fn:
					// A second directive in the same doc comment marks the function again.
					found.Spawners = append(found.Spawners, fn)
				}
			default:
				found.problem(c, "unknown directive goroutinectx:"+d.Name)
			}
		}
	}
	return found
}

func (f *Found) problem(c *ast.Comment, message string) {
	f.Problems = append(f.Problems, Problem{Pos: c.Pos(), Message: message})
}

// funcDocsOf maps the doc comment of each function of file to the function.
func funcDocsOf(file *ast.File) map[*ast.CommentGroup]*ast.FuncDecl {
	docs := make(map[*ast.CommentGroup]*ast.FuncDecl)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Doc != nil {
			docs[fn.Doc] = fn
		}
	}
	return docs
}

// addressed reports whether a comment is addressed to goroutinectx: its
// body, after "//" or "/*", starts with "goroutinectx:" once leading
// whitespace is skipped. Every directive is addressed, and an addressed
// comment that is not one is malformed.
func addressed(text string) bool {
	body, ok := strings.CutPrefix(text, "//")
	if !ok {
		if body, ok = strings.CutPrefix(text, "/*"); !ok {
			return false
		}
	}

	return strings.HasPrefix(strings.TrimLeftFunc(body, unicode.IsSpace), tool+":")
}

// parse parses a comment as a goroutinectx directive.
// Only the canonical form "//goroutinectx:name [args]" is a directive,
// as for any other Go directive. It reports false for any other comment,
// including another tool's directive and the malformed forms.
//
// A trailing comment explains the directive and is dropped first:
// "//goroutinectx:ignore // reason" is a bare ignore.
func parse(text string) (ast.Directive, bool) {
	if body, ok := strings.CutPrefix(text, "//"); ok {
		if before, _, found := strings.Cut(body, "//"); found {
			text = "//" + before
		}
	}
	d, ok := ast.ParseDirective(token.NoPos, text)
	if !ok || d.Tool != tool {
		return ast.Directive{}, false
	}

	return d, true
}

// ignoreCheckers gives the checker names the arguments of an ignore
// directive list: none to ignore all. A reason goes after "//", which parse
// drops, or after " - ", kept for compatibility. Anything else after the name
// is read as checker names. ignore.Build reports a name goroutinectx does not
// have, since a misspelled checker or a reason written without a separator
// would otherwise be read as a checker that never reports.
func ignoreCheckers(args string) []checker.Name {
	rest, _, _ := strings.Cut(args, " - ")
	if strings.HasPrefix(rest, "- ") || rest == "-" {
		return nil
	}

	var checkers []checker.Name
	for part := range strings.SplitSeq(rest, ",") {
		if name := checker.Name(strings.TrimSpace(part)); name != "" {
			checkers = append(checkers, name)
		}
	}
	return checkers
}
