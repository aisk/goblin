// Package source turns Goblin source text into a checked syntax tree. Every
// way of loading code (the CLI, imports in both backends, the REPL, tests)
// goes through here, so they all normalize, parse and check the same way.
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

// ReadError reports that a source file could not be read, as opposed to
// failing to parse or check.
type ReadError struct {
	Path string
	Err  error
}

func (e *ReadError) Error() string { return fmt.Sprintf("failed to read %s: %v", e.Path, e.Err) }
func (e *ReadError) Unwrap() error { return e.Err }

// LoadFile reads, parses and semantically checks a source file. The backends
// expect modules that went through here (the REPL is the one unchecked
// exception).
func LoadFile(path string) (*ast.Module, error) {
	mod, err := ParseFile(path)
	if err != nil {
		return nil, err
	}
	if err := semantic.CheckModule(mod); err != nil {
		return nil, err
	}
	return mod, nil
}

// ParseFile reads and parses a source file without checking it. Positions in
// the tree and in parse errors carry the file path.
func ParseFile(path string) (*ast.Module, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, &ReadError{Path: path, Err: err}
	}
	l := newLexer(src)
	l.Context = &lexer.SourceContext{Filepath: path}
	return parse(l)
}

// ParseString parses source text without checking it.
func ParseString(src string) (*ast.Module, error) {
	return parse(newLexer([]byte(src)))
}

// ModuleName derives the module display name from a source file path,
// e.g. "/path/to/mymod.goblin" → "mymod". Both backends use it for
// object.Module.Name and traceback frame module tags, so the two always agree.
func ModuleName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// newLexer appends a trailing newline when the source lacks one. The
// grammar's comment rule can only terminate on '\n' (gocc's generated lexer
// ends an ignored token at its first accepting state, so the rule cannot
// instead accept at end-of-file), which would otherwise make a comment on the
// last line a lex error.
func newLexer(src []byte) *lexer.Lexer {
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(src, '\n')
	}
	return lexer.NewLexer(src)
}

func parse(s parser.Scanner) (*ast.Module, error) {
	st, err := parser.NewParser().Parse(s)
	if err != nil {
		return nil, err
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		return nil, fmt.Errorf("internal error: unexpected AST type %T", st)
	}
	return mod, nil
}
