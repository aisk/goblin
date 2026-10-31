package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
)

// TestLineDirective_GeneratedSource confirms that transpilation emits Go
// `//line` directives that map back to the .goblin source file with an
// absolute path.
func TestLineDirective_GeneratedSource(t *testing.T) {
	mod, err := source.LoadFile("arithmetic.goblin")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := transpiler.TranspileToDir(mod, "arithmetic.goblin", dir); err != nil {
		t.Fatal(err)
	}
	goCode, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	re := regexp.MustCompile(`//line\s+(\S+\.goblin):(\d+):(\d+)`)
	matches := re.FindAllStringSubmatch(string(goCode), -1)
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
// compiled program produces a traceback that references the original .goblin
// source file (not the generated .go file).
func TestLineDirective_PanicTraceback(t *testing.T) {
	dir := t.TempDir()
	goblinSrc := "func inner() {\n  print(1 / 0)\n}\nfunc outer() {\n  inner()\n}\nouter()\n"
	goblinPath := filepath.Join(dir, "divzero.goblin")
	if err := os.WriteFile(goblinPath, []byte(goblinSrc), 0644); err != nil {
		t.Fatalf("write goblin: %v", err)
	}
	mod, err := source.LoadFile(goblinPath)
	if err != nil {
		t.Fatal(err)
	}
	var buildLog bytes.Buffer
	exe, err := transpiler.BuildExecutable(mod, goblinPath, transpiler.BuildOptions{
		Output: filepath.Join(dir, "divzero"),
		Stdout: &buildLog,
		Stderr: &buildLog,
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, buildLog.String())
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(exe)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
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
