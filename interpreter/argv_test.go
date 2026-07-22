package interpreter

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

func TestRunForwardsArgv(t *testing.T) {
	const source = `import "os"
for a in os.argv() {
    print(a)
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

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	runErr := Run(mod, "myscript.goblin", "foo", "bar")

	w.Close()
	os.Stdout = orig
	out := <-done

	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	want := "myscript.goblin\nfoo\nbar\n"
	if strings.ReplaceAll(out, "\r\n", "\n") != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}
