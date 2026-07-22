package interpreter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

func TestRunForwardsArgv(t *testing.T) {
	const source = `import "os"
var a = os.argv()
if a.size() != 3 {
    raise Error("bad size")
}
if a[0] != "myscript.goblin" {
    raise Error("bad 0")
}
if a[1] != "foo" {
    raise Error("bad 1")
}
if a[2] != "bar" {
    raise Error("bad 2")
}
`
	st, err := parser.NewParser().Parse(lexer.NewLexer([]byte(source)))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		t.Fatalf("unexpected AST type %T", st)
	}
	if err := semantic.CheckModule(mod); err != nil {
		t.Fatalf("semantic error: %v", err)
	}

	if err := Run(mod, "myscript.goblin", "foo", "bar"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestImportedModuleSeesEntryArgv(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "dep.goblin")
	if err := os.WriteFile(dep, []byte(`import "os"
var a = os.argv()
if a.size() != 2 || a[1] != "from-entry" {
    raise Error("dependency saw the wrong argv")
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	const source = `import "./dep"`
	st, err := parser.NewParser().Parse(lexer.NewLexer([]byte(source)))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	mod := st.(*ast.Module)
	if err := semantic.CheckModule(mod); err != nil {
		t.Fatalf("semantic error: %v", err)
	}

	if err := Run(mod, filepath.Join(dir, "main.goblin"), "from-entry"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
