package semantic

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/token"
)

// protocolArity maps a protocol method's conventional name to the exact number
// of parameters it must declare, including the leading self. A user type
// customizes built-in behavior (operators, comparison, conversion, iteration,
// indexing) by defining a method with one of these names.
var protocolArity = map[string]int{
	"__add": 2, "__sub": 2, "__mul": 2, "__div": 2,
	"__and": 2, "__or": 2, "__cmp": 2, "__getitem": 2,
	"__not": 1, "__str": 1, "__bool": 1, "__iter": 1,
	"__setitem": 3,
}

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
	case *ast.TypeDefine:
		if !isModuleScope {
			return c.newError(v.Position(), "type is only allowed at module scope")
		}
		if !c.currentScope.declare(v.Name) {
			return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Name)
		}

		seenFields := make(map[string]struct{}, len(v.Fields))
		seenDefault := false
		for _, field := range v.Fields {
			if _, ok := seenFields[field.Name]; ok {
				return c.newError(field.Pos, "duplicate type field name: %s", field.Name)
			}
			seenFields[field.Name] = struct{}{}

			if field.HasDefault() {
				seenDefault = true
				if err := c.checkExpression(field.DefaultValue); err != nil {
					return err
				}
				continue
			}
			if seenDefault {
				return c.newError(field.Pos, "required type field cannot appear after default field: %s", field.Name)
			}
		}

		seenMethods := make(map[string]struct{}, len(v.Methods))
		for _, method := range v.Methods {
			if _, ok := seenMethods[method.Name]; ok {
				return c.newError(method.Position(), "duplicate type method name: %s", method.Name)
			}
			seenMethods[method.Name] = struct{}{}

			if len(method.Parameters) == 0 || method.Parameters[0].Name != "self" || method.Parameters[0].VarArgs || method.Parameters[0].KwArgs {
				return c.newError(method.Position(), "type method must declare 'self' as the first parameter")
			}

			// Protocol methods (operators, comparison, conversion, iteration,
			// indexing) have fixed arities and no variadic/keyword parameters.
			if arity, ok := protocolArity[method.Name]; ok {
				if len(method.Parameters) != arity {
					return c.newError(method.Position(), "protocol method '%s' must declare exactly %d parameters including self, got %d", method.Name, arity, len(method.Parameters))
				}
				for _, param := range method.Parameters {
					if param.VarArgs || param.KwArgs {
						return c.newError(param.Pos, "protocol method '%s' cannot use variadic or keyword parameters", method.Name)
					}
				}
			}

			if err := c.withScope(func() error {
				c.funcDepth++
				defer func() { c.funcDepth-- }()

				seen := make(map[string]struct{}, len(method.Parameters))
				for i, param := range method.Parameters {
					if _, ok := seen[param.Name]; ok {
						return c.newError(param.Pos, "duplicate parameter name: %s", param.Name)
					}
					seen[param.Name] = struct{}{}
					if param.KwArgs {
						if i != len(method.Parameters)-1 {
							return c.newError(param.Pos, "kwargs parameter must be the last parameter")
						}
						continue
					}
					if param.VarArgs {
						if i < len(method.Parameters)-1 && !(i == len(method.Parameters)-2 && method.Parameters[len(method.Parameters)-1].KwArgs) {
							return c.newError(param.Pos, "args parameter must be the last parameter or followed by kwargs")
						}
						continue
					}
					if i > 0 {
						prev := method.Parameters[i-1]
						if prev.VarArgs || prev.KwArgs {
							return c.newError(param.Pos, "required parameter cannot appear after args/kwargs parameters")
						}
					}
				}

				for _, field := range v.Fields {
					if !c.currentScope.declare(field.Name) {
						return c.newError(field.Pos, "duplicate declaration in same scope: %s", field.Name)
					}
				}
				for _, param := range method.Parameters {
					if !c.currentScope.declare(param.Name) {
						return c.newError(param.Pos, "duplicate parameter name: %s", param.Name)
					}
				}

				return c.checkStatements(method.Body, false)
			}); err != nil {
				return err
			}
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
	case *ast.SetIndex:
		if err := c.checkExpression(v.Object); err != nil {
			return err
		}
		if err := c.checkExpression(v.Index); err != nil {
			return err
		}
		return c.checkExpression(v.Value)
	case *ast.SetAttr:
		if err := c.checkExpression(v.Object); err != nil {
			return err
		}
		return c.checkExpression(v.Value)
	case *ast.FunctionDefine:
		if !c.currentScope.declare(v.Name) {
			return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Name)
		}
		return c.checkFunction(v.Parameters, v.Body)
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
	case *ast.Continue:
		if c.loopDepth == 0 {
			return c.newError(v.Position(), "continue used outside loop")
		}
		return nil
	case *ast.Return:
		if c.funcDepth == 0 {
			return c.newError(v.Position(), "return used outside function")
		}
		return c.checkExpression(v.Value)
	case *ast.Raise:
		return c.checkExpression(v.Value)
	case *ast.TryCatch:
		if err := c.withScope(func() error {
			return c.checkStatements(v.TryBody, false)
		}); err != nil {
			return err
		}
		return c.withScope(func() error {
			c.currentScope.declare(v.CatchVar)
			return c.checkStatements(v.CatchBody, false)
		})
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

