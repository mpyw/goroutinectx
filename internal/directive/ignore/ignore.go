package ignore

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"github.com/mpyw/goroutinectx/internal/directive"
)

// CheckerName represents a checker that can be ignored.
type CheckerName string

// Valid checker names.
const (
	Goroutine       CheckerName = "goroutine"
	GoroutineDerive CheckerName = "goroutinederive"
	Waitgroup       CheckerName = "waitgroup"
	Errgroup        CheckerName = "errgroup"
	Spawner         CheckerName = "spawner"
	Spawnerlabel    CheckerName = "spawnerlabel"
	Gotask          CheckerName = "gotask"
	Conc            CheckerName = "conc"
)

// checkerNames lists every name an ignore directive may give, in the order
// the report of an unknown one lists them.
var checkerNames = []CheckerName{Goroutine, GoroutineDerive, Waitgroup, Errgroup, Conc, Spawner, Spawnerlabel, Gotask}

// Entry tracks an ignore directive and its usage.
type Entry struct {
	pos      token.Pos            // Position of the ignore comment
	checkers []CheckerName        // List of checker names (empty = all)
	used     map[CheckerName]bool // Track usage per checker
}

// Map tracks ignore entries by line number.
type Map map[int]*Entry

// EnabledCheckers tracks which checkers are currently enabled.
type EnabledCheckers map[CheckerName]bool

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
				used:     make(map[CheckerName]bool),
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
func parseComment(text string) ([]CheckerName, string, bool) {
	d, ok := directive.Parse(text)
	if !ok || d.Name != "ignore" {
		return nil, "", false
	}

	rest, _, _ := strings.Cut(d.Args, " - ")
	if strings.HasPrefix(rest, "- ") || rest == "-" {
		return nil, "", true
	}

	var checkers []CheckerName
	for part := range strings.SplitSeq(rest, ",") {
		name := CheckerName(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if !slices.Contains(checkerNames, name) {
			return nil, unknownChecker(name), true
		}
		checkers = append(checkers, name)
	}

	// No specific checkers = ignore all
	return checkers, "", true
}

func unknownChecker(name CheckerName) string {
	names := make([]string, len(checkerNames))
	for i, n := range checkerNames {
		names[i] = string(n)
	}
	return fmt.Sprintf("unknown checker %q in goroutinectx:ignore (want one of %s; write a reason after //)",
		string(name), strings.Join(names, ", "))
}

// ShouldIgnore returns true if the given line should be ignored for the specified checker.
func (m Map) ShouldIgnore(line int, checker CheckerName) bool {
	if m.shouldIgnoreEntry(m[line], checker) {
		return true
	}
	if m.shouldIgnoreEntry(m[line-1], checker) {
		return true
	}

	return false
}

// shouldIgnoreEntry checks if an entry ignores the specified checker.
func (m Map) shouldIgnoreEntry(entry *Entry, checker CheckerName) bool {
	if entry == nil {
		return false
	}

	// Empty checkers list means ignore all
	if len(entry.checkers) == 0 {
		entry.used[checker] = true
		return true
	}

	// Check if the specified checker is in the list
	if slices.Contains(entry.checkers, checker) {
		entry.used[checker] = true
		return true
	}

	return false
}

// UnusedIgnore represents an unused ignore directive.
type UnusedIgnore struct {
	Pos      token.Pos
	Checkers []CheckerName // Unused checker names (empty if entire directive is unused)
}

// GetUnusedIgnores returns ignore directives that were not used.
func (m Map) GetUnusedIgnores(enabled EnabledCheckers) []UnusedIgnore {
	var unused []UnusedIgnore

	for _, entry := range m {
		if len(entry.checkers) == 0 {
			// Ignore-all directive: check if any enabled checker used it
			anyUsed := false
			for checker := range enabled {
				if entry.used[checker] {
					anyUsed = true
					break
				}
			}
			if !anyUsed {
				unused = append(unused, UnusedIgnore{Pos: entry.pos})
			}
		} else {
			// Specific checkers: report each unused one
			var unusedCheckers []CheckerName
			for _, checker := range entry.checkers {
				if !enabled[checker] {
					// Checker is not enabled - report as invalid
					unusedCheckers = append(unusedCheckers, checker)
				} else if !entry.used[checker] {
					// Checker is enabled but wasn't used
					unusedCheckers = append(unusedCheckers, checker)
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
