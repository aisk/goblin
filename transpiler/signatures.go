package transpiler

// Interprocedural signature inference for closed direct functions.
//
// Direct-call lowering (directcall.go) already turns a module-level function
// into a plain Go function, but its parameters and result stay object.Object:
// the local specialisation in typeinfer.go stops at the function boundary
// because a caller may pass anything. For a function that is *closed* — every
// reference to its name in the module is a call in the fast shape, so every
// caller is known — that is no longer true: the parameter types are the join
// of what the call sites pass, and the return type is the join of what the
// body returns. A closed function whose signature comes out native is
// generated as, say, `func _direct_fib(n int64) (int64, error)`, and its call
// sites pass and receive native values.
//
// The inference is a module-wide fixed point in two phases. The first is
// optimistic and runs upward from tyUnknown, so recursion resolves: fib's
// parameter is unknown until `fib(n - 1)` is seen with n unknown, which stays
// unknown rather than collapsing to dynamic, and the outer `fib(32)` then
// settles it to int. The second phase re-checks every signature against the
// type environments the code generator will actually use — where anything
// still unknown has become dynamic — and downgrades whatever no longer
// agrees, so a native parameter is never handed a boxed value.

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/token"
)

// fnSig is the inferred signature of a closed direct function. A tyDynamic
// entry is an ordinary object.Object parameter or result.
type fnSig struct {
	params []staticType
	ret    staticType
}

func (s *fnSig) equal(o *fnSig) bool {
	if s.ret != o.ret || len(s.params) != len(o.params) {
		return false
	}
	for i := range s.params {
		if s.params[i] != o.params[i] {
			return false
		}
	}
	return true
}

// bodyRef is one function body of a module as the inference sees it: the
// module's own top level, a function definition at any depth, a function
// literal, or a type method. closed names the closed function the body
// belongs to, when it does.
type bodyRef struct {
	stmts  []ast.Statement
	params []*ast.Parameter
	closed string
}

// inferSignatures runs the two-phase inference over a module and returns the
// signatures of its closed direct functions, keyed by name. Every returned
// function is closed; a signature with only dynamic entries is still useful
// to the generator, which then knows it may drop the function's generic
// wrapper.
func inferSignatures(stmts []ast.Statement, direct map[string]directFn, rangeNative bool) map[string]*fnSig {
	closed := closedDirectFns(stmts, direct)
	if len(closed) == 0 {
		return nil
	}
	bodies := collectBodies(stmts, closed)

	fresh := func() map[string]*fnSig {
		sigs := make(map[string]*fnSig, len(closed))
		for name, fn := range closed {
			sigs[name] = &fnSig{params: make([]staticType, len(fn.Parameters)), ret: tyUnknown}
		}
		return sigs
	}

	// Phase one: monotone ascent from bottom. Each round recomputes every
	// body's environment from the current signatures and rebuilds the
	// signatures from the joins those environments give; the lattice is
	// finite, so this terminates.
	sigs := fresh()
	for {
		next := fresh()
		for _, body := range bodies {
			env := inferTypes(body.stmts, rangeNative, paramSeed(body, sigs), sigs)
			forEachCallSite(body.stmts, sigs, func(name string, args []ast.CallArgument) {
				sig := next[name]
				for i, arg := range args {
					sig.params[i] = join(sig.params[i], staticTypeOf(arg.Expr, env, sigs))
				}
			})
			if body.closed != "" {
				sig := next[body.closed]
				for i, param := range body.params {
					sig.params[i] = join(sig.params[i], lookupType(env, param.Name))
				}
				sig.ret = join(sig.ret, returnTypeOf(body.stmts, env, sigs))
			}
		}
		same := true
		for name := range sigs {
			if !sigs[name].equal(next[name]) {
				same = false
				break
			}
		}
		sigs = next
		if same {
			break
		}
	}

	// Phase two: descent against the generator's view. Unresolved entries are
	// dynamic from here on, and any native entry that a call site, a
	// parameter's own body environment or the return join no longer matches
	// is downgraded. Downgrades can only cascade downward, so this terminates
	// too.
	for _, sig := range sigs {
		for i, t := range sig.params {
			if t == tyUnknown {
				sig.params[i] = tyDynamic
			}
		}
		if sig.ret == tyUnknown {
			sig.ret = tyDynamic
		}
	}
	for changed := true; changed; {
		changed = false
		for _, body := range bodies {
			env := inferLocals(body.stmts, rangeNative, paramSeed(body, sigs), sigs)
			forEachCallSite(body.stmts, sigs, func(name string, args []ast.CallArgument) {
				sig := sigs[name]
				for i, arg := range args {
					if sig.params[i].native() && staticTypeOf(arg.Expr, env, sigs) != sig.params[i] {
						sig.params[i] = tyDynamic
						changed = true
					}
				}
			})
			if body.closed != "" {
				sig := sigs[body.closed]
				for i, param := range body.params {
					if sig.params[i].native() && lookupType(env, param.Name) != sig.params[i] {
						sig.params[i] = tyDynamic
						changed = true
					}
				}
				if sig.ret.native() && returnTypeOf(body.stmts, env, sigs) != sig.ret {
					sig.ret = tyDynamic
					changed = true
				}
			}
		}
	}
	return sigs
}

