package interpreter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

func TestRunReturnsGoblinTraceback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.goblin")
	source := "func inner() {\n  return 1 / 0\n}\nfunc outer() {\n  return inner()\n}\nouter()\n"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	l, err := lexer.NewLexerFile(path)
	if err != nil {
		t.Fatal(err)
	}
	node, err := parser.NewParser().Parse(l)
	if err != nil {
		t.Fatal(err)
	}
	mod := node.(*ast.Module)
	if err := semantic.CheckModule(mod); err != nil {
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
}
