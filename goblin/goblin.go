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
			Value: &ast.Literal{Value: object.Integer(42)},
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
				&ast.Symbol{Name: "name"},
				&ast.Literal{Value: object.String("!")},
			},
		},
		// print("answer:", answer)
		&ast.FunctionCall{
			Name: "print",
			Args: []ast.Expression{
				&ast.Literal{Value: object.String("answer:")},
				&ast.Symbol{Name: "answer"},
			},
		},
		// if false { print("yes!") }
		&ast.If{
			Condition: &ast.Literal{Value: object.False},
			Body: []ast.Statement{
				&ast.FunctionCall{
					Name: "print",
					Args: []ast.Expression{
						&ast.Literal{Value: object.String("yes!")},
					},
				},
			},
		},
		// if answer { print("42!") }
		&ast.If{
			Condition: &ast.Symbol{Name: "answer"},
			Body: []ast.Statement{
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
			Name: "greetings",
			Args: []string{"name"},
			Body: []ast.Statement{
				&ast.FunctionCall{
					Name: "print",
					Args: []ast.Expression{
						&ast.Literal{Value: object.String("hello")},
						&ast.Symbol{Name: "name"},
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

	input := []byte(`print("hello, world!")`)
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
