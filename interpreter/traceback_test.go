package interpreter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/source"
)

func TestRunReturnsGoblinTraceback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.goblin")
	src := "func inner() {\n  return 1 / 0\n}\nfunc outer() {\n  return inner()\n}\nouter()\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	mod, err := source.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = Run(mod, path)
	if err == nil {
		t.Fatal("expected runtime error")
	}
	trace := fmt.Sprintf("%+v", err)
	moduleAt := strings.Index(trace, "at <module>")
	outerAt := strings.Index(trace, "at outer")
	innerAt := strings.Index(trace, "at inner")
	if moduleAt < 0 || outerAt <= moduleAt || innerAt <= outerAt {
		t.Fatalf("unexpected frame order:\n%s", trace)
	}
	if !strings.Contains(trace, path) || !strings.Contains(trace, "division by zero") {
		t.Fatalf("traceback lacks source or cause:\n%s", trace)
	}
	// Frames point at the failing statement in each function, not at the
	// function definitions: outer() on line 7, return inner() on line 5,
	// return 1 / 0 on line 2.
	for _, want := range []string{".goblin:7:", ".goblin:5:", ".goblin:2:"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("traceback lacks statement position %q:\n%s", want, trace)
		}
	}
}
