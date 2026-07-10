package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLineDirective_GeneratedSource confirms that transpilation emits Go
// `//line` directives that map back to the .goblin source file with an
// absolute path.
func TestLineDirective_GeneratedSource(t *testing.T) {
	goblinFile, err := filepath.Abs(filepath.Join("..", "examples", "arithmetic.goblin"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	goCode := parseAndTranspile(t, goblinFile)

	re := regexp.MustCompile(`//line\s+(\S+\.goblin):(\d+):(\d+)`)
	matches := re.FindAllStringSubmatch(goCode, -1)
	if len(matches) == 0 {
		t.Fatalf("no //line directive found in generated source:\n%s", goCode)
	}
	for _, m := range matches {
		path := m[1]
		if !filepath.IsAbs(path) {
			t.Errorf("//line path is not absolute: %q", path)
		}
		if !strings.HasSuffix(path, "arithmetic.goblin") {
			t.Errorf("//line path does not point to source: %q", path)
		}
	}
}

// TestLineDirective_PanicTraceback confirms that a runtime error in the
// transpiled program produces a stack trace that references the original
// .goblin source file (not the temp .go file).
func TestLineDirective_PanicTraceback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goblin-line-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goblinSrc := "func inner() {\n  print(1 / 0)\n}\nfunc outer() {\n  inner()\n}\nouter()\n"
	goblinPath := filepath.Join(tempDir, "divzero.goblin")
	if err := os.WriteFile(goblinPath, []byte(goblinSrc), 0644); err != nil {
		t.Fatalf("write goblin: %v", err)
	}

	goCode := parseAndTranspile(t, goblinPath)
	goPath := filepath.Join(tempDir, "divzero.go")
	if err := os.WriteFile(goPath, []byte(goCode), 0644); err != nil {
		t.Fatalf("write go: %v", err)
	}

	cmd := exec.Command("go", "run", goPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr == nil {
		t.Fatalf("expected non-zero exit, got success.\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}
	combined := stderr.String() + stdout.String()
	if !strings.Contains(combined, "division by zero") {
		t.Errorf("expected 'division by zero' in output, got: %s", combined)
	}
	if !strings.Contains(combined, "divzero.goblin") {
		t.Errorf("expected stack trace to reference divzero.goblin, got: %s", combined)
	}
	moduleAt := strings.Index(combined, "at <module>")
	outerAt := strings.Index(combined, "at outer")
	innerAt := strings.Index(combined, "at inner")
	if moduleAt < 0 || outerAt <= moduleAt || innerAt <= outerAt {
		t.Errorf("expected Goblin function frames in call order, got: %s", combined)
	}
}
