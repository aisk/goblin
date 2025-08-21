package examples_test

import (
	"bytes"
	"io/ioutil"
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

// TestExamples runs tests for all .goblin files in the examples directory
func TestExamples(t *testing.T) {
	examplesDir := ".."
	
	files, err := filepath.Glob(filepath.Join(examplesDir, "examples", "*.goblin"))
	if err != nil {
		t.Fatalf("failed to find .goblin files: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("no .goblin files found in %s", filepath.Join(examplesDir, "examples"))
	}

	t.Logf("Found %d .goblin files to test", len(files))

	// Create temporary directory for compiled Go files
	tempDir, err := ioutil.TempDir("", "goblin-test-")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	for _, file := range files {
		baseName := strings.TrimSuffix(filepath.Base(file), ".goblin")
		t.Run(baseName, func(t *testing.T) {
			runExampleTest(t, file, tempDir)
		})
	}
}

// runExampleTest runs a single test for a Goblin file
func runExampleTest(t *testing.T, goblinFile string, tempDir string) {
	baseName := strings.TrimSuffix(filepath.Base(goblinFile), ".goblin")
	examplesDir := filepath.Join("..", "examples")
	
	// Read the Goblin source
	source, err := ioutil.ReadFile(goblinFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Parse the Goblin code
	l := lexer.NewLexer(source)
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Convert to Module and transpile to Go
	module, ok := st.(*ast.Module)
	if !ok {
		t.Fatalf("failed to convert AST to Module")
	}

	var buf bytes.Buffer
	err = transpiler.Transpile(module, &buf)
	if err != nil {
		t.Fatalf("transpile error: %v", err)
	}
	goCode := buf.String()

	// Write the Go file to temp directory
	goFile := filepath.Join(tempDir, baseName+".go")
	if err := ioutil.WriteFile(goFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write Go file: %v", err)
	}

	// Run the Go file and capture output
	stdout, stderr := runGoFile(t, goFile)

	// Compare with expected output
	expectedStdoutFile := filepath.Join(examplesDir, baseName+".stdout")
	expectedStderrFile := filepath.Join(examplesDir, baseName+".stderr")

	// Check if expected files exist, create them if they don't
	if _, err := os.Stat(expectedStdoutFile); os.IsNotExist(err) {
		// Create .stdout file with actual output
		if err := ioutil.WriteFile(expectedStdoutFile, stdout, 0644); err != nil {
			t.Fatalf("failed to create .stdout file: %v", err)
		}
		t.Logf("Created %s with actual output", baseName+".stdout")
		return // Skip comparison for newly created files
	}

	if _, err := os.Stat(expectedStderrFile); os.IsNotExist(err) {
		// Create .stderr file with actual output
		if err := ioutil.WriteFile(expectedStderrFile, stderr, 0644); err != nil {
			t.Fatalf("failed to create .stderr file: %v", err)
		}
		t.Logf("Created %s with actual output", baseName+".stderr")
		return // Skip comparison for newly created files
	}

	// Read expected output
	expectedStdout, err := ioutil.ReadFile(expectedStdoutFile)
	if err != nil {
		t.Fatalf("failed to read .stdout file: %v", err)
	}

	expectedStderr, err := ioutil.ReadFile(expectedStderrFile)
	if err != nil {
		t.Fatalf("failed to read .stderr file: %v", err)
	}

	// Compare outputs
	if string(stdout) != string(expectedStdout) {
		t.Errorf("stdout mismatch:\nExpected:\n%s\nActual:\n%s", 
			string(expectedStdout), string(stdout))
	}

	if string(stderr) != string(expectedStderr) {
		t.Errorf("stderr mismatch:\nExpected:\n%s\nActual:\n%s", 
			string(expectedStderr), string(stderr))
	}
}

// runGoFile runs a Go file and captures stdout and stderr
func runGoFile(t *testing.T, goFile string) ([]byte, []byte) {
	cmd := exec.Command("go", "run", goFile)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run failed: %v", err)
	}
	
	return stdout.Bytes(), stderr.Bytes()
}