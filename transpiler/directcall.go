package transpiler

import (
	"fmt"

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

// collectDirectFns decides which of a module's top-level functions and types
// qualify for direct-call lowering and assigns each one its Go name. A name
// qualifies only when this module binds it exactly once — a second definition,
// a `var`, a type, an import, or any assignment anywhere in the module means
// the binding may not hold the function by the time a call runs.
//
// A type's constructor lowers the same way: a construction supplying every
// field positionally becomes a call to a generated `_new_T(fields...)` that
// returns the struct literal, skipping object.Call and the argument slice it
// forces onto the heap. A type additionally has to be the only declaration of
// its name anywhere in the module, so that no parameter or local of the same
// name can make a call site mean something else.
func (g *generator) collectDirectFns(stmts []ast.Statement) (fns, ctors map[string]directFn) {
	rebound := map[string]struct{}{}
	collectAssignTargets(stmts, rebound)
	declared := declaredNames(stmts)

	bindings := map[string]int{}
	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case *ast.FunctionDefine:
			bindings[v.Name]++
		case *ast.Declare:
			bindings[v.Name] += 2
		case *ast.TypeDefine, *ast.TraitDefine:
			bindings[statementName(v)] += 2
		case *ast.Import:
			bindings[v.Name] += 2
		}
	}

	fns = map[string]directFn{}
	ctors = map[string]directFn{}
	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case *ast.FunctionDefine:
			if !directCallEligible(v) || bindings[v.Name] > 1 {
				continue
			}
			if _, ok := rebound[v.Name]; ok {
				continue
			}
			fns[v.Name] = directFn{
				goName: g.localName("direct_" + v.Name),
				arity:  len(v.Parameters),
			}
		case *ast.TypeDefine:
			if bindings[v.Name] > 2 || declared[v.Name] != 1 {
				continue
			}
			if _, ok := rebound[v.Name]; ok {
				continue
			}
			ctors[v.Name] = directFn{
				goName: g.localName("new_" + v.Name),
				arity:  len(v.Fields),
			}
		}
	}
	return fns, ctors
}

// tryDirectCall lowers a call through a bare name into a direct Go call when
// the name still resolves to a lowerable module-level function at this site
// and the arguments have the fast shape: plain positional, exact arity. The
// returned expression has the same (object.Object, error) shape object.Call
// has, so callers consume it unchanged. ok is false when the call must go
// through the generic path.
func (g *generator) tryDirectCall(name string, args []ast.CallArgument, onError errHandler) (pre []jen.Code, call *jen.Statement, ok bool, err error) {
	info, isDirect := g.directFns[name]
	if !isDirect {
		// A type name is registered in moduleVars to route the bare
		// identifier to its constructor variable, so it is looked up before
		// that check rejects it.
		info, isDirect = g.directCtors[name]
		if !isDirect {
			return nil, nil, false, nil
		}
	} else if _, imported := g.moduleVars[name]; imported {
		return nil, nil, false, nil
	}
	if g.shadowedBelowModule(name) {
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

	// A closed function's native parameters take the argument unboxed. The
	// signature inference has verified that every call site agrees, so a
	// mismatch here is a bug in one of the two and is reported as such.
	sig := g.sigs[name]
	argExprs := make([]jen.Code, 0, len(args))
	for i, arg := range args {
		var argPre []jen.Code
		var argExpr *jen.Statement
		var err error
		if sig != nil && sig.params[i].native() {
			if got := g.nativeTypeOf(arg.Expr); got != sig.params[i] {
				return nil, nil, false, fmt.Errorf("%s: internal error: argument %d of %s inferred as %s but the parameter is %s", arg.Expr.Position(), i, name, goTypeOf(got), goTypeOf(sig.params[i]))
			}
			argPre, argExpr, err = g.emitNative(arg.Expr, onError)
		} else {
			argPre, argExpr, err = g.transpileExpression(arg.Expr, onError)
		}
		if err != nil {
			return nil, nil, false, err
		}
		pre = append(pre, argPre...)
		argExprs = append(argExprs, argExpr)
	}
	return pre, jen.Id(info.goName).Call(argExprs...), true, nil
}

// moduleBinding resolves a name to the Go variable holding an imported module
// or a type constructor, unless a scope below the module's own has declared
// the name, in which case the name means that local instead.
func (g *generator) moduleBinding(name string) (string, bool) {
	mapped, ok := g.moduleVars[name]
	if !ok || g.shadowedBelowModule(name) {
		return "", false
	}
	return mapped, true
}

// shadowedBelowModule reports whether any scope opened after the module's own
// declares name, in which case the name no longer refers to the module-level
// binding at the current site.
func (g *generator) shadowedBelowModule(name string) bool {
	for i := g.moduleScopeIdx + 1; i < len(g.userScopes); i++ {
		if _, ok := g.userScopes[i][name]; ok {
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
	walkNodes(stmts, true, func(node ast.Statement) {
		if assign, ok := node.(*ast.Assign); ok {
			out[assign.Target] = struct{}{}
		}
	})
}

// statementName is the name a type or trait declaration binds.
func statementName(stmt ast.Statement) string {
	switch v := stmt.(type) {
	case *ast.TypeDefine:
		return v.Name
	case *ast.TraitDefine:
		return v.Name
	}
	return ""
}
