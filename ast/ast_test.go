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

// TestTrailingComma checks that one comma may follow the last item of every
// comma-separated list, and that a comma with no item before it stays an
// error.
func TestTrailingComma(t *testing.T) {
	tests := []struct {
		source string
		ok     bool
	}{
		{"var a = [1, 2,]\n", true},
		{"var a = [\n    1,\n    2,\n]\n", true},
		{"var a = {\"k\": 1,}\n", true},
		{"var a = {\n    \"k\": 1,\n}\n", true},
		{"f(1, 2,)\n", true},
		{"f(\n    1,\n    key=2,\n)\n", true},
		{"f(*rest,)\n", true},
		{"func f(a, b = 1,) { }\n", true},
		{"func f(\n    a,\n    *rest,\n) { }\n", true},
		{"var f = func(a,) { }\n", true},
		{"type P(x, y = 0,) { }\n", true},
		{"type P(\n    x,\n    y,\n) { }\n", true},
		{"var a = [,]\n", false},
		{"var a = [1,,]\n", false},
		{"var a = {,}\n", false},
		{"f(,)\n", false},
		{"func f(,) { }\n", false},
		{"type P(,) { }\n", false},
		{"var a, b, = [1, 2]\n", false},
		{"for a, b, in [[1, 2]] { }\n", false},
	}

	for _, test := range tests {
		_, err := parser.NewParser().Parse(lexer.NewLexer([]byte(test.source)))
		if (err == nil) != test.ok {
			t.Errorf("%q: parsed = %v, want %v (err: %v)", test.source, err == nil, test.ok, err)
		}
	}
}
