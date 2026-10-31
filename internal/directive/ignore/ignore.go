package ignore

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/checkerdef"
	"github.com/mpyw/goroutinectx/internal/directive"
)

// Entry tracks an ignore directive and its usage.
type Entry struct {
	pos      token.Pos             // Position of the ignore comment
	trailing bool                  // The comment follows code on its line
	checkers []checker.Name        // List of checker names (empty = all)
	used     map[checker.Name]bool // Track usage per checker
}

// Map tracks ignore entries by line number.
type Map map[int]*Entry

// EnabledCheckers tracks which checkers are currently enabled.
type EnabledCheckers map[checker.Name]bool

// Build gives the ignore map of a file from its ignore directives, as
// [directive.Scan] finds them, and the ignores that name a checker
// goroutinectx does not have. Such an ignore silences nothing: the whole
// directive is dropped on one unknown name.
func Build(fset *token.FileSet, file *ast.File, ignores []directive.Ignore) (Map, []directive.Problem) {
	m := make(Map)
	var problems []directive.Problem
	known := ignores[:0:0]
	for _, d := range ignores {
		if i := slices.IndexFunc(d.Checkers, func(n checker.Name) bool { return !slices.Contains(checkerdef.Names, n) }); i >= 0 {
			problems = append(problems, directive.Problem{Pos: d.Comment.Pos(), Message: unknownChecker(d.Checkers[i])})
			continue
		}
		known = append(known, d)
	}
	if len(known) == 0 {
		return m, problems // Most files have none, and need no walk of their code
	}

	codeStarts := codeStartsByLine(fset, file)
	for _, d := range known {
		line := fset.PositionFor(d.Comment.Pos(), false).Line
		start, hasCode := codeStarts[line]
		m[line] = &Entry{
			pos:      d.Comment.Pos(),
			trailing: hasCode && start < d.Comment.Pos(),
			checkers: d.Checkers,
			used:     make(map[checker.Name]bool),
		}
	}

	return m, problems
}

func unknownChecker(name checker.Name) string {
	names := make([]string, len(checkerdef.Names))
	for i, n := range checkerdef.Names {
		names[i] = string(n)
	}
	return fmt.Sprintf("unknown checker %q in goroutinectx:ignore (want one of %s; write a reason after //)",
		string(name), strings.Join(names, ", "))
}

// codeStartsByLine gives, for each line where a node of file starts or ends,
// the first such position. A comment after that position follows code.
func codeStartsByLine(fset *token.FileSet, file *ast.File) map[int]token.Pos {
	starts := make(map[int]token.Pos)
	mark := func(pos token.Pos) {
		if !pos.IsValid() {
			return
		}
		line := fset.PositionFor(pos, false).Line
		if first, ok := starts[line]; !ok || pos < first {
			starts[line] = pos
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n.(type) {
		case nil, *ast.Comment, *ast.CommentGroup:
			return false
		}
		mark(n.Pos())
		mark(n.End() - 1)
		return true
	})
	return starts
}

// Covering gives the ignore that silences the reports of the checker name on
// line, or nil: an ignore on the line, or one on its own on the line above.
// An ignore that follows code covers its own line only. Asking marks
// nothing: call [Entry.MarkUsed] on the ignore that silences a report.
func (m Map) Covering(line int, name checker.Name) *Entry {
	if e := m[line]; e.silences(name) {
		return e
	}
	if above := m[line-1]; above != nil && !above.trailing && above.silences(name) {
		return above
	}

	return nil
}

// silences reports whether e ignores the checker name.
func (e *Entry) silences(name checker.Name) bool {
	if e == nil {
		return false
	}

	// Empty checkers list means ignore all
	return len(e.checkers) == 0 || slices.Contains(e.checkers, name)
}

// MarkUsed records that e silenced a report of the checker name, so it is
// not reported as unused for it.
func (e *Entry) MarkUsed(name checker.Name) {
	e.used[name] = true
}

// UnusedIgnore represents an unused ignore directive.
type UnusedIgnore struct {
	Pos      token.Pos
	Checkers []checker.Name // Unused checker names (empty if entire directive is unused)
}

// GetUnusedIgnores returns ignore directives that were not used.
func (m Map) GetUnusedIgnores(enabled EnabledCheckers) []UnusedIgnore {
	var unused []UnusedIgnore

	for _, entry := range m {
		if len(entry.checkers) == 0 {
			// Ignore-all directive: check if any enabled checker used it
			anyUsed := false
			for name := range enabled {
				if entry.used[name] {
					anyUsed = true
					break
				}
			}
			if !anyUsed {
				unused = append(unused, UnusedIgnore{Pos: entry.pos})
			}
		} else {
			// Specific checkers: report each unused one
			var unusedCheckers []checker.Name
			for _, name := range entry.checkers {
				if !enabled[name] {
					// Checker is not enabled - report as invalid
					unusedCheckers = append(unusedCheckers, name)
				} else if !entry.used[name] {
					// Checker is enabled but wasn't used
					unusedCheckers = append(unusedCheckers, name)
				}
			}
			if len(unusedCheckers) > 0 {
				unused = append(unused, UnusedIgnore{
					Pos:      entry.pos,
					Checkers: unusedCheckers,
				})
			}
		}
	}

	return unused
}
