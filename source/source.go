// Package source turns Goblin source text into a checked syntax tree. Every
// way of loading code (the CLI, imports in both backends, the REPL, tests)
// goes through here, so they all normalize, parse and check the same way.
package source

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/ast"
	parseErrors "github.com/aisk/goblin/errors"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/token"
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

// parse parses a whole module, including the expressions interpolated into
// its string literals.
func parse(s parser.Scanner) (*ast.Module, error) {
	mod, err := parseModule(s)
	if err != nil {
		return nil, err
	}
	if err := resolveInterpolations(mod); err != nil {
		return nil, err
	}
	return mod, nil
}

func parseModule(s parser.Scanner) (*ast.Module, error) {
	st, err := parser.NewParser().Parse(s)
	if err != nil {
		// The parser reports a reduce error at the token after the
		// construct; one that knows its exact position is moved there.
		var perr *parseErrors.Error
		var serr *ast.SyntaxError
		if errors.As(err, &perr) && errors.As(perr.Err, &serr) {
			pos := serr.Pos
			perr.ErrorToken = &token.Token{Type: perr.ErrorToken.Type, Lit: perr.ErrorToken.Lit, Pos: pos}
		}
		return nil, err
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		return nil, fmt.Errorf("internal error: unexpected AST type %T", st)
	}
	return mod, nil
}

// resolveInterpolations parses the source of every expression interpolated
// into a string literal and puts the expression in its place. The source is
// parsed by the same parser as a module of one expression statement, padded
// so that it sits at the line and column it has in the file: positions in
// the expression, and parse errors in it, then point into the literal.
func resolveInterpolations(mod *ast.Module) error {
	var err error
	ast.InspectList(mod.Body, func(n ast.Statement) bool {
		interp, ok := n.(*ast.Interpolation)
		if err != nil || !ok {
			return err == nil
		}
		for i, part := range interp.Parts {
			src, ok := part.(*ast.InterpolationSource)
			if !ok {
				continue
			}
			var expr ast.Expression
			if expr, err = parseInterpolated(src); err != nil {
				return false
			}
			if src.Spec != nil {
				// `{x:spec}` is format(x, "spec"), with the spec checked
				// already; the node sits where the expression does.
				expr = ast.NewFormat(expr, *src.Spec)
			}
			interp.Parts[i] = expr
		}
		return true
	})
	return err
}

func parseInterpolated(src *ast.InterpolationSource) (ast.Expression, error) {
	pos := src.Position()
	padded := strings.Repeat("\n", pos.Line-1) + strings.Repeat(" ", pos.Column-1) + src.Source + "\n"
	l := lexer.NewLexer([]byte(padded))
	l.Context = pos.Context
	mod, err := parseModule(l)
	if err != nil {
		// An expression cut short (`{a +}`) fails at the end of the padded
		// source, past the literal; report it at the closing brace.
		var perr *parseErrors.Error
		if errors.As(err, &perr) && perr.ErrorToken.Pos.Line > pos.Line {
			end := pos
			for _, r := range src.Source {
				if r == '\t' {
					end.Column += 4
				} else {
					end.Column++
				}
			}
			end.Offset += len(src.Source)
			perr.ErrorToken = &token.Token{Type: perr.ErrorToken.Type, Lit: perr.ErrorToken.Lit, Pos: end}
		}
		return nil, err
	}
	if len(mod.Body) == 1 {
		if expr, ok := mod.Body[0].(ast.Expression); ok {
			return expr, nil
		}
	}
	return nil, &parseErrors.Error{
		Err:        errors.New("an interpolation must be a single expression"),
		ErrorToken: &token.Token{Pos: pos},
	}
}