// paramSeed returns the seed environment for a body: the current signature's
// parameter types when the body belongs to a closed function, nothing
// otherwise.
func paramSeed(body bodyRef, sigs map[string]*fnSig) map[string]staticType {
	if body.closed == "" {
		return nil
	}
	sig := sigs[body.closed]
	seed := make(map[string]staticType, len(body.params))
	for i, param := range body.params {
		seed[param.Name] = sig.params[i]
	}
	return seed
}

// lookupType reads a name from an environment, absent meaning dynamic: a
// name that is not a candidate is an ordinary boxed value.
func lookupType(env map[string]staticType, name string) staticType {
	if t, ok := env[name]; ok {
		return t
	}
	return tyDynamic
}

// returnTypeOf joins the types of every `return` in a body. The parser
// appends an implicit `return nil` to every function; it only counts when it
// is reachable, i.e. when the statements before it do not already return on
// every path — a body that always returns natively must not be pulled to
// dynamic by a return nobody can reach.
func returnTypeOf(stmts []ast.Statement, env map[string]staticType, sigs map[string]*fnSig) staticType {
	explicit := stmts
	if n := len(stmts); n > 0 && isImplicitReturn(stmts[n-1]) {
		explicit = stmts[:n-1]
		if !alwaysReturns(explicit) {
			return tyDynamic
		}
	}
	t := tyUnknown
	forEachStatement(explicit, func(stmt ast.Statement) {
		if ret, ok := stmt.(*ast.Return); ok {
			t = join(t, staticTypeOf(ret.Value, env, sigs))
		}
	})
	return t
}

// isImplicitReturn recognizes the `return nil` the parser appends to function
// bodies: it is the only return without a source position.
func isImplicitReturn(stmt ast.Statement) bool {
	ret, ok := stmt.(*ast.Return)
	return ok && ret.Position() == (token.Pos{})
}

// alwaysReturns reports whether every path through stmts ends in a return or
// a raise. It follows the same shapes Go accepts as terminating statements,
// so a body it approves compiles without a trailing return of its own.
func alwaysReturns(stmts []ast.Statement) bool {
	if len(stmts) == 0 {
		return false
	}
	switch last := stmts[len(stmts)-1].(type) {
	case *ast.Return, *ast.Raise:
		return true
	case *ast.IfElse:
		return alwaysReturns(last.IfBody) && alwaysReturns(last.ElseBody)
	}
	return false
}

