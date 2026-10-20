package metatest

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Options represents the options.json configuration.
type Options struct {
	ExcludeDirs []string `json:"excludeDirs"`
}

// Structure represents the combined test metadata (built at runtime).
type Structure struct {
	Options Options
	Tests   map[string]Test

	// targets is populated at runtime by scanning testdata/src
	targets []string
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
	// Load options.json
	optionsFile := filepath.Join("options.json")
	data, err := os.ReadFile(optionsFile)
	if err != nil {
		t.Fatalf("Failed to read options.json: %v", err)
	}

	var options Options
	if err := json.Unmarshal(data, &options); err != nil {
		t.Fatalf("Failed to parse options.json: %v", err)
	}

	// Load tests from tests/ directory
	tests, err := loadTests("tests")
	if err != nil {
		t.Fatalf("Failed to load tests: %v", err)
	}

	structure := Structure{
		Options: options,
		Tests:   tests,
	}

	// Discover targets from testdata/src, excluding specified dirs
	structure.targets, err = discoverTargets(structure.Options.ExcludeDirs)
	if err != nil {
		t.Fatalf("Failed to discover targets: %v", err)
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

// loadTests reads all test JSON files from the given directory.
func loadTests(dir string) (map[string]Test, error) {
	tests := make(map[string]Test)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		testName := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(dir, entry.Name())

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		var test Test
		if err := json.Unmarshal(data, &test); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
		}

		tests[testName] = test
	}

	return tests, nil
}

// discoverTargets scans testdata/src and returns all directories except excluded ones.
func discoverTargets(excludeDirs []string) ([]string, error) {
	srcDir := filepath.Join("..", "src")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}

	excludeSet := make(map[string]bool)
	for _, dir := range excludeDirs {
		excludeSet[dir] = true
	}

	var targets []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if excludeSet[name] {
			continue
		}
		targets = append(targets, name)
	}

	return targets, nil
}

func validateTest(t *testing.T, structure *Structure, testName string, test *Test) {
	// Build excludeSet from options
	excludeSet := make(map[string]bool)
	for _, dir := range structure.Options.ExcludeDirs {
		excludeSet[dir] = true
	}

	// Validate targets exist in discovered targets list
	for _, target := range test.Targets {
		// Skip excluded targets
		if excludeSet[target] {
			continue
		}
		if !contains(structure.targets, target) {
			t.Errorf("Target %q not found in testdata/src (discovered targets: %v)", target, structure.targets)
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

	// Build excludeSet from options
	excludeSet := make(map[string]bool)
	for _, dir := range structure.Options.ExcludeDirs {
		excludeSet[dir] = true
	}

	// Get function name from variant.Functions
	for _, target := range test.Targets {
		// Skip excluded targets
		if excludeSet[target] {
			continue
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
		if !validateFunctionInFile(t, testFile, funcName, test, variant, variantType, target, structure.targets, testName) {
			t.Errorf("Function %q not found in %s for target %q", funcName, testFile, target)
		}
	}
}

// validateAllFunctionsAccountedFor checks that all functions in test files
// are either in structure.json or marked with //vt:helper
func validateAllFunctionsAccountedFor(t *testing.T, structure *Structure) {
	// Build map of expected functions by target and file
	expectedFunctions := make(map[string]map[string]map[string]bool) // target -> filename -> funcName -> true
	for _, test := range structure.Tests {
		for _, variant := range test.Variants {
			if variant == nil {
				continue
			}
			for _, target := range test.Targets {
				funcName := variant.Functions[target]
				fileName := test.Level + ".go"

				if expectedFunctions[target] == nil {
					expectedFunctions[target] = make(map[string]map[string]bool)
				}
				if expectedFunctions[target][fileName] == nil {
					expectedFunctions[target][fileName] = make(map[string]bool)
				}
				expectedFunctions[target][fileName][funcName] = true
			}
		}
	}

	// Check each discovered target's files
	for _, target := range structure.targets {
		targetDir := filepath.Join("..", "src", target)
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			t.Errorf("Failed to read target dir %s: %v", targetDir, err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}

			fileName := entry.Name()
			filePath := filepath.Join(targetDir, fileName)

			// Parse file and get all functions
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
			if err != nil {
				t.Errorf("Failed to parse %s: %v", filePath, err)
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
				if expectedFunctions[target] == nil ||
					expectedFunctions[target][fileName] == nil ||
					!expectedFunctions[target][fileName][funcName] {
					t.Errorf("Function %q in %s is not in structure.json and not marked with //vt:helper",
						funcName, filePath)
				}
			}
		}
	}
}

func findTestFile(target, level string) string {
	targetDir := filepath.Join("..", "src", target)
	fileName := level + ".go"
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
