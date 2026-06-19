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
// interpreter and compares its stdout against the corresponding .stdout file
// (the same expected output the transpiler is checked against).
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
			expectedFile := baseName + ".stdout"
			expected, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Skipf("no expected stdout (%s): %v", expectedFile, err)
			}

			stdout := runInterpreter(t, file)
			if normalize(stdout) != normalize(string(expected)) {
				t.Errorf("stdout mismatch:\nExpected:\n%s\nActual:\n%s", string(expected), stdout)
			}
		})
	}
}

// runInterpreter parses and interprets a file, capturing everything written to
// os.Stdout.
func runInterpreter(t *testing.T, goblinFile string) string {
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

	// Capture os.Stdout (built-in print writes there via fmt.Print).
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	runErr := interpreter.Run(module, goblinFile)

	w.Close()
	os.Stdout = orig
	out := <-done

	if runErr != nil {
		t.Fatalf("interpreter error: %v", runErr)
	}
	return out
}