// checkFunction validates a function's parameter list and body in a fresh
// scope. It backs both named function definitions and anonymous function
// literals.
func (c *checker) checkFunction(params []*ast.Parameter, body []ast.Statement) error {
	return c.withScope(func() error {
		c.funcDepth++
		defer func() { c.funcDepth-- }()

		seen := make(map[string]struct{}, len(params))
		for i, param := range params {
			if _, ok := seen[param.Name]; ok {
				return c.newError(param.Pos, "duplicate parameter name: %s", param.Name)
			}
			seen[param.Name] = struct{}{}
			if param.KwArgs {
				if i != len(params)-1 {
					return c.newError(param.Pos, "kwargs parameter must be the last parameter")
				}
				continue
			}
			if param.VarArgs {
				if i < len(params)-1 && !(i == len(params)-2 && params[len(params)-1].KwArgs) {
					return c.newError(param.Pos, "args parameter must be the last parameter or followed by kwargs")
				}
				continue
			}
			if i > 0 {
				prev := params[i-1]
				if prev.VarArgs || prev.KwArgs {
					return c.newError(param.Pos, "required parameter cannot appear after args/kwargs parameters")
				}
			}
		}

		for _, param := range params {
			if !c.currentScope.declare(param.Name) {
				return c.newError(param.Pos, "duplicate parameter name: %s", param.Name)
			}
		}

		return c.checkStatements(body, false)
	})
}

func (c *checker) checkExpression(expr ast.Expression) error {
	switch v := expr.(type) {
	case *ast.FunctionLiteral:
		return c.checkFunction(v.Parameters, v.Body)
	case *ast.Identifier:
		if !isBuiltin(v.Name) && !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "undefined identifier: %s", v.Name)
		}
		return nil
	case *ast.Literal:
		return nil
	case *ast.FunctionCall:
		if !isBuiltin(v.Name) && !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "undefined identifier: %s", v.Name)
		}
		return c.checkCallArguments(v.Args)
	case *ast.CallExpression:
		if id, ok := v.Callee.(*ast.Identifier); ok && isBuiltin(id.Name) {
			return c.checkCallArguments(v.Args)
		}
		if err := c.checkExpression(v.Callee); err != nil {
			return err
		}
		return c.checkCallArguments(v.Args)
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

func (c *checker) checkCallArguments(args []ast.CallArgument) error {
	seenKeyword := false
	seenKeywordNames := make(map[string]struct{})
	for i, arg := range args {
		switch arg.Kind {
		case ast.CallArgumentStarred:
			if seenKeyword {
				return c.newError(arg.Expr.Position(), "positional argument cannot appear after keyword arguments")
			}
			if i != len(args)-1 {
				return c.newError(arg.Expr.Position(), "starred argument must be the last argument")
			}
		case ast.CallArgumentKeyword, ast.CallArgumentKeywordUnpack:
			seenKeyword = true
			if arg.Kind == ast.CallArgumentKeyword {
				if _, ok := seenKeywordNames[arg.Name]; ok {
					return c.newError(arg.NamePos, "duplicate keyword argument: %s", arg.Name)
				}
				seenKeywordNames[arg.Name] = struct{}{}
			}
		case ast.CallArgumentPositional:
			if seenKeyword {
				return c.newError(arg.Expr.Position(), "positional argument cannot appear after keyword arguments")
			}
		}
		if err := c.checkExpression(arg.Expr); err != nil {
			return err
		}
	}
	return nil
}