// closedDirectFns picks, out of a module's direct-call candidates, the
// functions whose every reference is a fast-shape call: the name is declared
// exactly once in the whole module (so no scope can shadow it and every call
// through it is this function), it is never used as a value, exported, or
// mentioned in a parameter default, and it has no defaults of its own (the
// generic wrapper that would evaluate them is what closedness removes).
func closedDirectFns(stmts []ast.Statement, direct map[string]directFn) map[string]*ast.FunctionDefine {
	declared := declaredNames(stmts)
	excluded := map[string]struct{}{}

	fastShape := func(name string, args []ast.CallArgument) bool {
		info, ok := direct[name]
		if !ok || len(args) != info.arity {
			return false
		}
		for _, arg := range args {
			if arg.Kind != ast.CallArgumentPositional {
				return false
			}
		}
		return true
	}
	excludeDefaults := func(params []*ast.Parameter) {
		for _, param := range params {
			if param.Default != nil {
				walkExpr(param.Default, func(node ast.Statement) {
					switch n := node.(type) {
					case *ast.Identifier:
						excluded[n.Name] = struct{}{}
					case *ast.FunctionCall, *ast.CallExpression:
						if name, _, ok := bareCall(n.(ast.Expression)); ok {
							excluded[name] = struct{}{}
						}
					}
				})
			}
		}
	}

	walkNodes(stmts, true, func(node ast.Statement) {
		switch n := node.(type) {
		case *ast.TypeDefine:
			for _, method := range n.Methods {
				excludeDefaults(method.Parameters)
			}
		case *ast.FunctionDefine:
			excludeDefaults(n.Parameters)
		case *ast.FunctionLiteral:
			excludeDefaults(n.Parameters)
		case *ast.Export:
			excluded[n.Name] = struct{}{}
		case *ast.Identifier:
			excluded[n.Name] = struct{}{}
		case *ast.FunctionCall, *ast.CallExpression:
			if name, args, ok := bareCall(n.(ast.Expression)); ok && !fastShape(name, args) {
				excluded[name] = struct{}{}
			}
		}
	})

	closed := map[string]*ast.FunctionDefine{}
	for _, stmt := range stmts {
		fn, ok := stmt.(*ast.FunctionDefine)
		if !ok {
			continue
		}
		if _, isDirect := direct[fn.Name]; !isDirect || declared[fn.Name] != 1 {
			continue
		}
		if _, isExcluded := excluded[fn.Name]; isExcluded {
			continue
		}
		hasDefault := false
		for _, param := range fn.Parameters {
			hasDefault = hasDefault || param.Default != nil
		}
		if hasDefault {
			continue
		}
		closed[fn.Name] = fn
	}
	return closed
}

// declaredNames counts how many times each name is bound anywhere in a
// module: by a `var`, a for-loop or catch variable, a function, type or
// import, or a parameter of any function, literal or method.
func declaredNames(stmts []ast.Statement) map[string]int {
	declared := map[string]int{}
	params := func(ps []*ast.Parameter) {
		for _, p := range ps {
			declared[p.Name]++
		}
	}
	walkNodes(stmts, true, func(node ast.Statement) {
		switch n := node.(type) {
		case *ast.Declare:
			declared[n.Name]++
		case *ast.For:
			declared[n.Variable]++
		case *ast.TryCatch:
			declared[n.CatchVar]++
		case *ast.Import:
			declared[n.Name]++
		case *ast.TypeDefine:
			declared[n.Name]++
			for _, method := range n.Methods {
				params(method.Parameters)
			}
		case *ast.FunctionDefine:
			declared[n.Name]++
			params(n.Parameters)
		case *ast.FunctionLiteral:
			params(n.Parameters)
		}
	})
	return declared
}

// bodyRebinds reports whether a body declares or assigns name anywhere in its
// own scope, nested bodies excluded.
func bodyRebinds(stmts []ast.Statement, name string) bool {
	found := false
	walkNodes(stmts, false, func(node ast.Statement) {
		switch n := node.(type) {
		case *ast.Declare:
			found = found || n.Name == name
		case *ast.Assign:
			found = found || n.Target == name
		case *ast.For:
			found = found || n.Variable == name
		case *ast.TryCatch:
			found = found || n.CatchVar == name
		}
	})
	return found
}

// collectBodies lists every function body in the module, the top level
// first, in the order the walker meets them.
func collectBodies(stmts []ast.Statement, closed map[string]*ast.FunctionDefine) []bodyRef {
	bodies := []bodyRef{{stmts: stmts}}
	walkNodes(stmts, true, func(node ast.Statement) {
		switch n := node.(type) {
		case *ast.FunctionDefine:
			ref := bodyRef{stmts: n.Body, params: n.Parameters}
			if closed[n.Name] == n {
				ref.closed = n.Name
			}
			bodies = append(bodies, ref)
		case *ast.FunctionLiteral:
			bodies = append(bodies, bodyRef{stmts: n.Body, params: n.Parameters})
		case *ast.TypeDefine:
			for _, method := range n.Methods {
				bodies = append(bodies, bodyRef{stmts: method.Body, params: method.Parameters})
			}
		}
	})
	return bodies
}

