package interpreter

import (
	"fmt"
	"sort"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
)

// CompletionCandidates returns names available at the end of a simple member
// path. An empty path returns session globals and builtins. Non-empty paths are
// resolved through GetAttr only; functions are never called, so REPL
// completion cannot trigger arbitrary Goblin execution.
func (s *Session) CompletionCandidates(path []string) []string {
	if len(path) == 0 {
		names := make(map[string]struct{}, len(s.global.vars)+len(extension.BuiltinsModule.Members))
		for name := range extension.BuiltinsModule.Members {
			names[name] = struct{}{}
		}
		for name := range s.global.vars {
			names[name] = struct{}{}
		}
		return sortedNames(names)
	}

	value, ok := s.global.Get(path[0])
	if !ok {
		value, ok = extension.BuiltinsModule.Members[path[0]]
	}
	if !ok {
		return nil
	}
	for _, name := range path[1:] {
		var err error
		value, err = value.GetAttr(name)
		if err != nil {
			return nil
		}
	}
	return value.Attributes()
}

func sortedNames(names map[string]struct{}) []string {
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// Session is a persistent interpreter context. Unlike Run, it keeps its global
// scope and module registry across calls, so successive Eval invocations share
// state — the basis for a REPL.
type Session struct {
	global  *Environment
	reg     *object.Registry
	baseDir string
	argv    []string
}

// replArgv0 is os.argv()[0] inside the REPL, so interactive sessions do not
// expose the goblin binary's process arguments as script argv.
const replArgv0 = "<repl>"

// NewSession creates a session. baseDir is used to resolve relative imports.
func NewSession(baseDir string) *Session {
	return &Session{
		global:  NewEnvironment(nil),
		reg:     object.NewRegistry(),
		baseDir: baseDir,
		argv:    []string{replArgv0},
	}
}

// Eval parses and evaluates a source fragment against the session's scope. If
// the fragment's last statement is an expression, its value is returned (for
// REPL display); otherwise it returns nil.
func (s *Session) Eval(src string) (object.Object, error) {
	st, err := parser.NewParser().Parse(lexer.NewLexer([]byte(src)))
	if err != nil {
		// The grammar only accepts identifier-led expression statements, so a
		// fragment like `1 + 2` fails to parse as a statement. Retry it as a
		// bare expression for REPL display.
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
	if err := loadInto(mod, s.global, s.baseDir, s.reg, s.argv); err != nil {
		return nil, err
	}

	var result object.Object
	for _, stmt := range mod.Body {
		if expr, ok := stmt.(ast.Expression); ok {
			v, err := evalExpr(expr, s.global)
			if err != nil {
				return nil, object.WithFrame(err, stackFrame("repl", "<module>", expr.Position()))
			}
			result = v
		} else {
			if err := evalStatement(stmt, s.global); err != nil {
				return nil, object.WithFrame(err, stackFrame("repl", "<module>", stmt.Position()))
			}
			result = nil
		}
	}
	return result, nil
}

// evalAsExpression evaluates src as a bare expression. Because the grammar
// rejects most bare expressions in statement position, we coerce parsing by
// wrapping the fragment in a `return` statement — which accepts any
// expression — then evaluate the extracted expression against the live scope.
// This leaves no trace in the session's scope (unlike binding to a throwaway
// variable). The parsed return value reports whether the wrapped form parsed;
// when false, callers should surface the original error instead.
func (s *Session) evalAsExpression(src string) (value object.Object, evalErr error, parsed bool) {
	st, err := parser.NewParser().Parse(lexer.NewLexer([]byte("return " + src)))
	if err != nil {
		return nil, nil, false
	}
	mod, ok := st.(*ast.Module)
	if !ok || len(mod.Body) != 1 {
		return nil, nil, false
	}
	ret, ok := mod.Body[0].(*ast.Return)
	if !ok {
		return nil, nil, false
	}
	v, evalErr := evalExpr(ret.Value, s.global)
	return v, evalErr, true
}
