package main

import (
	"fmt"
	"os"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/transpiler"
)

func main() {
	var err error

	if len(os.Args) != 2 {
		println("Usage:  goblin <your_source_code.goblin>")
		os.Exit(1)
	}
	input, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to read file %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}

	l := lexer.NewLexer(input)
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	m, ok := st.(*ast.Module)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: internal error: unexpected AST type\n")
		os.Exit(1)
	}

	err = transpiler.TranspileToDir(m, os.Args[1], "output")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "output written to output/\n")
}