// forEachCallSite visits every call to a closed function within one body,
// nested bodies excluded — those are bodies of their own, with their own
// environments.
func forEachCallSite(stmts []ast.Statement, sigs map[string]*fnSig, fn func(name string, args []ast.CallArgument)) {
	walkNodes(stmts, false, func(node ast.Statement) {
		switch node.(type) {
		case *ast.FunctionCall, *ast.CallExpression:
			if name, args, ok := bareCall(node.(ast.Expression)); ok {
				if _, closed := sigs[name]; closed {
					fn(name, args)
				}
			}
		}
	})
}

// walkNodes visits every statement and expression node under stmts in
// pre-order. With nested set it descends into function definitions,
// function literals and type methods; otherwise it visits those nodes but
// stays out of their bodies. Parameter defaults are never entered — they
// belong to the enclosing scope and callers handle them explicitly. A call's
// callee identifier is not visited on its own: the call node stands for it,
// which is how a visitor tells a call from a use of the name as a value.
func walkNodes(stmts []ast.Statement, nested bool, visit func(node ast.Statement)) {
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		if expr, ok := stmt.(ast.Expression); ok {
			walkExprWith(expr, nested, visit)
			continue
		}
		visit(stmt)
		switch s := stmt.(type) {
		case *ast.Declare:
			walkExprWith(s.Value, nested, visit)
		case *ast.Assign:
			walkExprWith(s.Value, nested, visit)
		case *ast.SetIndex:
			walkExprWith(s.Object, nested, visit)
			walkExprWith(s.Index, nested, visit)
			walkExprWith(s.Value, nested, visit)
		case *ast.SetAttr:
			walkExprWith(s.Object, nested, visit)
			walkExprWith(s.Value, nested, visit)
		case *ast.IfElse:
			walkExprWith(s.Condition, nested, visit)
			walkNodes(s.IfBody, nested, visit)
			walkNodes(s.ElseBody, nested, visit)
		case *ast.While:
			walkExprWith(s.Condition, nested, visit)
			walkNodes(s.Body, nested, visit)
		case *ast.For:
			walkExprWith(s.Iterator, nested, visit)
			walkNodes(s.Body, nested, visit)
		case *ast.TryCatch:
			walkNodes(s.TryBody, nested, visit)
			walkNodes(s.CatchBody, nested, visit)
		case *ast.FunctionDefine:
			if nested {
				walkNodes(s.Body, nested, visit)
			}
		case *ast.TypeDefine:
			if nested {
				for _, method := range s.Methods {
					walkNodes(method.Body, nested, visit)
				}
			}
		case *ast.Return:
			walkExprWith(s.Value, nested, visit)
		case *ast.Raise:
			walkExprWith(s.Value, nested, visit)
		}
	}
}

// walkExpr visits every node of an expression tree, function literal bodies
// included.
func walkExpr(expr ast.Expression, visit func(node ast.Statement)) {
	walkExprWith(expr, true, visit)
}

func walkExprWith(expr ast.Expression, nested bool, visit func(node ast.Statement)) {
	if expr == nil {
		return
	}
	visit(expr)
	switch e := expr.(type) {
	case *ast.FunctionLiteral:
		if nested {
			walkNodes(e.Body, nested, visit)
		}
	case *ast.BinaryOperation:
		walkExprWith(e.LHS, nested, visit)
		walkExprWith(e.RHS, nested, visit)
	case *ast.UnaryOperation:
		walkExprWith(e.Operand, nested, visit)
	case *ast.IndexExpression:
		walkExprWith(e.Object, nested, visit)
		walkExprWith(e.Index, nested, visit)
	case *ast.MemberExpression:
		walkExprWith(e.Object, nested, visit)
	case *ast.ListLiteral:
		for _, item := range e.Elements {
			walkExprWith(item, nested, visit)
		}
	case *ast.DictLiteral:
		for _, pair := range e.Elements {
			walkExprWith(pair.Key, nested, visit)
			walkExprWith(pair.Value, nested, visit)
		}
	case *ast.FunctionCall:
		for _, arg := range e.Args {
			walkExprWith(arg.Expr, nested, visit)
		}
	case *ast.CallExpression:
		if _, isIdent := e.Callee.(*ast.Identifier); !isIdent {
			walkExprWith(e.Callee, nested, visit)
		}
		for _, arg := range e.Args {
			walkExprWith(arg.Expr, nested, visit)
		}
	}
}
