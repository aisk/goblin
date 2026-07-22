package interpreter

import (
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

func TestRunForwardsArgv(t *testing.T) {
	// Run builds argv as [sourcePath] + scriptArgs; raise if the snapshot is wrong.
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
