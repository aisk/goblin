package transpiler

import (
	"github.com/aisk/goblin/ast"
	"github.com/dave/jennifer/jen"
)

// Direct-call lowering.
//
// A module-level function is normally an object.Object variable holding an
// &object.Function, and every call goes through object.Call with a freshly
// allocated CallArgs — one slice allocation plus a type switch plus the
// generated binding preamble per call. For the overwhelmingly common case (the
// callee is a module-level `func` that nothing ever rebinds, called with plain
// positional arguments of the right arity) all of that is knowable at
// transpile time.
//
// For such functions the transpiler additionally emits a plain package-level
// Go function
//
//	func _direct_fib_N(n object.Object) (object.Object, error) { <body> }
//
// and the &object.Function wrapper degrades to a thin adapter that forwards to
// it. Call sites that statically match the fast shape call the function
// directly — a static Go call the compiler can inline; everything else — the
// function used as a value, spawn, keyword or unpacked arguments, wrong arity
// — still goes through the wrapper and behaves exactly as before.
type directFn struct {
	goName string // name of the generated package-level Go function
	arity  int    // number of fixed parameters
}

// collectDirectFns decides which of a module's top-level functions qualify
// for direct-call lowering and assigns each one its Go closure name. A name
// qualifies only when this module binds it exactly once — a second definition,
// a `var`, a type, an import, or any assignment anywhere in the module means
// the binding may not hold the function by the time a call runs.
func (ctx *transpileContext) collectDirectFns(stmts []ast.Statement) map[string]directFn {
	rebound := map[string]struct{}{}
	collectAssignTargets(stmts, rebound)

	bindings := map[string]int{}
	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case *ast.FunctionDefine:
			bindings[v.Name]++
		case *ast.Declare:
			bindings[v.Name] += 2
		case *ast.TypeDefine:
			bindings[v.Name] += 2
		case *ast.Import:
			bindings[v.Name] += 2
		}
	}

	fns := map[string]directFn{}
	for _, stmt := range stmts {
		fn, ok := stmt.(*ast.FunctionDefine)
		if !ok || !directCallEligible(fn) || bindings[fn.Name] > 1 {
			continue
		}
		if _, ok := rebound[fn.Name]; ok {
			continue
		}
		fns[fn.Name] = directFn{
			goName: ctx.localName("direct_" + fn.Name),
			arity:  len(fn.Parameters),
		}
	}
	return fns
}

// tryDirectCall lowers a call through a bare name into a direct Go call when
// the name still resolves to a lowerable module-level function at this site
// and the arguments have the fast shape: plain positional, exact arity. The
// returned expression has the same (object.Object, error) shape object.Call
// has, so callers consume it unchanged. ok is false when the call must go
// through the generic path.
func (ctx *transpileContext) tryDirectCall(name string, args []ast.CallArgument, onError errHandler) (pre []jen.Code, call *jen.Statement, ok bool, err error) {
	info, isDirect := ctx.directFns[name]
	if !isDirect || ctx.shadowedBelowModule(name) {
		return nil, nil, false, nil
	}
	if _, imported := ctx.moduleImports[name]; imported {
		return nil, nil, false, nil
	}
	if len(args) != info.arity {
		return nil, nil, false, nil
	}
	for _, arg := range args {
		if arg.Kind != ast.CallArgumentPositional {
			return nil, nil, false, nil
		}
	}

	argExprs := make([]jen.Code, 0, len(args))
	for _, arg := range args {
		argPre, argExpr, err := ctx.transpileExpression(arg.Expr, onError)
		if err != nil {
			return nil, nil, false, err
		}
		pre = append(pre, argPre...)
		argExprs = append(argExprs, argExpr)
	}
	return pre, jen.Id(info.goName).Call(argExprs...), true, nil
}

// shadowedBelowModule reports whether any scope opened after the module's own
// declares name, in which case the name no longer refers to the module-level
// binding at the current site.
func (ctx *transpileContext) shadowedBelowModule(name string) bool {
	for i := ctx.moduleScopeIdx + 1; i < len(ctx.userScopes); i++ {
		if _, ok := ctx.userScopes[i][name]; ok {
			return true
		}
	}
	return false
}

// directCallEligible reports whether a module-level function definition
// qualifies for direct-call lowering. Varargs and kwargs parameters
// disqualify it — the direct signature has one Go parameter per fixed Goblin
// parameter. Defaults are fine: a call site is only lowered when it supplies
// every parameter, and other call shapes reach the wrapper's BindArguments
// path where defaults are handled as usual.
func directCallEligible(fn *ast.FunctionDefine) bool {
	for _, param := range fn.Parameters {
		if param.VarArgs || param.KwArgs {
			return false
		}
	}
	return true
}

// collectAssignTargets records the target name of every assignment anywhere
// in stmts, descending into nested function bodies, type methods, and
// function literals. A module-level function whose name is ever assigned may
// be rebound at runtime, so its call sites cannot be lowered; a plain
// reference (recursion, passing the function as a value) does not count.
func collectAssignTargets(stmts []ast.Statement, out map[string]struct{}) {
	var visitExpr func(ast.Expression)
	visitExpr = func(expr ast.Expression) {
		switch e := expr.(type) {
		case nil:
			return
		case *ast.FunctionLiteral:
			collectAssignTargets(e.Body, out)
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
			visitExpr(s.Value)
		case *ast.Assign:
			out[s.Target] = struct{}{}
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
			collectAssignTargets(s.IfBody, out)
			collectAssignTargets(s.ElseBody, out)
		case *ast.While:
			visitExpr(s.Condition)
			collectAssignTargets(s.Body, out)
		case *ast.For:
			visitExpr(s.Iterator)
			collectAssignTargets(s.Body, out)
		case *ast.TryCatch:
			collectAssignTargets(s.TryBody, out)
			collectAssignTargets(s.CatchBody, out)
		case *ast.FunctionDefine:
			collectAssignTargets(s.Body, out)
		case *ast.TypeDefine:
			for _, m := range s.Methods {
				collectAssignTargets(m.Body, out)
			}
		case *ast.Return:
			visitExpr(s.Value)
		case *ast.Raise:
			visitExpr(s.Value)
		case *ast.FunctionCall:
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
