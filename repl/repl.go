// Package repl implements goblin's interactive read-eval-print loop on top of
// an interpreter.Session.
package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aisk/goblin/interpreter"
	"github.com/aisk/goblin/object"
	"github.com/chzyer/readline"
)

// HistoryPath returns the default location of the persistent history file,
// ~/.goblin_history, or "" when the home directory is unknown.
func HistoryPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".goblin_history")
	}
	return ""
}

// Run drives a read-eval-print loop on the terminal, accumulating lines until
// brackets are balanced so multi-line constructs (functions, types, blocks)
// can be entered. It uses readline for line editing, history (kept in
// historyFile unless it is empty), and Ctrl-C/Ctrl-D handling.
func Run(historyFile string) error {
	session := interpreter.NewSession(".")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          ">>> ",
		HistoryFile:     historyFile,
		HistoryLimit:    1000,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    &completer{session: session},
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

// keywords are offered as completions at the start of a path. A test keeps the
// list in sync with the grammar.
var keywords = []string{
	"break", "catch", "continue", "else", "export", "false", "for", "func",
	"if", "impl", "import", "in", "nil", "raise", "return", "trait", "true",
	"try", "type", "var", "while",
}

type completer struct {
	session *interpreter.Session
}

// Do completes ASCII Goblin identifiers and side-effect-free member paths.
// Readline expects each candidate to contain only the suffix that should be
// inserted at the cursor.
func (c *completer) Do(line []rune, pos int) ([][]rune, int) {
	path, prefix, ok := completionPath(line, pos)
	if !ok {
		return nil, 0
	}

	names := c.session.CompletionCandidates(path)
	if len(path) == 0 {
		names = append(names, keywords...)
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
	fmt.Fprintln(out, fmt.Sprint(result))
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
