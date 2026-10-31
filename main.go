package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aisk/goblin/interpreter"
	"github.com/aisk/goblin/repl"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "goblin",
	Short:         "Goblin is a programming language that transpiles to Go",
	Long:          "Goblin is a programming language that transpiles to Go.",
	SilenceErrors: true,
}

var buildExeCmd = &cobra.Command{
	Use:   "build-exe <source.goblin>",
	Short: "Compile a Goblin source file to a native executable",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		sourceFile := args[0]

		m, err := source.LoadFile(sourceFile)
		if err != nil {
			return err
		}

		output, _ := cmd.Flags().GetString("output")
		verbose, _ := cmd.Flags().GetBool("verbose")
		race, _ := cmd.Flags().GetBool("race")
		out, err := transpiler.BuildExecutable(m, sourceFile, transpiler.BuildOptions{
			Output:  output,
			Race:    race,
			Verbose: verbose,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "built: %s\n", out)
		return nil
	},
}

var runCmd = &cobra.Command{
	Use:   "run <source.goblin> [args...]",
	Short: "Interpret a Goblin source file directly (tree-walking interpreter)",
	Long: `Interpret a Goblin source file directly (tree-walking interpreter).

The first argument must be the source file. All arguments after it are
forwarded to the script as os.argv(), including flag-like values such as
-h or --verbose. Leading flags are rejected. CLI help is "goblin run -h"
or "goblin help run" (alone, with no source file).`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if wantsHelp(args) {
			return cmd.Help()
		}
		if err := requireSourceFirst(args); err != nil {
			return err
		}
		sourceFile := args[0]
		scriptArgs := args[1:]

		m, err := source.LoadFile(sourceFile)
		if err != nil {
			return err
		}

		return interpreter.Run(m, sourceFile, scriptArgs...)
	},
}

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start an interactive Goblin REPL",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return repl.Run(repl.HistoryPath())
	},
}

// wantsHelp reports whether args are only -h/--help (goblin run's own help).
// Flags after the source file are forwarded to the script; leading flags are
// rejected by requireSourceFirst.
func wantsHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "--help")
}

// requireSourceFirst requires the first argument to be a source path, not a flag.
// Flag-like values belong after the source file so they reach os.argv().
func requireSourceFirst(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("source file required")
	}
	if strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("source file required before script arguments; got %q (flag-like). Use \"goblin run -h\" or \"goblin help run\" for help", args[0])
	}
	return nil
}

func init() {
	buildExeCmd.Flags().StringP("output", "o", "", "output binary path (default: <source_name> in current directory)")
	buildExeCmd.Flags().BoolP("verbose", "v", false, "print the temporary build directory and go build command")
	buildExeCmd.Flags().Bool("race", false, "build with the Go race detector enabled")
	rootCmd.AddCommand(buildExeCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(replCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
