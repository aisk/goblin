package source

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/parser"
)

// Parse runs the generated parser over the tokens produced by s and returns
// the resulting module. Every backend and the REPL parse through here.
func Parse(s parser.Scanner) (*ast.Module, error) {
	st, err := parser.NewParser().Parse(s)
	if err != nil {
		return nil, err
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		return nil, fmt.Errorf("internal error: unexpected AST type %T", st)
	}
	return mod, nil
}
