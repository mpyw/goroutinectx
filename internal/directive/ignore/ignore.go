package ignore

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"github.com/mpyw/goroutinectx/internal/checker"
	"github.com/mpyw/goroutinectx/internal/directive"
)

// Entry tracks an ignore directive and its usage.
type Entry struct {
	pos      token.Pos             // Position of the ignore comment
	checkers []checker.Name        // List of checker names (empty = all)
	used     map[checker.Name]bool // Track usage per checker
}

// Map tracks ignore entries by line number.
type Map map[int]*Entry

// EnabledCheckers tracks which checkers are currently enabled.
type EnabledCheckers map[checker.Name]bool

// Problem is an ignore directive that names a checker goroutinectx does not
// have. It is not an ignore: it silences nothing.
type Problem struct {
	Pos     token.Pos
	Message string
}

// Build scans a file for ignore comments and returns a map, and the ignore
// comments that name an unknown checker.
func Build(fset *token.FileSet, file *ast.File) (Map, []Problem) {
	m := make(Map)
	var problems []Problem

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			checkers, problem, ok := parseComment(c.Text)
			if !ok {
				continue
			}
			if problem != "" {
				problems = append(problems, Problem{Pos: c.Pos(), Message: problem})
				continue
			}
			line := fset.PositionFor(c.Pos(), false).Line
			m[line] = &Entry{
				pos:      c.Pos(),
				checkers: checkers,
				used:     make(map[checker.Name]bool),
			}
		}
	}

	return m, problems
}

// parseComment parses an ignore directive and returns the checker names.
// Returns nil slice if no specific checkers are specified (ignore all).
// Returns false if not an ignore comment. A reason goes after "//", which
// directive.Parse drops, or after " - ", kept for compatibility. Anything else
// after the name is read as checker names, and a name goroutinectx does not
// have is a problem: the directive silences nothing. A misspelled checker or a
// reason written without a separator would otherwise be read as a checker that
// never reports.
func parseComment(text string) ([]checker.Name, string, bool) {
	d, ok := directive.Parse(text)
	if !ok || d.Name != "ignore" {
		return nil, "", false
	}

	rest, _, _ := strings.Cut(d.Args, " - ")
	if strings.HasPrefix(rest, "- ") || rest == "-" {
		return nil, "", true
	}

	var checkers []checker.Name
	for part := range strings.SplitSeq(rest, ",") {
		name := checker.Name(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if !slices.Contains(checker.Names, name) {
			return nil, unknownChecker(name), true
		}
		checkers = append(checkers, name)
	}

	// No specific checkers = ignore all
	return checkers, "", true
}

func unknownChecker(name checker.Name) string {
	names := make([]string, len(checker.Names))
	for i, n := range checker.Names {
		names[i] = string(n)
	}
	return fmt.Sprintf("unknown checker %q in goroutinectx:ignore (want one of %s; write a reason after //)",
		string(name), strings.Join(names, ", "))
}

// ShouldIgnore returns true if the given line should be ignored for the specified checker.
func (m Map) ShouldIgnore(line int, name checker.Name) bool {
	if m.shouldIgnoreEntry(m[line], name) {
		return true
	}
	if m.shouldIgnoreEntry(m[line-1], name) {
		return true
	}

	return false
}

// shouldIgnoreEntry checks if an entry ignores the specified checker.
func (m Map) shouldIgnoreEntry(entry *Entry, name checker.Name) bool {
	if entry == nil {
		return false
	}

	// Empty checkers list means ignore all
	if len(entry.checkers) == 0 {
		entry.used[name] = true
		return true
	}

	// Check if the specified checker is in the list
	if slices.Contains(entry.checkers, name) {
		entry.used[name] = true
		return true
	}

	return false
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
