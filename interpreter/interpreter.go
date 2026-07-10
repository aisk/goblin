// Package interpreter provides a tree-walking interpreter for Goblin.
//
// Unlike the transpiler, which emits Go source code, the interpreter walks the
// AST directly and evaluates each node using the runtime types in the object
// package. Both share the same object.Object value system, so arithmetic,
// comparison, iteration and built-in functions behave identically.
package interpreter

import (
	"fmt"
	"path/filepath"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// Environment is a lexical scope mapping names to values, with a parent link.
type Environment struct {
	vars   map[string]object.Object
	parent *Environment
}

// NewEnvironment creates a scope whose lookups fall back to parent.
func NewEnvironment(parent *Environment) *Environment {
	return &Environment{vars: make(map[string]object.Object), parent: parent}
}

// Get resolves a name, walking up the scope chain.
func (e *Environment) Get(name string) (object.Object, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// Define binds a name in the current scope (used by `let`).
func (e *Environment) Define(name string, v object.Object) {
	e.vars[name] = v
}

// Assign updates an existing binding in the chain, or defines it in the
// current scope if it does not exist yet.
func (e *Environment) Assign(name string, v object.Object) {
	for s := e; s != nil; s = s.parent {
		if _, ok := s.vars[name]; ok {
			s.vars[name] = v
			return
		}
	}
	e.vars[name] = v
}

// Control-flow signals. They implement error so they can propagate up through
// the eval call stack and be caught by the appropriate construct.
type breakSignal struct{}

func (breakSignal) Error() string { return "break outside loop" }

type continueSignal struct{}

func (continueSignal) Error() string { return "continue outside loop" }

type returnSignal struct{ value object.Object }

func (returnSignal) Error() string { return "return outside function" }

// Run interprets a parsed module. sourcePath is the path of the source file,
// used to resolve relative imports.
func Run(mod *ast.Module, sourcePath string) error {
	global := NewEnvironment(nil)
	reg := object.NewRegistry()

	// Resolve imports and hoist top-level function/type definitions so
	// references (including recursion and forward references) resolve
	// regardless of source order.
	if err := loadInto(mod, global, filepath.Dir(sourcePath), reg); err != nil {
		return err
	}

	err := evalStatements(mod.Body, global)
	if err != nil {
		var pos token.Pos
		if len(mod.Body) > 0 {
			pos = mod.Body[0].Position()
		}
		return object.WithFrame(err, stackFrame(moduleName(sourcePath), "<module>", pos))
	}
	return nil
}

func evalStatements(stmts []ast.Statement, env *Environment) error {
	for _, stmt := range stmts {
		if err := evalStatement(stmt, env); err != nil {
			return err
		}
	}
	return nil
}

func evalStatement(stmt ast.Statement, env *Environment) error {
	switch s := stmt.(type) {
	case *ast.Declare:
		v, err := evalExpr(s.Value, env)
		if err != nil {
			return err
		}
		env.Define(s.Name, v)
		return nil

	case *ast.Assign:
		v, err := evalExpr(s.Value, env)
		if err != nil {
			return err
		}
		env.Assign(s.Target, v)
		return nil

	case *ast.SetIndex:
		obj, err := evalExpr(s.Object, env)
		if err != nil {
			return err
		}
		idx, err := evalExpr(s.Index, env)
		if err != nil {
			return err
		}
		v, err := evalExpr(s.Value, env)
		if err != nil {
			return err
		}
		return object.SetItem(obj, idx, v)

	case *ast.SetAttr:
		obj, err := evalExpr(s.Object, env)
		if err != nil {
			return err
		}
		v, err := evalExpr(s.Value, env)
		if err != nil {
			return err
		}
		return object.SetAttribute(obj, s.Property, v)

	case *ast.IfElse:
		cond, err := evalExpr(s.Condition, env)
		if err != nil {
			return err
		}
		truthy, err := object.Truthy(cond)
		if err != nil {
			return err
		}
		if truthy {
			return evalStatements(s.IfBody, env)
		}
		if s.ElseBody != nil {
			return evalStatements(s.ElseBody, env)
		}
		return nil

	case *ast.While:
		for {
			cond, err := evalExpr(s.Condition, env)
			if err != nil {
				return err
			}
			truthy, err := object.Truthy(cond)
			if err != nil {
				return err
			}
			if !truthy {
				return nil
			}
			if err := evalStatements(s.Body, env); err != nil {
				if _, ok := err.(breakSignal); ok {
					return nil
				}
				if _, ok := err.(continueSignal); ok {
					continue
				}
				return err
			}
		}

	case *ast.For:
		iter, err := evalExpr(s.Iterator, env)
		if err != nil {
			return err
		}
		items, err := iter.Iter()
		if err != nil {
			return err
		}
		for _, item := range items {
			env.Assign(s.Variable, item)
			if err := evalStatements(s.Body, env); err != nil {
				if _, ok := err.(breakSignal); ok {
					return nil
				}
				if _, ok := err.(continueSignal); ok {
					continue
				}
				return err
			}
		}
		return nil

	case *ast.Break:
		return breakSignal{}

	case *ast.Continue:
		return continueSignal{}

	case *ast.Return:
		v, err := evalExpr(s.Value, env)
		if err != nil {
			return err
		}
		return returnSignal{value: v}

	case *ast.Raise:
		v, err := evalExpr(s.Value, env)
		if err != nil {
			return err
		}
		return object.Raise(v)

	case *ast.TryCatch:
		err := evalStatements(s.TryBody, env)
		if err == nil {
			return nil
		}
		// Control-flow signals must pass through untouched; only genuine
		// exceptions (raised Errors and runtime errors) are caught.
		switch err.(type) {
		case breakSignal, continueSignal, returnSignal:
			return err
		}
		catchEnv := NewEnvironment(env)
		catchEnv.Define(s.CatchVar, object.ExcValue(err))
		return evalStatements(s.CatchBody, catchEnv)

	case *ast.FunctionDefine:
		// Define on encounter too (covers non-top-level definitions).
		env.Define(s.Name, makeFunction(s, env))
		return nil

	case *ast.TypeDefine:
		defineType(s, env)
		return nil

	case *ast.Import, *ast.Export:
		// Imports are resolved and bound by loadInto before execution;
		// exports are markers consumed when a module is loaded.
		return nil

	case ast.Expression:
		// Bare expression statement (e.g. a call); evaluate for side effects.
		_, err := evalExpr(s, env)
		return err

	default:
		return fmt.Errorf("interpreter: unsupported statement %T", stmt)
	}
}

func evalExpr(expr ast.Expression, env *Environment) (object.Object, error) {
	switch e := expr.(type) {
	case *ast.Literal:
		return e.Value, nil

	case *ast.Identifier:
		if v, ok := env.Get(e.Name); ok {
			return v, nil
		}
		if b, ok := extension.BuiltinsModule.Members[e.Name]; ok {
			return b, nil
		}
		return nil, object.NewNameError("undefined: %s", e.Name)

	case *ast.ListLiteral:
		elements := make([]object.Object, len(e.Elements))
		for i, el := range e.Elements {
			v, err := evalExpr(el, env)
			if err != nil {
				return nil, err
			}
			elements[i] = v
		}
		return &object.List{Elements: elements}, nil

	case *ast.DictLiteral:
		d := object.NewDict()
		for _, el := range e.Elements {
			k, err := evalExpr(el.Key, env)
			if err != nil {
				return nil, err
			}
			v, err := evalExpr(el.Value, env)
			if err != nil {
				return nil, err
			}
			d.Set(k, v)
		}
		return d, nil

	case *ast.BinaryOperation:
		return evalBinary(e, env)

	case *ast.UnaryOperation:
		return evalUnary(e, env)

	case *ast.IndexExpression:
		obj, err := evalExpr(e.Object, env)
		if err != nil {
			return nil, err
		}
		idx, err := evalExpr(e.Index, env)
		if err != nil {
			return nil, err
		}
		return obj.Index(idx)

	case *ast.MemberExpression:
		obj, err := evalExpr(e.Object, env)
		if err != nil {
			return nil, err
		}
		return obj.GetAttr(e.Property)

	case *ast.FunctionLiteral:
		return makeClosure("<lambda>", e.Position(), e.Parameters, e.Body, env), nil

	case *ast.FunctionCall:
		callee, err := resolveName(e.Name, env)
		if err != nil {
			return nil, err
		}
		args, err := evalArgs(e.Args, env)
		if err != nil {
			return nil, err
		}
		return object.Call(callee, args)

	case *ast.CallExpression:
		callee, err := evalExpr(e.Callee, env)
		if err != nil {
			return nil, err
		}
		args, err := evalArgs(e.Args, env)
		if err != nil {
			return nil, err
		}
		return object.Call(callee, args)

	default:
		return nil, fmt.Errorf("interpreter: unsupported expression %T", expr)
	}
}

func resolveName(name string, env *Environment) (object.Object, error) {
	if v, ok := env.Get(name); ok {
		return v, nil
	}
	if b, ok := extension.BuiltinsModule.Members[name]; ok {
		return b, nil
	}
	return nil, object.NewNameError("undefined: %s", name)
}

func evalBinary(e *ast.BinaryOperation, env *Environment) (object.Object, error) {
	// Short-circuit logical operators: the RHS is only evaluated when the LHS
	// does not already determine the result, so side effects (and errors) in
	// the skipped operand never run.
	if e.Operator == ast.And || e.Operator == ast.Or {
		lhs, err := evalExpr(e.LHS, env)
		if err != nil {
			return nil, err
		}
		lhsTruthy, err := object.Truthy(lhs)
		if err != nil {
			return nil, err
		}
		if e.Operator == ast.And && !lhsTruthy {
			return object.False, nil
		}
		if e.Operator == ast.Or && lhsTruthy {
			return object.True, nil
		}
		rhs, err := evalExpr(e.RHS, env)
		if err != nil {
			return nil, err
		}
		rhsTruthy, err := object.Truthy(rhs)
		if err != nil {
			return nil, err
		}
		return object.Bool(rhsTruthy), nil
	}

	lhs, err := evalExpr(e.LHS, env)
	if err != nil {
		return nil, err
	}
	rhs, err := evalExpr(e.RHS, env)
	if err != nil {
		return nil, err
	}

	switch e.Operator {
	case ast.Add:
		return lhs.Add(rhs)
	case ast.Minus:
		return lhs.Minus(rhs)
	case ast.Multiply:
		return lhs.Multiply(rhs)
	case ast.Divide:
		return lhs.Divide(rhs)
	case ast.Equal, ast.NotEqual, ast.LessThan, ast.GreaterThan, ast.LessOrEqual, ast.GreaterOrEqual:
		c, err := lhs.Compare(rhs)
		if err != nil {
			return nil, err
		}
		return object.Bool(compareResult(e.Operator, c)), nil
	default:
		return nil, fmt.Errorf("interpreter: unknown operator %q", e.Operator)
	}
}

func compareResult(op string, c int) bool {
	switch op {
	case ast.Equal:
		return c == 0
	case ast.NotEqual:
		return c != 0
	case ast.LessThan:
		return c < 0
	case ast.GreaterThan:
		return c > 0
	case ast.LessOrEqual:
		return c <= 0
	case ast.GreaterOrEqual:
		return c >= 0
	}
	return false
}

func evalUnary(e *ast.UnaryOperation, env *Environment) (object.Object, error) {
	operand, err := evalExpr(e.Operand, env)
	if err != nil {
		return nil, err
	}
	switch e.Operator {
	case ast.Not:
		return operand.Not()
	case ast.Add:
		return operand, nil
	case ast.Minus:
		switch v := operand.(type) {
		case object.Integer:
			return object.Integer(-int64(v)), nil
		case object.Float:
			return object.Float(-float64(v)), nil
		default:
			return nil, object.NewTypeError("cannot negate %T", operand)
		}
	default:
		return nil, fmt.Errorf("interpreter: unknown unary operator %q", e.Operator)
	}
}

func evalArgs(args []ast.CallArgument, env *Environment) (object.CallArgs, error) {
	var call object.CallArgs
	for _, arg := range args {
		v, err := evalExpr(arg.Expr, env)
		if err != nil {
			return call, err
		}
		switch arg.Kind {
		case ast.CallArgumentPositional:
			call.Positional = append(call.Positional, v)
		case ast.CallArgumentStarred:
			items, err := v.Iter()
			if err != nil {
				return call, err
			}
			call.Positional = append(call.Positional, items...)
		case ast.CallArgumentKeyword:
			if call.Keyword == nil {
				call.Keyword = object.Kwargs{}
			}
			call.Keyword[arg.Name] = v
		case ast.CallArgumentKeywordUnpack:
			d, ok := v.(*object.Dict)
			if !ok {
				return call, object.NewTypeError("argument after ** must be a dict, got %T", v)
			}
			if call.Keyword == nil {
				call.Keyword = object.Kwargs{}
			}
			for _, entry := range d.Entries {
				call.Keyword[entry.Key.String()] = entry.Value
			}
		default:
			return call, fmt.Errorf("interpreter: unsupported call argument kind %v", arg.Kind)
		}
	}
	return call, nil
}

// makeFunction wraps a Goblin function definition as a callable object.Function,
// capturing the defining environment for closures.
func makeFunction(def *ast.FunctionDefine, env *Environment) *object.Function {
	return makeClosure(def.Name, def.Position(), def.Parameters, def.Body, env)
}

// makeClosure builds a callable object.Function from parameters and a body,
// capturing env for closures. It backs both named definitions and anonymous
// function literals. name is used for repr and BindArguments diagnostics.
func makeClosure(name string, pos token.Pos, params []*ast.Parameter, body []ast.Statement, env *Environment) *object.Function {
	var fixed []string
	var varArgs, kwArgs string
	for _, p := range params {
		switch {
		case p.VarArgs:
			varArgs = p.Name
		case p.KwArgs:
			kwArgs = p.Name
		default:
			fixed = append(fixed, p.Name)
		}
	}
	module := ""
	if src, ok := pos.Context.(token.Sourcer); ok && src != nil {
		module = moduleName(src.Source())
	}
	frame := stackFrame(module, name, pos)

	return &object.Function{
		Name: name,
		Fn: func(args object.CallArgs) (object.Object, error) {
			bound, err := object.BindArguments(name, fixed, varArgs, kwArgs, args)
			if err != nil {
				return nil, object.WithFrame(err, frame)
			}
			local := NewEnvironment(env)
			for n, val := range bound {
				local.Define(n, val)
			}
			err = evalStatements(body, local)
			if rs, ok := err.(returnSignal); ok {
				if rs.value == nil {
					return object.Nil, nil
				}
				return rs.value, nil
			}
			if err != nil {
				return nil, object.WithFrame(err, frame)
			}
			return object.Nil, nil
		},
	}
}
