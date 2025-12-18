package metatest

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// supportsWaitgroupGo returns true if the current Go version supports sync.WaitGroup.Go()
// which was added in Go 1.25.
func supportsWaitgroupGo() bool {
	// runtime.Version() returns something like "go1.25.3"
	version := runtime.Version()
	// Extract major.minor version
	if !strings.HasPrefix(version, "go") {
		return false
	}
	version = strings.TrimPrefix(version, "go")
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major := parts[0]
	minor := parts[1]
	// Go 1.25+ supports WaitGroup.Go()
	if major == "1" {
		if len(minor) >= 2 && minor >= "25" {
			return true
		}
	}
	return false
}

// Structure represents the test metadata structure.
type Structure struct {
	Targets []string        `json:"targets"`
	Tests   map[string]Test `json:"tests"`
}

// Test represents a single test pattern across multiple checkers.
type Test struct {
	Title    string              `json:"title"`
	Targets  []string            `json:"targets"`
	Level    string              `json:"level"` // Shared level for all targets
	Variants map[string]*Variant `json:"variants"`
}

// Variant represents a good, bad, limitation, or notChecked variant.
type Variant struct {
	Description string            `json:"description"`
	Functions   map[string]string `json:"functions"`
}

func TestStructureValidation(t *testing.T) {
	// Load structure.json
	structureFile := filepath.Join("structure.json")
	data, err := os.ReadFile(structureFile)
	if err != nil {
		t.Fatalf("Failed to read structure.json: %v", err)
	}

	var structure Structure
	if err := json.Unmarshal(data, &structure); err != nil {
		t.Fatalf("Failed to parse structure.json: %v", err)
	}

	// Validate each test
	for testName, test := range structure.Tests {
		t.Run(testName, func(t *testing.T) {
			validateTest(t, &structure, testName, &test)
		})
	}

	// Validate that all functions are accounted for
	t.Run("AllFunctionsAccountedFor", func(t *testing.T) {
		validateAllFunctionsAccountedFor(t, &structure)
	})
}

func validateTest(t *testing.T, structure *Structure, testName string, test *Test) {
	// Validate targets exist in global targets list
	for _, target := range test.Targets {
		if !contains(structure.Targets, target) {
			t.Errorf("Target %q not found in global targets list", target)
		}
	}

	// Validate each variant
	for variantType, variant := range test.Variants {
		if variant == nil {
			continue // null variant is valid
		}

		t.Run(variantType, func(t *testing.T) {
			validateVariant(t, structure, testName, test, variantType, variant)
		})
	}
}

func validateVariant(t *testing.T, structure *Structure, testName string, test *Test, variantType string, variant *Variant) {
	// Validate level is set
	if test.Level == "" {
		t.Errorf("Missing level in test %q", testName)
		return
	}

	// Get function name from variant.Functions
	for _, target := range test.Targets {
		// Skip waitgroup tests on Go < 1.25
		if target == "waitgroup" && !supportsWaitgroupGo() {
			t.Skipf("Skipping waitgroup test: sync.WaitGroup.Go() requires Go 1.25+")
		}
		funcName, ok := variant.Functions[target]
		if !ok {
			t.Errorf("Missing function for target %q in test %q variant %q", target, testName, variantType)
			continue
		}

		// Find specific test file for this level
		testFile := findTestFile(target, test.Level)
		if testFile == "" {
			t.Errorf("Test file %s.go not found for target %q", test.Level, target)
			continue
		}

		// Check if function exists and has correct comments
		if !validateFunctionInFile(t, testFile, funcName, test, variant, variantType, target, structure.Targets, testName) {
			t.Errorf("Function %q not found in %s for target %q", funcName, testFile, target)
		}
	}
}

// validateAllFunctionsAccountedFor checks that all functions in test files
// are either in structure.json or marked with //vt:helper
func validateAllFunctionsAccountedFor(t *testing.T, structure *Structure) {
	// Build map of expected functions by target and level
	expectedFunctions := make(map[string]map[string]map[string]bool) // target -> level -> funcName -> true
	for _, test := range structure.Tests {
		for _, variant := range test.Variants {
			if variant == nil {
				continue
			}
			for _, target := range test.Targets {
				funcName := variant.Functions[target]

				if expectedFunctions[target] == nil {
					expectedFunctions[target] = make(map[string]map[string]bool)
				}
				if expectedFunctions[target][test.Level] == nil {
					expectedFunctions[target][test.Level] = make(map[string]bool)
				}
				expectedFunctions[target][test.Level][funcName] = true
			}
		}
	}

	// Check each target's files
	for target := range expectedFunctions {
		// Skip waitgroup on Go < 1.25
		if target == "waitgroup" && !supportsWaitgroupGo() {
			continue
		}
		for level := range expectedFunctions[target] {
			testFile := findTestFile(target, level)
			if testFile == "" {
				continue
			}

			// Parse file and get all functions
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, testFile, nil, parser.ParseComments)
			if err != nil {
				t.Errorf("Failed to parse %s: %v", testFile, err)
				continue
			}

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				funcName := fn.Name.Name

				// Check if it's a helper
				isHelper := false
				if fn.Doc != nil {
					for _, comment := range fn.Doc.List {
						if strings.Contains(comment.Text, "//vt:helper") {
							isHelper = true
							break
						}
					}
				}

				if isHelper {
					continue
				}

				// Function must be in structure.json
				if !expectedFunctions[target][level][funcName] {
					t.Errorf("Function %q in %s is not in structure.json and not marked with //vt:helper",
						funcName, testFile)
				}
			}
		}
	}
}

