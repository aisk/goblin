package transpiler

// Local scalar type specialisation.
//
// Every Goblin value is an object.Object, which means an int64 has to be boxed
// on the heap for anything but the smallest values, and every arithmetic
// operation is an interface method call returning (Object, error). For a loop
// whose variables are all numbers that is pure overhead: the types never
// change, so the boxing and the dynamic dispatch buy nothing.
//
// This pass finds local variables whose type can be *proven* constant across
// every assignment in the enclosing function body, and lets the code generator
// declare them as native Go int64/float64/bool. It is deliberately not
// speculative: there are no runtime guards and no bail-out path, so anything
// that cannot be proven simply stays an object.Object and is generated exactly
// as before.
//
// The analysis is flow-insensitive — it collects *all* assignments to a name
// and joins their types. `if c { x = 1 } else { x = "s" }` therefore makes x
// dynamic, which is conservative but correct, and it needs no CFG.

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
)

// staticType is the lattice the analysis works over:
//
//	        dynamic            (top: must be object.Object)
//	      /    |    \
//	tyInt  tyFloat  tyBool
//	      \    |    /
//	        unknown            (bottom: no assignment seen yet)
//
// Two different concrete types join to dynamic rather than widening. Int and
// float in particular must not merge: Goblin's `/` truncates for integers
// (`10 / 2` is Integer 5) but not for floats, so widening an int variable to
// float64 would silently change division semantics.
type staticType uint8

const (
	tyUnknown staticType = iota
	tyInt
	tyFloat
	tyBool
	tyDynamic
)

func (t staticType) native() bool {
	return t == tyInt || t == tyFloat || t == tyBool
}

func numeric(t staticType) bool {
	return t == tyInt || t == tyFloat
}

func join(a, b staticType) staticType {
	switch {
	case a == b:
		return a
	case a == tyUnknown:
		return b
	case b == tyUnknown:
		return a
	default:
		return tyDynamic
	}
}

// goTypeOf maps a native static type to the Go type used to declare it.
func goTypeOf(t staticType) string {
	switch t {
	case tyInt:
		return "int64"
	case tyFloat:
		return "float64"
	case tyBool:
		return "bool"
	}
	return ""
}

// inferLocals analyses one function body (a module's top level counts as one)
// and returns the variables that can be declared with a native Go type.
//
// Only names introduced by `var` in this body are candidates. Parameters are
// excluded: a caller may pass anything, and proving otherwise would need
// interprocedural analysis.
func inferLocals(body []ast.Statement) map[string]staticType {
	types := map[string]staticType{}

	// Candidates: every `var` in this body, at any block depth. Block scoping
	// means two `var x` in sibling blocks are distinct variables, but merging
	// them under one name is safe — the join can only lose precision, and Go's
	// own block scoping keeps the generated declarations correct.
	collectDeclarations(body, types)
	if len(types) == 0 {
		return nil
	}

	// Names a nested function refers to are pinned to dynamic. A nested Go
	// closure gets its own type environment, so it would generate `n.Add(...)`
	// for a captured variable that we had turned into an int64 — which would
	// not compile. Excluding them keeps each function body self-contained.
	poisonCaptured(body, types)

	// Fixed point. `i = i + 1` refers to itself, so one pass is not enough;
	// the lattice is three levels deep, so this converges in a few rounds.
	for {
		changed := false
		walkAssignments(body, func(name string, value ast.Expression) {
			cur, ok := types[name]
			if !ok || cur == tyDynamic {
				return
			}
			next := join(cur, staticTypeOf(value, types))
			if next != cur {
				types[name] = next
				changed = true
			}
		})
		if !changed {
			break
		}
	}

	for name, t := range types {
		if !t.native() {
			delete(types, name)
		}
	}
	if len(types) == 0 {
		return nil
	}
	return types
}

// collectDeclarations seeds the candidate set with every `var` in this body.
// Bindings introduced by anything other than `var` are pinned to dynamic: a
// for-in variable takes its type from the container, and a catch variable is
// always an error object. Both would otherwise collide with a same-named
// specialised variable in the generated Go.
func collectDeclarations(stmts []ast.Statement, types map[string]staticType) {
	forEachStatement(stmts, func(stmt ast.Statement) {
		switch s := stmt.(type) {
		case *ast.Declare:
			if _, seen := types[s.Name]; !seen {
				types[s.Name] = tyUnknown
			}
		case *ast.For:
			types[s.Variable] = tyDynamic
		case *ast.TryCatch:
			types[s.CatchVar] = tyDynamic
		}
	})
}

