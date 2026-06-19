package interpreter

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
)

// Session is a persistent interpreter context. Unlike Run, it keeps its global
// scope and module registry across calls, so successive Eval invocations share
// state — the basis for a REPL.
type Session struct {
	global  *Environment
	reg     *object.Registry
	baseDir string
}

// NewSession creates a session. baseDir is used to resolve relative imports.
func NewSession(baseDir string) *Session {
	return &Session{
		global:  NewEnvironment(nil),
		reg:     object.NewRegistry(),
		baseDir: baseDir,
	}
}

// replResultVar holds the value of a bare expression entered at the REPL. The
// grammar only accepts identifier-led expression statements, so a fragment like
// `1 + 2` is evaluated by wrapping it as an assignment to this variable.
const replResultVar = "__repl_result__"

// Eval parses and evaluates a source fragment against the session's scope. If
// the fragment's last statement is an expression, its value is returned (for
// REPL display); otherwise it returns nil.
func (s *Session) Eval(src string) (object.Object, error) {
	st, err := parser.NewParser().Parse(lexer.NewLexer([]byte(src)))
	if err != nil {
		// The fragment isn't a valid statement; try evaluating it as a bare
		// expression by binding it to a throwaway variable.
		if v, evalErr, parsed := s.evalAsExpression(src); parsed {
			return v, evalErr
		}
		return nil, err
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		return nil, fmt.Errorf("internal error: unexpected AST type")
	}
	// No static semantic check here: it analyses a fragment in isolation and
	// would reject references to names declared on earlier REPL lines. The
	// interpreter reports undefined names at runtime against the live scope.

	// Resolve imports and hoist definitions into the persistent scope.
	if err := loadInto(mod, s.global, s.baseDir, s.reg); err != nil {
		return nil, err
	}

	var result object.Object
	for _, stmt := range mod.Body {
		if expr, ok := stmt.(ast.Expression); ok {
			v, err := evalExpr(expr, s.global)
			if err != nil {
				return nil, err
			}
			result = v
		} else {
			if err := evalStatement(stmt, s.global); err != nil {
				return nil, err
			}
			result = nil
		}
	}
	return result, nil
}

// evalAsExpression evaluates src as a bare expression by wrapping it in an
// assignment. The parsed return value reports whether the wrapped form parsed;
// when false, callers should surface the original error instead.
func (s *Session) evalAsExpression(src string) (value object.Object, evalErr error, parsed bool) {
	wrapped := replResultVar + " = " + src
	st, err := parser.NewParser().Parse(lexer.NewLexer([]byte(wrapped)))
	if err != nil {
		return nil, nil, false
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		return nil, nil, false
	}
	for _, stmt := range mod.Body {
		if err := evalStatement(stmt, s.global); err != nil {
			return nil, err, true
		}
	}
	v, _ := s.global.Get(replResultVar)
	return v, nil, true
}
