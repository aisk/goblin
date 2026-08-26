package ast_test

import (
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
)

// TestEmptyBlocks covers every construct whose body can reduce through
// `Statements : empty`, which yields a nil rather than an empty slice. Each
// one used to panic with an interface conversion error instead of producing a
// module with an empty body.
func TestEmptyBlocks(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "empty source", source: ""},
		{name: "comment only", source: "# nothing here\n"},
		{name: "if", source: "if true { }\n"},
		{name: "if else", source: "if true { } else { }\n"},
		{name: "else if chain", source: "if false { } else if true { } else { }\n"},
		{name: "while", source: "while false { }\n"},
		{name: "for", source: "for x in [1] { }\n"},
		{name: "try catch", source: "try { } catch e { }\n"},
		{name: "function", source: "func f() { }\n"},
		{name: "function literal", source: "var f = func() { }\n"},
		{name: "type", source: "type A(x) { }\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parser.NewParser().Parse(lexer.NewLexer([]byte(test.source)))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if _, ok := result.(*ast.Module); !ok {
				t.Fatalf("expected *ast.Module, got %T", result)
			}
		})
	}
}
