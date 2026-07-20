package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/interpreter"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/transpiler"
	"github.com/chzyer/readline"
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

		// Determine output path. It must be absolute: go build runs in the
		// temporary directory, where a relative -o would land (and be
		// removed along with it).
		out, _ := cmd.Flags().GetString("output")
		if out == "" {
			base := filepath.Base(sourceFile)
			out = strings.TrimSuffix(base, ".goblin")
		}
		if !filepath.IsAbs(out) {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			out = filepath.Join(cwd, out)
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

var runCmd = &cobra.Command{
	Use:   "run <source.goblin>",
	Short: "Interpret a Goblin source file directly (tree-walking interpreter)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
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

		return interpreter.Run(m, sourceFile)
	},
}

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start an interactive Goblin REPL",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return runREPL()
	},
}

// goblinHistoryPath returns the path to the persistent REPL history file,
// defaulting to ~/.goblin_history.
func goblinHistoryPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".goblin_history")
	}
	return ""
}

// runREPL drives a read-eval-print loop, accumulating lines until brackets are
// balanced so multi-line constructs (functions, types, blocks) can be entered.
// It uses readline for line editing, history, and Ctrl-C/Ctrl-D handling.
func runREPL() error {
	session := interpreter.NewSession(".")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          ">>> ",
		HistoryFile:     goblinHistoryPath(),
		HistoryLimit:    1000,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    &replCompleter{session: session},
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	out := rl.Stdout()
	fmt.Fprintln(out, "Goblin REPL. Press Ctrl-D to exit.")
	var buf strings.Builder
	for {
		if buf.Len() == 0 {
			rl.SetPrompt(">>> ")
		} else {
			rl.SetPrompt("... ")
		}

		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			// On Ctrl-C: drop any in-progress multi-line input; exit if buffer
			// was already empty and the user confirms by sending EOF.
			buf.Reset()
			continue
		}
		if err == io.EOF {
			if buf.Len() > 0 {
				evalLine(out, session, buf.String())
			}
			fmt.Fprintln(out)
			return nil
		}
		if err != nil {
			return err
		}

		buf.WriteString(line)
		buf.WriteByte('\n')
		// Keep reading until brackets balance, unless the line is blank (which
		// forces evaluation so the user can recover from a typo).
		if !bracketsBalanced(buf.String()) && strings.TrimSpace(line) != "" {
			continue
		}

		src := buf.String()
		buf.Reset()
		if strings.TrimSpace(src) == "" {
			continue
		}
		evalLine(out, session, src)
	}
}

var replKeywords = []string{
	"break", "catch", "continue", "else", "export", "false", "for", "func",
	"if", "import", "in", "nil", "raise", "return", "true", "try", "type",
	"var", "while",
}

type replCompleter struct {
	session *interpreter.Session
}

// Do completes ASCII Goblin identifiers and side-effect-free member paths.
// Readline expects each candidate to contain only the suffix that should be
// inserted at the cursor.
func (c *replCompleter) Do(line []rune, pos int) ([][]rune, int) {
	path, prefix, ok := completionPath(line, pos)
	if !ok {
		return nil, 0
	}

	names := c.session.CompletionCandidates(path)
	if len(path) == 0 {
		names = append(names, replKeywords...)
		sort.Strings(names)
	}

	candidates := make([][]rune, 0, len(names))
	previous := ""
	for _, name := range names {
		if name == previous || !strings.HasPrefix(name, prefix) {
			continue
		}
		previous = name
		candidates = append(candidates, []rune(name[len(prefix):]))
	}
	return candidates, len([]rune(prefix))
}

func completionPath(line []rune, pos int) (path []string, prefix string, ok bool) {
	if pos < 0 || pos > len(line) {
		return nil, "", false
	}
	start := pos
	for start > 0 && (isIdentifierRune(line[start-1]) || line[start-1] == '.') {
		start--
	}
	fragment := string(line[start:pos])
	parts := strings.Split(fragment, ".")
	for i, part := range parts {
		if part == "" && i != len(parts)-1 {
			return nil, "", false
		}
		if part != "" && !isIdentifierStart(rune(part[0])) {
			return nil, "", false
		}
	}
	return parts[:len(parts)-1], parts[len(parts)-1], true
}

func isIdentifierRune(r rune) bool {
	return isIdentifierStart(r) || r >= '0' && r <= '9'
}

func isIdentifierStart(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_'
}

func evalLine(out io.Writer, session *interpreter.Session, src string) {
	result, err := session.Eval(src)
	if err != nil {
		fmt.Fprintf(out, "%+v\n", err)
		return
	}
	// Display the value of an expression, but stay quiet for statements and
	// for `none` (e.g. the result of print()).
	if result == nil {
		return
	}
	if _, isUnit := result.(object.Unit); isUnit {
		return
	}
	fmt.Fprintln(out, result.String())
}

// bracketsBalanced reports whether all (), [], {} are closed, ignoring those
// inside string literals and # comments.
func bracketsBalanced(src string) bool {
	depth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			switch c {
			case '\\':
				i++
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
		}
	}
	return depth <= 0
}

func init() {
	buildExeCmd.Flags().StringP("output", "o", "", "output binary path (default: <source_name> in current directory)")
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
