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
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/transpiler"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "goblin",
	Short: "Goblin is a programming language that transpiles to Go",
	Long:  "Goblin is a programming language that transpiles to Go.",
}

var buildExeCmd = &cobra.Command{
	Use:   "build-exe <source.goblin>",
	Short: "Compile a Goblin source file to a native executable",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceFile := args[0]

		l, err := lexer.NewLexerFile(sourceFile)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", sourceFile, err)
		}

		p := parser.NewParser()
		st, err := p.Parse(l)
		if err != nil {
			return err
		}
		m, ok := st.(*ast.Module)
		if !ok {
			return fmt.Errorf("internal error: unexpected AST type")
		}
		if err := semantic.CheckModule(m); err != nil {
			return err
		}

		tmpDir, err := os.MkdirTemp("", "goblin-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		err = transpiler.TranspileToDir(m, sourceFile, tmpDir)
		if err != nil {
			return err
		}

		// Determine output path.
		out, _ := cmd.Flags().GetString("output")
		if out == "" {
			base := filepath.Base(sourceFile)
			binaryName := strings.TrimSuffix(base, ".goblin")
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			out = filepath.Join(cwd, binaryName)
		}

		goBuild := exec.Command("go", "build", "-mod=mod", "-o", out, ".")
		goBuild.Dir = tmpDir
		goBuild.Stdout = os.Stdout
		goBuild.Stderr = os.Stderr
		fmt.Fprintf(os.Stderr, "build dir: %s\n", tmpDir)
		fmt.Fprintf(os.Stderr, "run: (cd %s && %s)\n", tmpDir, strings.Join(goBuild.Args, " "))
		if err = goBuild.Run(); err != nil {
			return fmt.Errorf("go build failed: %w", err)
		}

		fmt.Fprintf(os.Stderr, "built: %s\n", out)
		return nil
	},
}

func init() {
	buildExeCmd.Flags().StringP("output", "o", "", "output binary path (default: <source_name> in current directory)")
	rootCmd.AddCommand(buildExeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
