package semantic

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/token"
)

type Diagnostic struct {
	Pos     token.Pos
	Kind    string
	Message string
}

type Error struct {
	Diagnostic Diagnostic
}

func (e *Error) Error() string {
	diag := e.Diagnostic
	return fmt.Sprintf("%s: semantic error: %s", formatPos(diag.Pos), diag.Message)
}

type symbol struct {
	name string
}

type scope struct {
	parent  *scope
	symbols map[string]symbol
}

func newScope(parent *scope) *scope {
	return &scope{
		parent:  parent,
		symbols: make(map[string]symbol),
	}
}

func (s *scope) declare(name string) bool {
	if _, exists := s.symbols[name]; exists {
		return false
	}
	s.symbols[name] = symbol{name: name}
	return true
}

func (s *scope) lookup(name string) bool {
	for cur := s; cur != nil; cur = cur.parent {
		if _, exists := cur.symbols[name]; exists {
			return true
		}
	}
	return false
}

type checker struct {
	currentScope *scope
	loopDepth    int
	funcDepth    int
}

func CheckModule(mod *ast.Module) error {
	c := &checker{
		currentScope: newScope(nil),
	}

	// Import names are available across the whole module in current transpiler behavior.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok {
			continue
		}
		if !c.currentScope.declare(imp.Name) {
			return c.newError(imp.Position(), "duplicate declaration in same scope: %s", imp.Name)
		}
	}

	return c.checkStatements(mod.Body, true)
}

func (c *checker) checkStatements(stmts []ast.Statement, isModuleScope bool) error {
	for _, stmt := range stmts {
		if err := c.checkStatement(stmt, isModuleScope); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) withScope(fn func() error) error {
	prev := c.currentScope
	c.currentScope = newScope(prev)
	defer func() {
		c.currentScope = prev
	}()
	return fn()
}

func (c *checker) checkStatement(stmt ast.Statement, isModuleScope bool) error {
	switch v := stmt.(type) {
	case *ast.Import:
		if !isModuleScope {
			return c.newError(v.Position(), "import is only allowed at module scope")
		}
		return nil
	case *ast.Declare:
		if err := c.checkExpression(v.Value); err != nil {
			return err
		}
		if !c.currentScope.declare(v.Name) {
			return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Name)
		}
		return nil
	case *ast.Assign:
		if !c.currentScope.lookup(v.Target) {
			return c.newError(v.Position(), "assignment to undefined identifier: %s", v.Target)
		}
		return c.checkExpression(v.Value)
	case *ast.FunctionDefine:
		if !c.currentScope.declare(v.Name) {
			return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Name)
		}
		return c.withScope(func() error {
			c.funcDepth++
			defer func() { c.funcDepth-- }()

			for _, param := range v.Parameters {
				if !c.currentScope.declare(param) {
					return c.newError(v.Position(), "duplicate parameter name: %s", param)
				}
			}

			return c.checkStatements(v.Body, false)
		})
	case *ast.IfElse:
		if err := c.checkExpression(v.Condition); err != nil {
			return err
		}
		if err := c.withScope(func() error {
			return c.checkStatements(v.IfBody, false)
		}); err != nil {
			return err
		}
		return c.withScope(func() error {
			return c.checkStatements(v.ElseBody, false)
		})
	case *ast.While:
		if err := c.checkExpression(v.Condition); err != nil {
			return err
		}
		return c.withScope(func() error {
			c.loopDepth++
			defer func() { c.loopDepth-- }()
			return c.checkStatements(v.Body, false)
		})
	case *ast.For:
		if err := c.checkExpression(v.Iterator); err != nil {
			return err
		}
		return c.withScope(func() error {
			c.loopDepth++
			defer func() { c.loopDepth-- }()

			if !c.currentScope.declare(v.Variable) {
				return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Variable)
			}
			return c.checkStatements(v.Body, false)
		})
	case *ast.Break:
		if c.loopDepth == 0 {
			return c.newError(v.Position(), "break used outside loop")
		}
		return nil
	case *ast.Return:
		if c.funcDepth == 0 {
			return c.newError(v.Position(), "return used outside function")
		}
		return c.checkExpression(v.Value)
	case *ast.Export:
		if !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "export of undefined identifier: %s", v.Name)
		}
		return nil
	case ast.Expression:
		return c.checkExpression(v)
	default:
		return nil
	}
}

func (c *checker) checkExpression(expr ast.Expression) error {
	switch v := expr.(type) {
	case *ast.Identifier:
		if !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "undefined identifier: %s", v.Name)
		}
		return nil
	case *ast.Literal:
		return nil
	case *ast.FunctionCall:
		if !isBuiltin(v.Name) && !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "undefined identifier: %s", v.Name)
		}
		for _, arg := range v.Args {
			if err := c.checkExpression(arg); err != nil {
				return err
			}
		}
		return nil
	case *ast.CallExpression:
		if id, ok := v.Callee.(*ast.Identifier); ok && isBuiltin(id.Name) {
			for _, arg := range v.Args {
				if err := c.checkExpression(arg); err != nil {
					return err
				}
			}
			return nil
		}
		if err := c.checkExpression(v.Callee); err != nil {
			return err
		}
		for _, arg := range v.Args {
			if err := c.checkExpression(arg); err != nil {
				return err
			}
		}
		return nil
	case *ast.BinaryOperation:
		if err := c.checkExpression(v.LHS); err != nil {
			return err
		}
		return c.checkExpression(v.RHS)
	case *ast.UnaryOperation:
		return c.checkExpression(v.Operand)
	case *ast.ListLiteral:
		for _, elem := range v.Elements {
			if err := c.checkExpression(elem); err != nil {
				return err
			}
		}
		return nil
	case *ast.DictLiteral:
		for _, elem := range v.Elements {
			if err := c.checkExpression(elem.Key); err != nil {
				return err
			}
			if err := c.checkExpression(elem.Value); err != nil {
				return err
			}
		}
		return nil
	case *ast.IndexExpression:
		if err := c.checkExpression(v.Object); err != nil {
			return err
		}
		return c.checkExpression(v.Index)
	case *ast.MemberExpression:
		return c.checkExpression(v.Object)
	default:
		return nil
	}
}

func (c *checker) newError(pos token.Pos, format string, args ...any) error {
	return &Error{
		Diagnostic: Diagnostic{
			Pos:     pos,
			Kind:    "semantic",
			Message: fmt.Sprintf(format, args...),
		},
	}
}

func formatPos(pos token.Pos) string {
	if pos.Line <= 0 {
		return "<unknown>"
	}
	if src, ok := pos.Context.(token.Sourcer); ok {
		return fmt.Sprintf("%s:%d:%d", src.Source(), pos.Line, pos.Column)
	}
	return fmt.Sprintf("%d:%d", pos.Line, pos.Column)
}

func isBuiltin(name string) bool {
	_, ok := extension.BuiltinsModule.Members[name]
	return ok
}