// poisonCaptured pins every name that appears anywhere inside a nested scope to
// dynamic. A nested function is transpiled with its own type environment, so it
// would emit `n.Add(...)` for a name this body had turned into an int64, and
// that does not compile. Type definitions count too — their method bodies are
// nested scopes as well.
//
// This has to see through arbitrary nesting depth, which is why it uses its own
// traversal rather than the scope-local helpers below.
func poisonCaptured(stmts []ast.Statement, types map[string]staticType) {
	names := map[string]struct{}{}
	collectNestedNames(stmts, names, false)
	for name := range names {
		if _, tracked := types[name]; tracked {
			types[name] = tyDynamic
		}
	}
}

// collectNestedNames walks statements recursively. Once inside a nested scope
// (inner == true) every identifier, assignment target and declaration name it
// meets is recorded.
func collectNestedNames(stmts []ast.Statement, names map[string]struct{}, inner bool) {
	var visitExpr func(ast.Expression)
	visitExpr = func(expr ast.Expression) {
		switch e := expr.(type) {
		case nil:
			return
		case *ast.Identifier:
			if inner {
				names[e.Name] = struct{}{}
			}
		case *ast.FunctionLiteral:
			collectNestedNames(e.Body, names, true)
		case *ast.BinaryOperation:
			visitExpr(e.LHS)
			visitExpr(e.RHS)
		case *ast.UnaryOperation:
			visitExpr(e.Operand)
		case *ast.IndexExpression:
			visitExpr(e.Object)
			visitExpr(e.Index)
		case *ast.MemberExpression:
			visitExpr(e.Object)
		case *ast.ListLiteral:
			for _, item := range e.Elements {
				visitExpr(item)
			}
		case *ast.DictLiteral:
			for _, pair := range e.Elements {
				visitExpr(pair.Key)
				visitExpr(pair.Value)
			}
		case *ast.FunctionCall:
			if inner {
				names[e.Name] = struct{}{}
			}
			for _, arg := range e.Args {
				visitExpr(arg.Expr)
			}
		case *ast.CallExpression:
			visitExpr(e.Callee)
			for _, arg := range e.Args {
				visitExpr(arg.Expr)
			}
		}
	}

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case nil:
			continue
		case *ast.Declare:
			if inner {
				names[s.Name] = struct{}{}
			}
			visitExpr(s.Value)
		case *ast.Assign:
			if inner {
				names[s.Target] = struct{}{}
			}
			visitExpr(s.Value)
		case *ast.SetIndex:
			visitExpr(s.Object)
			visitExpr(s.Index)
			visitExpr(s.Value)
		case *ast.SetAttr:
			visitExpr(s.Object)
			visitExpr(s.Value)
		case *ast.IfElse:
			visitExpr(s.Condition)
			collectNestedNames(s.IfBody, names, inner)
			collectNestedNames(s.ElseBody, names, inner)
		case *ast.While:
			visitExpr(s.Condition)
			collectNestedNames(s.Body, names, inner)
		case *ast.For:
			visitExpr(s.Iterator)
			collectNestedNames(s.Body, names, inner)
		case *ast.TryCatch:
			collectNestedNames(s.TryBody, names, inner)
			collectNestedNames(s.CatchBody, names, inner)
		case *ast.FunctionDefine:
			collectNestedNames(s.Body, names, true)
		case *ast.TypeDefine:
			for _, m := range s.Methods {
				collectNestedNames(m.Body, names, true)
			}
		case *ast.Return:
			visitExpr(s.Value)
		case *ast.Raise:
			visitExpr(s.Value)
		case *ast.Export:
			// An exported name is stored into the module's member map as an
			// object.Object, so it must stay boxed.
			names[s.Name] = struct{}{}
		case *ast.FunctionCall:
			if inner {
				names[s.Name] = struct{}{}
			}
			for _, arg := range s.Args {
				visitExpr(arg.Expr)
			}
		case *ast.CallExpression:
			visitExpr(s.Callee)
			for _, arg := range s.Args {
				visitExpr(arg.Expr)
			}
		case *ast.BinaryOperation:
			visitExpr(s.LHS)
			visitExpr(s.RHS)
		case *ast.UnaryOperation:
			visitExpr(s.Operand)
		case *ast.IndexExpression:
			visitExpr(s)
		case *ast.MemberExpression:
			visitExpr(s)
		}
	}
}

