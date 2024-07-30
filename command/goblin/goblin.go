package main

import (
	"log"
	"os"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/transpiler"
)

var hello = ast.Module{
	Name: "main",
	Body: []ast.Statement{
		&ast.Declare{
			Name:  "answer",
			Value: &ast.Literal{Value: object.Integer(0)},
		},
		&ast.Assign{
			Target: "answer",
			Value:  &ast.Literal{Value: object.Integer(42)},
		},
		&ast.Declare{
			Name:  "name",
			Value: &ast.Literal{Value: object.String("jim")},
		},
		// print("hello,", name, "!")
		&ast.FunctionCall{
			Name: "print",
			Args: []ast.Expression{
				&ast.Literal{Value: object.String("hello,")},
				&ast.Identifier{Name: "name"},
				&ast.Literal{Value: object.String("!")},
			},
		},
		// print("answer:", answer)
		&ast.FunctionCall{
			Name: "print",
			Args: []ast.Expression{
				&ast.Literal{Value: object.String("answer:")},
				&ast.Identifier{Name: "answer"},
			},
		},
		// if false { print("yes!") } else { print("no!") }
		&ast.IfElse{
			Condition: &ast.Literal{Value: object.False},
			IfBody: []ast.Statement{
				&ast.FunctionCall{
					Name: "print",
					Args: []ast.Expression{
						&ast.Literal{Value: object.String("yes!")},
					},
				},
			},
			ElseBody: []ast.Statement{
				&ast.FunctionCall{
					Name: "print",
					Args: []ast.Expression{
						&ast.Literal{Value: object.String("no!")},
					},
				},
			},
		},
		// if answer { print("42!") }
		&ast.IfElse{
			Condition: &ast.Identifier{Name: "answer"},
			IfBody: []ast.Statement{
				&ast.FunctionCall{
					Name: "print",
					Args: []ast.Expression{
						&ast.Literal{Value: object.String("42!")},
					},
				},
			},
		},
		// while (false) { print("impossible") }
		&ast.While{
			Condition: &ast.Literal{Value: object.False},
			Body: []ast.Statement{
				&ast.FunctionCall{
					Name: "print",
					Args: []ast.Expression{
						&ast.Literal{Value: object.String("impossible")},
					},
				},
			},
		},
		&ast.FunctionDefine{
			Name:       "greetings",
			Parameters: []string{"name"},
			Body: []ast.Statement{
				&ast.FunctionCall{
					Name: "print",
					Args: []ast.Expression{
						&ast.Literal{Value: object.String("hello")},
						&ast.Identifier{Name: "name"},
					},
				},
				&ast.Return{Value: &ast.Literal{Value: object.True}},
			},
		},
		// grettings("jim")
		&ast.FunctionCall{
			Name: "greetings",
			Args: []ast.Expression{&ast.Literal{Value: object.String("jim")}},
		},
		// print(greetings("jim"))
		&ast.FunctionCall{
			Name: "print",
			Args: []ast.Expression{
				&ast.FunctionCall{
					Name: "greetings",
					Args: []ast.Expression{&ast.Literal{Value: object.String("jim")}},
				},
			},
		},
	},
}

func main() {
	var err error

	if len(os.Args) != 2 {
		println("Usage:  goblin <your_source_code.goblin>")
		os.Exit(1)
	}
	input, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	l := lexer.NewLexer(input)
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		panic(err)
	}
	m, ok := st.(*ast.Module)
	if !ok {
		panic("not ok!")
	}

	err = transpiler.Transpile(m, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}
