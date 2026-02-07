package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
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

// parseAndTranspile reads a .goblin file, parses and transpiles it to Go code.
func parseAndTranspile(t *testing.T, goblinFile string) string {
	t.Helper()

	source, err := os.ReadFile(goblinFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	l := lexer.NewLexer(source)
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	module, ok := st.(*ast.Module)
	if !ok {
		t.Fatalf("failed to convert AST to Module")
	}

	var buf bytes.Buffer
	if err := transpiler.Transpile(module, &buf); err != nil {
		t.Fatalf("transpile error: %v", err)
	}
	return buf.String()
}

// writeAndRun writes Go code to a temp file and executes it, returning stdout and stderr.
func writeAndRun(t *testing.T, tempDir, baseName, goCode string) (string, string) {
	t.Helper()

	goFile := filepath.Join(tempDir, baseName+".go")
	if err := os.WriteFile(goFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write Go file: %v", err)
	}

	cmd := exec.Command("go", "run", goFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("go run failed: %v\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), stderr.String()
}

// checkOutput compares actual output against the expected file.
// If autoCreate is true and the file doesn't exist, it creates the file with actual output.
// If autoCreate is false and the file doesn't exist, it assumes expected output is empty.
func checkOutput(t *testing.T, examplesDir, baseName, suffix, actual string, autoCreate bool) {
	t.Helper()

	expectedFile := filepath.Join(examplesDir, baseName+suffix)

	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		if autoCreate {
			if err := os.WriteFile(expectedFile, []byte(actual), 0644); err != nil {
				t.Fatalf("failed to create %s file: %v", suffix, err)
			}
			t.Logf("Created %s%s with actual output", baseName, suffix)
			return
		}
		// No file and no autoCreate: expect empty output
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
