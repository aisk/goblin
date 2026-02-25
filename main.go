package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	tmpDir, err := os.MkdirTemp("", "goblin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	err = transpiler.TranspileToDir(m, os.Args[1], tmpDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	base := filepath.Base(os.Args[1])
	binaryName := strings.TrimSuffix(base, ".goblin")

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	outputBin := filepath.Join(cwd, binaryName)

	cmd := exec.Command("go", "build", "-mod=mod", "-o", outputBin, ".")
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: go build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "built: %s\n", outputBin)
}