// walkAssignments visits every value bound to a name in this body, both `var`
// declarations and plain assignments.
func walkAssignments(stmts []ast.Statement, fn func(name string, value ast.Expression)) {
	forEachStatement(stmts, func(stmt ast.Statement) {
		switch s := stmt.(type) {
		case *ast.Declare:
			fn(s.Name, s.Value)
		case *ast.Assign:
			fn(s.Target, s.Value)
		}
	})
}

// staticTypeOf is the single source of truth for "can this expression be
// evaluated natively, and as what". emitNative below mirrors it exactly; if the
// two ever disagree, the generated code stops compiling rather than silently
// misbehaving, because a native declaration would receive a boxed value.
func staticTypeOf(expr ast.Expression, types map[string]staticType) staticType {
	switch e := expr.(type) {
	case *ast.Literal:
		switch e.Value.(type) {
		case object.Integer:
			return tyInt
		case object.Float:
			return tyFloat
		case object.Bool:
			return tyBool
		}
		return tyDynamic

	case *ast.Identifier:
		if t, ok := types[e.Name]; ok {
			return t
		}
		return tyDynamic

	case *ast.UnaryOperation:
		operand := staticTypeOf(e.Operand, types)
		switch e.Operator {
		case "-", "+":
			if operand == tyInt || operand == tyFloat {
				return operand
			}
		case "!":
			if operand == tyBool {
				return tyBool
			}
		}
		return tyDynamic

	case *ast.BinaryOperation:
		lhs := staticTypeOf(e.LHS, types)
		rhs := staticTypeOf(e.RHS, types)
		switch e.Operator {
		// Division is included, but unlike the other operators it cannot be a
		// bare Go `/`: Goblin raises a catchable ZeroDivisionError where Go
		// either panics (integers) or yields ±Inf (floats). emitNative
		// therefore guards it, which is why native expressions are allowed to
		// carry preceding statements.
		case "+", "-", "*", "/", "%":
			// Mixed int/float arithmetic widens to float, exactly as
			// object.Integer's methods do. Only int op int stays integral,
			// which is what keeps `/` truncating in the same cases Goblin
			// truncates.
			if lhs == tyInt && rhs == tyInt {
				return tyInt
			}
			if numeric(lhs) && numeric(rhs) {
				return tyFloat
			}
		case "<", "<=", ">", ">=":
			// Ordering is numbers only. Go cannot order bools at all, and
			// Goblin's Bool has no ordering either.
			if numeric(lhs) && numeric(rhs) {
				return tyBool
			}
		case "==", "!=":
			// Integer.Equals(Float) compares as float64, same as Go once the
			// int side is widened.
			if numeric(lhs) && numeric(rhs) {
				return tyBool
			}
			if lhs == tyBool && rhs == tyBool {
				return tyBool
			}
		case ast.And, ast.Or:
			// A division anywhere in the right operand would have its
			// zero-check hoisted in front of the whole expression, which breaks
			// short-circuiting: `b != 0 && a / b > 1` would raise the very
			// error the guard was written to avoid.
			if lhs == tyBool && rhs == tyBool && !containsZeroCheckedArithmetic(e.RHS) {
				return tyBool
			}
		}
		return tyDynamic
	}

	return tyDynamic
}

// containsZeroCheckedArithmetic reports whether a native-eligible expression
// tree performs division or modulo, i.e. whether emitNative would need to hoist
// a guard for it.
func containsZeroCheckedArithmetic(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.UnaryOperation:
		return containsZeroCheckedArithmetic(e.Operand)
	case *ast.BinaryOperation:
		return e.Operator == "/" || e.Operator == "%" ||
			containsZeroCheckedArithmetic(e.LHS) || containsZeroCheckedArithmetic(e.RHS)
	}
	return false
}

// forEachStatement visits every statement in the current function body,
// descending into blocks but never into a nested function literal or
// definition — those are separate scopes with their own analysis.
// poisonCaptured reaches into them explicitly instead.

func forEachStatement(stmts []ast.Statement, fn func(ast.Statement)) {
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		fn(stmt)
		switch s := stmt.(type) {
		case *ast.IfElse:
			forEachStatement(s.IfBody, fn)
			forEachStatement(s.ElseBody, fn)
		case *ast.While:
			forEachStatement(s.Body, fn)
		case *ast.For:
			forEachStatement(s.Body, fn)
		case *ast.TryCatch:
			forEachStatement(s.TryBody, fn)
			forEachStatement(s.CatchBody, fn)
		}
	}
}