func findTestFile(target, level string) string {
	targetDir := filepath.Join("..", "src", target)

	// Map level to actual filename based on target
	var fileName string
	switch target {
	case "spawner":
		fileName = "spawner.go"
	case "goroutinederive":
		fileName = "goroutinederive.go"
	case "carrier":
		fileName = "carrier.go"
	default:
		fileName = level + ".go"
	}

	filePath := filepath.Join(targetDir, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); err == nil {
		return filePath
	}

	return ""
}

func validateFunctionInFile(t *testing.T, filePath, funcName string, test *Test, variant *Variant, variantType, currentTarget string, allTargets []string, testName string) bool {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		t.Errorf("Failed to parse %s: %v", filePath, err)
		return false
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}

		// Found the function - validate comments
		if fn.Doc == nil || len(fn.Doc.List) == 0 {
			t.Errorf("Function %q in %s has no doc comments", funcName, filePath)
			return true
		}

		comments := extractComments(fn.Doc)
		commentLines := strings.Split(strings.TrimSpace(comments), "\n")

		// New comment format: // [GOOD or BAD]: Title
		variantLabel := strings.ToUpper(variantType)
		expectedFirstLine := fmt.Sprintf("[%s]: %s", variantLabel, test.Title)

		// Check first line matches expected format
		if len(commentLines) == 0 || strings.TrimSpace(commentLines[0]) != expectedFirstLine {
			t.Errorf("Function %q in %s: first comment line should be %q, got %q",
				funcName, filePath, expectedFirstLine,
				func() string {
					if len(commentLines) > 0 {
						return commentLines[0]
					}
					return "(empty)"
				}())
		}

		// Check description: should be on second non-empty line
		if !strings.Contains(comments, variant.Description) {
			t.Errorf("Function %q in %s missing description %q in comments", funcName, filePath, variant.Description)
		}

		// Check "See also" references
		otherTargets := getOtherTargets(test.Targets, currentTarget, allTargets)
		if len(otherTargets) > 0 {
			if !strings.Contains(comments, "See also:") {
				t.Errorf("Function %q in %s missing 'See also:' section", funcName, filePath)
			} else {
				validateSeeAlso(t, comments, otherTargets, variant.Functions, funcName, filePath)
			}
		}

		return true
	}

	return false
}

func extractComments(doc *ast.CommentGroup) string {
	var sb strings.Builder
	for _, comment := range doc.List {
		text := strings.TrimPrefix(comment.Text, "//")
		text = strings.TrimSpace(text)
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return sb.String()
}

func getOtherTargets(testTargets []string, currentTarget string, allTargets []string) []string {
	// Filter testTargets to exclude currentTarget, maintain order from allTargets
	var result []string
	for _, target := range allTargets {
		if target == currentTarget {
			continue
		}
		if contains(testTargets, target) {
			result = append(result, target)
		}
	}
	return result
}

func validateSeeAlso(t *testing.T, comments string, expectedTargets []string, functions map[string]string, funcName, filePath string) {
	// Extract "See also:" section
	seeAlsoIdx := strings.Index(comments, "See also:")
	if seeAlsoIdx == -1 {
		return
	}

	seeAlsoSection := comments[seeAlsoIdx:]

	// Check each expected target appears in correct order
	lastIdx := 0
	for _, target := range expectedTargets {
		expectedFunc := functions[target]
		idx := strings.Index(seeAlsoSection[lastIdx:], target)
		if idx == -1 {
			t.Errorf("Function %q in %s: 'See also:' missing reference to %s (%s)",
				funcName, filePath, target, expectedFunc)
			continue
		}

		// Check if function name is also mentioned
		if !strings.Contains(seeAlsoSection, expectedFunc) {
			t.Errorf("Function %q in %s: 'See also:' mentions %s but not function %s",
				funcName, filePath, target, expectedFunc)
		}

		lastIdx = idx + len(target)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
