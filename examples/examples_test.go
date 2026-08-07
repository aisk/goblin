package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
)

func TestExamples(t *testing.T) {
	examplesDir := filepath.Join("..", "examples")

	files, err := filepath.Glob(filepath.Join(examplesDir, "*.goblin"))
	if err != nil {
		t.Fatalf("failed to find .goblin files: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no .goblin files found in %s", examplesDir)
	}

	tempDir, err := os.MkdirTemp("", "goblin-test-")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	for _, file := range files {
		baseName := strings.TrimSuffix(filepath.Base(file), ".goblin")
		t.Run(baseName, func(t *testing.T) {
			goCode := parseAndTranspile(t, file)
			stdout, stderr := writeAndRun(t, tempDir, baseName, goCode)
			checkOutput(t, examplesDir, baseName, ".stdout", stdout, true)
			checkOutput(t, examplesDir, baseName, ".stderr", stderr, false)
		})
	}
}

// concurrencyMarkers matches the constructs that start goroutines or use
// channels; any example using one is also run under the Go race detector.
// Detection by content means a new concurrency example cannot silently skip
// the race run.
var concurrencyMarkers = regexp.MustCompile(`\bspawn\b|\bChan\(|\bGoblin\(`)

// raceExamples lists the examples whose transpiled form is also run under the
// race detector, guarding the coordination patterns the book recommends.
func raceExamples(t *testing.T, examplesDir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(examplesDir, "*.goblin"))
	if err != nil || len(files) == 0 {
		t.Fatalf("failed to find .goblin files: %v", err)
	}
	var names []string
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		if concurrencyMarkers.Match(src) {
			names = append(names, strings.TrimSuffix(filepath.Base(file), ".goblin"))
		}
	}
	return names
}

func TestExamplesRace(t *testing.T) {
	examplesDir := filepath.Join("..", "examples")

	tempDir, err := os.MkdirTemp("", "goblin-race-test-")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	for _, baseName := range raceExamples(t, examplesDir) {
		t.Run(baseName, func(t *testing.T) {
			goCode := parseAndTranspile(t, filepath.Join(examplesDir, baseName+".goblin"))
			stdout, stderr := writeAndRun(t, tempDir, baseName, goCode, "-race")
			checkOutput(t, examplesDir, baseName, ".stdout", stdout, true)
			checkOutput(t, examplesDir, baseName, ".stderr", stderr, false)
		})
	}
}

// parseAndTranspile reads a .goblin file, parses and transpiles it to Go code.
func parseAndTranspile(t *testing.T, goblinFile string) string {
	t.Helper()

	l, err := source.NewLexerFile(goblinFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	module, ok := st.(*ast.Module)
	if !ok {
		t.Fatalf("failed to convert AST to Module")
	}
	if err := semantic.CheckModule(module); err != nil {
		t.Fatalf("semantic error: %v", err)
	}

	var buf bytes.Buffer
	if err := transpiler.Transpile(module, &buf); err != nil {
		t.Fatalf("transpile error: %v", err)
	}
	return buf.String()
}

// writeAndRun writes Go code to a temp file and executes it, returning stdout
// and stderr. Extra buildFlags (such as -race) are passed to go run.
func writeAndRun(t *testing.T, tempDir, baseName, goCode string, buildFlags ...string) (string, string) {
	t.Helper()

	goFile := filepath.Join(tempDir, baseName+".go")
	if err := os.WriteFile(goFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write Go file: %v", err)
	}

	args := append([]string{"run"}, buildFlags...)
	args = append(args, goFile)
	cmd := exec.Command("go", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("go run failed: %v\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), stderr.String()
}

// checkOutput compares actual output against the expected file.
// A stdout golden file is required. A missing stderr file means empty stderr.
func checkOutput(t *testing.T, examplesDir, baseName, suffix, actual string, required bool) {
	t.Helper()

	expectedFile := filepath.Join(examplesDir, baseName+suffix)

	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		if required {
			t.Fatalf("missing expected output file: %s", expectedFile)
		}
		if normalize(actual) != "" {
			t.Errorf("%s mismatch (no expected file, assuming empty):\nActual:\n%s", suffix, actual)
		}
		return
	}

	expected, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("failed to read %s file: %v", suffix, err)
	}

	if normalize(actual) != normalize(string(expected)) {
		t.Errorf("%s mismatch:\nExpected:\n%s\nActual:\n%s", suffix, string(expected), actual)
	}
}

// normalize standardizes line endings for cross-platform comparison.
func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
