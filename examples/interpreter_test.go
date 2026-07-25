package examples_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/interpreter"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

// TestInterpreterExamples runs each .goblin file through the tree-walking
// interpreter and compares stdout/stderr against the corresponding golden
// files (the same expected output the transpiler is checked against).
func TestInterpreterExamples(t *testing.T) {
	files, err := filepath.Glob("*.goblin")
	if err != nil {
		t.Fatalf("failed to find .goblin files: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no .goblin files found")
	}

	for _, file := range files {
		baseName := strings.TrimSuffix(filepath.Base(file), ".goblin")
		t.Run(baseName, func(t *testing.T) {
			stdout, stderr := runInterpreter(t, file)
			checkOutput(t, ".", baseName, ".stdout", stdout, true)
			checkOutput(t, ".", baseName, ".stderr", stderr, false)
		})
	}
}

// runInterpreter parses and interprets a file, capturing os.Stdout and os.Stderr.
func runInterpreter(t *testing.T, goblinFile string) (stdout, stderr string) {
	t.Helper()

	l, err := lexer.NewLexerFile(goblinFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	st, err := parser.NewParser().Parse(l)
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

	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() {
		os.Stdout, os.Stderr = origOut, origErr
	}()

	outDone := make(chan string)
	errDone := make(chan string)
	go func() {
		data, _ := io.ReadAll(outR)
		outDone <- string(data)
	}()
	go func() {
		data, _ := io.ReadAll(errR)
		errDone <- string(data)
	}()

	runErr := interpreter.Run(module, goblinFile)

	outW.Close()
	errW.Close()
	stdout = <-outDone
	stderr = <-errDone

	if runErr != nil {
		t.Fatalf("interpreter error: %v\nstderr: %s", runErr, stderr)
	}
	return stdout, stderr
}
