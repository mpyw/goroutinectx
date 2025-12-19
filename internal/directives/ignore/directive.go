// Package ignore handles //goroutinectx:ignore directives.
package ignore

import (
	"go/ast"
	"go/token"
	"strings"
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
)

// AllCheckerNames returns all valid checker names.
func AllCheckerNames() []CheckerName {
	return []CheckerName{
		Goroutine,
		GoroutineDerive,
		Waitgroup,
		Errgroup,
		Spawner,
		Spawnerlabel,
		Gotask,
	}
}

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

// Build scans a file for ignore comments and returns a map.
func Build(fset *token.FileSet, file *ast.File) Map {
	m := make(Map)

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if checkers, ok := parseIgnoreComment(c.Text); ok {
				line := fset.Position(c.Pos()).Line
				m[line] = &Entry{
					pos:      c.Pos(),
					checkers: checkers,
					used:     make(map[CheckerName]bool),
				}
			}
		}
	}

	return m
}

// parseIgnoreComment parses an ignore directive and returns the checker names.
// Returns nil slice if no specific checkers are specified (ignore all).
// Returns false if not an ignore comment.
//
// Supported formats:
//   - //goroutinectx:ignore                           -> ignore all checkers
//   - //goroutinectx:ignore goroutine                 -> ignore specific checker
//   - //goroutinectx:ignore goroutine,errgroup        -> ignore multiple checkers
//   - //goroutinectx:ignore - reason                  -> ignore all with comment
//   - //goroutinectx:ignore goroutine - reason        -> ignore specific with comment
func parseIgnoreComment(text string) ([]CheckerName, bool) {
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimSpace(text)

	if !strings.HasPrefix(text, "goroutinectx:ignore") {
		return nil, false
	}

	// Extract checker names after "goroutinectx:ignore"
	rest := strings.TrimPrefix(text, "goroutinectx:ignore")
	rest = strings.TrimSpace(rest)

	if rest == "" {
		return nil, true // No specific checkers = ignore all
	}

	// Stop at comment markers: " - ", " // ", or " //"
	// These indicate the start of a human-readable comment
	if idx := strings.Index(rest, " - "); idx >= 0 {
		rest = rest[:idx]
	}
	if idx := strings.Index(rest, " //"); idx >= 0 {
		rest = rest[:idx]
	}
	// Also handle "- " at the start (no checkers specified, just comment)
	if strings.HasPrefix(rest, "- ") || rest == "-" {
		return nil, true
	}

	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, true // No specific checkers after trimming = ignore all
	}

	// Parse comma-separated checker names
	parts := strings.Split(rest, ",")
	checkers := make([]CheckerName, 0, len(parts))

	for _, part := range parts {
		name := CheckerName(strings.TrimSpace(part))
		if name != "" {
			checkers = append(checkers, name)
		}
	}

	return checkers, true
}

// ShouldIgnore returns true if the given line should be ignored for the specified checker.
// It checks if the same line or the previous line has an ignore comment.
// When an ignore is used, it marks the entry as used for that checker.
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
	for _, c := range entry.checkers {
		if c == checker {
			entry.used[checker] = true
			return true
		}
	}

	return false
}

// UnusedIgnore represents an unused ignore directive.
type UnusedIgnore struct {
	Pos      token.Pos
	Checkers []CheckerName // Unused checker names (empty if entire directive is unused)
}

// GetUnusedIgnores returns ignore directives that were not used.
// It takes the enabled checkers to determine which unused specifications are valid to report.
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
