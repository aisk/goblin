package transpiler

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

func (g *generator) transpileDeclare(decl *ast.Declare, onError errHandler) ([]jen.Code, error) {
	if t := g.typeOf(decl.Name); t.native() {
		pre, value, err := g.emitNative(decl.Value, onError)
		if err != nil {
			return nil, err
		}
		g.declareUserName(decl.Name)
		declStmt := jen.Var().Id(decl.Name).Id(goTypeOf(t)).Op("=").Add(value)
		declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
		return append(pre, declStmt), nil
	}

	preStmts, value, err := g.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	g.declareUserName(decl.Name)
	declStmt := jen.Var().Id(decl.Name).Qual(pathObject, "Object").Op("=").Add(value)
	declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
	return append(preStmts, declStmt), nil
}

func (g *generator) transpileAssign(decl *ast.Assign, onError errHandler) ([]jen.Code, error) {
	if g.typeOf(decl.Target).native() {
		pre, value, err := g.emitNative(decl.Value, onError)
		if err != nil {
			return nil, err
		}
		assignStmt := jen.Id(decl.Target).Op("=").Add(value)
		assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
		return append(pre, assignStmt), nil
	}

	preStmts, value, err := g.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	assignStmt := jen.Id(decl.Target).Op("=").Add(value)
	assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
	return append(preStmts, assignStmt), nil
}

// transpileDestructure lowers `var a, b = value` and `a, b = value`: the
// value is unpacked into a Go slice once, and each store then transpiles as
// the ordinary statement it is, reading its element through *ast.Unpacked.
// store transpiles one of them.
func (g *generator) transpileDestructure(d *ast.Destructure, onError errHandler, store func(ast.Statement) ([]jen.Code, error)) ([]jen.Code, error) {
	pre, value, err := g.transpileExpression(d.Value, onError)
	if err != nil {
		return nil, err
	}
	itemsVar := g.localName("items")
	errVar := g.localName("err")
	codes := append(pre,
		jen.List(jen.Id(itemsVar), jen.Id(errVar)).Op(":=").Qual(pathObject, "Unpack").Call(value, jen.Lit(len(d.Stores))),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	saved := g.unpacked
	g.unpacked = itemsVar
	defer func() { g.unpacked = saved }()
	for _, s := range d.Stores {
		stored, err := store(s)
		if err != nil {
			return nil, err
		}
		codes = append(codes, stored...)
	}
	return codes, nil
}

func (g *generator) transpileSetIndex(s *ast.SetIndex, onError errHandler) ([]jen.Code, error) {
	objPre, obj, err := g.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	idxPre, idx, native, err := g.transpileIndexOperand(s.Index, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := g.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	setter := "SetIndex"
	if native {
		setter = "SetIndexInt"
	}
	errVar := g.localName("err")
	stmts, codes := g.sequence([]operand{
		{expr: s.Object, pre: objPre, code: obj, native: tyDynamic},
		{expr: s.Index, pre: idxPre, code: idx, native: nativeIf(native, tyInt)},
		{expr: s.Value, pre: valPre, code: val, native: tyDynamic},
	})
	obj, idx, val = codes[0], codes[1], codes[2]
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, setter).Call(obj, idx, val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (g *generator) transpileSetAttr(s *ast.SetAttr, onError errHandler) ([]jen.Code, error) {
	// `self.field = v` inside a method is a plain Go field store.
	if info, ok := g.selfMember(s.Object, s.Property); ok && info.fields[s.Property] {
		valPre, val, err := g.transpileExpression(s.Value, onError)
		if err != nil {
			return nil, err
		}
		return append(valPre, jen.Id(info.receiver).Dot(fieldGoName(s.Property)).Op("=").Add(val)), nil
	}

	objPre, obj, err := g.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := g.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	errVar := g.localName("err")
	stmts, codes := g.sequence([]operand{
		{expr: s.Object, pre: objPre, code: obj, native: tyDynamic},
		{expr: s.Value, pre: valPre, code: val, native: tyDynamic},
	})
	obj, val = codes[0], codes[1]
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "SetAttr").Call(obj, jen.Lit(s.Property), val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (g *generator) transpileIfElse(ifelse *ast.IfElse, onError errHandler) ([]jen.Code, error) {
	pre, cond, err := g.transpileCondition(ifelse.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := g.transpileStatements(ifelse.IfBody, onError)
	if err != nil {
		return nil, err
	}
	elseBody, err := g.transpileStatements(ifelse.ElseBody, onError)
	if err != nil {
		return nil, err
	}
	return append(pre, jen.If(cond).Block(body...).Else().Block(elseBody...)), nil
}

func (g *generator) transpileWhile(while_ *ast.While, onError errHandler) ([]jen.Code, error) {
	condPre, cond, err := g.transpileCondition(while_.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := g.transpileStatements(while_.Body, onError)
	if err != nil {
		return nil, err
	}
	// A condition without preceding statements is a plain Go for-condition,
	// which is what lets a hot numeric loop compile down to an ordinary Go
	// `for`. Statements the condition needs (a zero check, a call, a boxed
	// comparison) have to run every iteration, so the loop then becomes
	// `for { statements; if !cond { break }; body }`.
	if len(condPre) == 0 {
		return []jen.Code{jen.For(cond).Block(body...)}, nil
	}
	loopBody := append([]jen.Code{}, condPre...)
	loopBody = append(loopBody, jen.If(jen.Op("!").Parens(cond)).Block(jen.Break()))
	loopBody = append(loopBody, body...)
	return []jen.Code{jen.For().Block(loopBody...)}, nil
}

func (g *generator) transpileBreak(break_ *ast.Break) ([]jen.Code, error) {
	return []jen.Code{jen.Break()}, nil
}

func (g *generator) transpileContinue(continue_ *ast.Continue) ([]jen.Code, error) {
	return []jen.Code{jen.Continue()}, nil
}

func (g *generator) transpileFor(for_ *ast.For, onError errHandler) ([]jen.Code, error) {
	// `for x in range(a, b)` runs as a native counting loop when `range` is
	// known to be the builtin, instead of materialising a list of boxed
	// integers only to iterate it once. The condition must stay in lockstep
	// with collectDeclarations, which types the loop variable accordingly.
	if g.rangeNative {
		if startExpr, endExpr, ok := rangeCallParts(for_.Iterator); ok {
			return g.transpileRangeFor(for_, startExpr, endExpr, onError)
		}
	}

	iterPreStmts, iterator, err := g.transpileExpression(for_.Iterator, onError)
	if err != nil {
		return nil, err
	}

	// The loop pulls one element per pass of the body from an
	// object.Iteration, so a lazy iterable is read only as far as the loop
	// gets, and an error the iterator raises is reported at the for
	// statement. It is a plain Go loop rather than a range over an
	// iter.Seq so the body keeps updating the function's locals in place.
	iterVar := g.localName("iter")
	itemVar := g.localName("item")
	okVar := g.localName("ok")
	errVar := g.localName("err")

	// The pass is closed exactly once however the loop ends: the end of the
	// pass and a break both land after the Go loop, an error leaving the
	// body or raised by the iterator goes through loopOnError, and a return
	// closes every enclosing pass itself (see transpileReturn). A try inside
	// the body has its own handler, so an error it catches leaves the pass
	// open, as it should.
	closePass := jen.Id(iterVar).Dot("Close").Call()
	loopOnError := func(errVar string) jen.Code {
		return jen.Block(closePass, onError(errVar))
	}

	popScope := g.pushUserScope()
	g.declareUserName(for_.Variable)
	g.body.iters = append(g.body.iters, iterVar)
	body, err := g.transpileStatements(for_.Body, loopOnError)
	g.body.iters = g.body.iters[:len(g.body.iters)-1]
	popScope()
	if err != nil {
		return nil, err
	}

	// Fast is the inlined slice path (a List); Pull covers lazy sources and
	// the end of the pass.
	forLoopBody := []jen.Code{
		jen.List(jen.Id(itemVar), jen.Id(okVar)).Op(":=").Id(iterVar).Dot("Fast").Call(),
		jen.If(jen.Op("!").Id(okVar)).Block(
			jen.Var().Id(errVar).Error(),
			jen.List(jen.Id(itemVar), jen.Id(okVar), jen.Id(errVar)).Op("=").Id(iterVar).Dot("Pull").Call(),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(loopOnError(errVar)),
			jen.If(jen.Op("!").Id(okVar)).Block(jen.Break()),
		),
		jen.Id(for_.Variable).Op(":=").Id(itemVar),
		jen.Id("_").Op("=").Id(for_.Variable),
	}
	forLoopBody = append(forLoopBody, body...)

	result := append(iterPreStmts,
		jen.List(jen.Id(iterVar), jen.Id(errVar)).Op(":=").Qual(pathObject, "Iterate").Call(iterator),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.For().Block(forLoopBody...),
		closePass,
	)

	return []jen.Code{jen.Block(result...)}, nil
}

// transpileRangeFor lowers `for x in range(a, b)` onto a native Go counting
// loop. The bounds are evaluated once, before the loop, exactly as range()'s
// arguments would be; when they are not provably Integer the shared
// extension.RangeBounds helper validates them, so a bad bound reports the very
// error range() itself raises. The loop variable is a fresh binding each
// iteration — assigning it in the body never affects the iteration, matching
// list iteration semantics — and is declared as a native int64 when the
// inference pass proved every assignment to it in this body stays Integer.
func (g *generator) transpileRangeFor(for_ *ast.For, startExpr, endExpr ast.Expression, onError errHandler) ([]jen.Code, error) {
	counterVar := g.localName("range_i")
	endVar := g.localName("range_end")

	var pre []jen.Code
	var startCode *jen.Statement
	// A nil startExpr is the one-argument form, which starts at zero.
	if (startExpr == nil || g.nativeTypeOf(startExpr) == tyInt) && g.nativeTypeOf(endExpr) == tyInt {
		var startPre []jen.Code
		start := jen.Int64().Call(jen.Lit(0))
		if startExpr != nil {
			var err error
			if startPre, start, err = g.emitNative(startExpr, onError); err != nil {
				return nil, err
			}
		}
		endPre, end, err := g.emitNative(endExpr, onError)
		if err != nil {
			return nil, err
		}
		pre = append(append(pre, startPre...), endPre...)
		pre = append(pre, jen.Id(endVar).Op(":=").Add(end))
		startCode = start
	} else {
		var startPre []jen.Code
		start := jen.Qual(pathObject, "Integer").Call(jen.Lit(0))
		if startExpr != nil {
			var err error
			if startPre, start, err = g.transpileExpression(startExpr, onError); err != nil {
				return nil, err
			}
		}
		endPre, end, err := g.transpileExpression(endExpr, onError)
		if err != nil {
			return nil, err
		}
		startVar := g.localName("range_start")
		errVar := g.localName("err")
		pre = append(append(append(pre, startPre...), endPre...),
			jen.List(jen.Id(startVar), jen.Id(endVar), jen.Id(errVar)).Op(":=").Qual(pathExtension, "RangeBounds").Call(start, end),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		startCode = jen.Id(startVar)
	}

	popScope := g.pushUserScope()
	g.declareUserName(for_.Variable)
	body, err := g.transpileStatements(for_.Body, onError)
	popScope()
	if err != nil {
		return nil, err
	}

	loopBody := make([]jen.Code, 0, len(body)+2)
	if g.typeOf(for_.Variable) == tyInt {
		loopBody = append(loopBody, jen.Id(for_.Variable).Op(":=").Id(counterVar))
	} else {
		loopBody = append(loopBody,
			jen.Var().Id(for_.Variable).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Integer").Call(jen.Id(counterVar)),
		)
	}
	loopBody = append(loopBody, jen.Id("_").Op("=").Id(for_.Variable))
	loopBody = append(loopBody, body...)

	result := append(pre, jen.For(
		jen.Id(counterVar).Op(":=").Add(startCode),
		jen.Id(counterVar).Op("<").Id(endVar),
		jen.Id(counterVar).Op("++"),
	).Block(loopBody...))

	return []jen.Code{jen.Block(result...)}, nil
}

func (g *generator) transpileReturn(return_ *ast.Return, onError errHandler) ([]jen.Code, error) {
	if g.body.ret.native() {
		// The signature inference only makes a return type native when every
		// reachable return agrees with it; the parser's implicit `return nil`
		// is then unreachable and only has to keep Go's terminating-statement
		// rule satisfied.
		if g.nativeTypeOf(return_.Value) == g.body.ret {
			pre, value, err := g.emitNative(return_.Value, onError)
			if err != nil {
				return nil, err
			}
			return append(pre, g.closingReturn(value)...), nil
		}
		if isImplicitReturn(return_) {
			return []jen.Code{jen.Return(jen.List(zeroOf(g.body.ret), jen.Nil()))}, nil
		}
		return nil, fmt.Errorf("%s: internal error: return does not match the inferred native return type", return_.Position())
	}

	preStmts, r, err := g.transpileExpression(return_.Value, onError)
	if err != nil {
		return nil, err
	}
	return append(preStmts, g.closingReturn(r)...), nil
}

// closingReturn returns value from the body. Inside for loops the value is
// evaluated first, then every enclosing pass is closed, innermost first, and
// only then does the body return (and its deferred calls run).
func (g *generator) closingReturn(value *jen.Statement) []jen.Code {
	iters := g.body.iters
	if len(iters) == 0 {
		return []jen.Code{jen.Return(jen.List(value, jen.Nil()))}
	}
	retVar := g.localName("ret")
	code := []jen.Code{jen.Id(retVar).Op(":=").Add(value)}
	for i := len(iters) - 1; i >= 0; i-- {
		code = append(code, jen.Id(iters[i]).Dot("Close").Call())
	}
	return append(code, jen.Return(jen.List(jen.Id(retVar), jen.Nil())))
}

// transpileRaise turns `raise expr` into an error (the raised Error value, or a
// type error if expr is not an Error) that is then routed through the active
// error handler (a top-level `return`, or a jump to the enclosing `try`'s catch
// label).
func (g *generator) transpileRaise(raise *ast.Raise, onError errHandler) ([]jen.Code, error) {
	preStmts, val, err := g.transpileExpression(raise.Value, onError)
	if err != nil {
		return nil, err
	}
	errVar := g.localName("err")
	code := append(preStmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "Raise").Call(val),
		onError(errVar),
	)
	return code, nil
}

// transpileDefer lowers `defer f(args)`. The callee and the arguments are
// evaluated here into Go locals, and the call is pushed, as a closure over
// them, on the body's object.Defers stack; wrapDeferred makes the body run
// that stack when it ends. The callee is evaluated as a value, never lowered
// to a direct call: the call happens later, through object.Call. An error it
// raises is reported at the defer statement, in the enclosing function's
// frame, whatever `try` block the statement sat in.
func (g *generator) transpileDefer(d *ast.Defer, onError errHandler) ([]jen.Code, error) {
	calleePre, callee, err := g.transpileExpression(d.Call.Callee, onError)
	if err != nil {
		return nil, err
	}
	argsPre, args, err := g.transpileCallArguments(d.Call.Args, onError)
	if err != nil {
		return nil, err
	}
	if g.argsMayHaveEffects(d.Call.Args) && !g.stableCode(d.Call.Callee) {
		calleePre, callee = g.captureValue(calleePre, callee, tyDynamic)
	}
	if g.body.defers == "" {
		g.body.defers = g.localName("defers")
	}
	fnVar := g.localName("deferred")
	argsVar := g.localName("args")
	errVar := g.localName("err")
	frame := frameCode(g.body.module, g.body.function, d.Position())
	codes := append(calleePre, argsPre...)
	return append(codes,
		jen.List(jen.Id(fnVar), jen.Id(argsVar)).Op(":=").List(callee, args),
		jen.Id(g.body.defers).Dot("Push").Call(jen.Func().Params().Error().Block(
			jen.If(
				jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Qual(pathObject, "Call").Call(jen.Id(fnVar), jen.Id(argsVar)),
				jen.Id(errVar).Op("!=").Nil(),
			).Block(jen.Return(jen.Qual(pathObject, "WithFrame").Call(jen.Id(errVar), frame))),
			jen.Return(jen.Nil()),
		)),
	), nil
}

// transpileTryCatch lowers `try { ... } catch e: Kind { ... } ...` using
// goto/labels so that `return`/`break`/`continue` inside the try body still
// target the real enclosing function/loop (a closure or defer/recover
// boundary would capture them). Inside the try body the error handler stores
// the error and jumps to the catch label; the unconditional post-block check
// guarantees the label is referenced even when the body has no fallible
// operation. The clauses are then tested in order, each kind evaluated with
// the enclosing handler, and the first match runs its body.
//
// When no clause matches, the error goes to the enclosing handler as if
// there were no try. That handler reports the position of the statement it
// is generated for, while the interpreter reports the statement inside the
// try body that failed; so each failing site records its index in excAt and
// the fall-through dispatches on it, generating the enclosing handler once
// per distinct site position.
//
// The first statement after the catch label hands an exit request
// (object.IsExit) to the enclosing handler, so os.exit unwinds through the
// try, running deferred calls on the way, and no clause receives it.
func (g *generator) transpileTryCatch(tc *ast.TryCatch, onError errHandler) ([]jen.Code, error) {
	excVar := g.localName("exc")
	catchLabel := g.localName("catch")
	doneLabel := g.localName("done")
	catchAll := tc.Clauses[len(tc.Clauses)-1].Kind == nil

	var atVar string
	var sites []token.Pos
	siteIndex := map[[2]int]int{}
	if !catchAll {
		atVar = g.localName("excAt")
	}

	tryOnError := func(errVar string) jen.Code {
		codes := []jen.Code{jen.Id(excVar).Op("=").Id(errVar)}
		if !catchAll {
			pos := g.errorPos
			key := [2]int{pos.Line, pos.Column}
			i, seen := siteIndex[key]
			if !seen {
				i = len(sites)
				siteIndex[key] = i
				sites = append(sites, pos)
			}
			codes = append(codes, jen.Id(atVar).Op("=").Lit(i))
		}
		return jen.Block(append(codes, jen.Goto().Id(catchLabel))...)
	}

	tryBody, err := g.transpileStatements(tc.TryBody, tryOnError)
	if err != nil {
		return nil, err
	}

	// An exit request (os.exit) is not an error any clause receives: it goes
	// straight to the enclosing handler, before any kind is evaluated. Its
	// position is never reported, so the try statement's own is used.
	savedPos := g.errorPos
	g.errorPos = tc.Position()
	exitGate := jen.If(jen.Qual(pathObject, "IsExit").Call(jen.Id(excVar))).Block(onError(excVar))
	g.errorPos = savedPos

	caughtVar := g.localName("caught")
	catchBlock := []jen.Code{
		exitGate,
		jen.Id(caughtVar).Op(":=").Qual(pathObject, "ErrorValue").Call(jen.Id(excVar)),
		jen.Id("_").Op("=").Id(caughtVar),
	}
	for _, clause := range tc.Clauses {
		// The kind is evaluated in the scope enclosing the try, so it is
		// generated before the clause variable is declared.
		var kindPre []jen.Code
		var kind *jen.Statement
		if clause.Kind != nil {
			kindPre, kind, err = g.transpileExpression(clause.Kind, onError)
			if err != nil {
				return nil, err
			}
		}
		// The clause variable lives in its own scope covering the clause
		// body, and shadows builtins there like any user name (the
		// interpreter defines it in a child scope).
		popScope := g.pushUserScope()
		g.declareUserName(clause.Var)
		body, err := g.transpileStatements(clause.Body, onError)
		popScope()
		if err != nil {
			return nil, err
		}
		run := append([]jen.Code{
			jen.Id(clause.Var).Op(":=").Id(caughtVar),
			jen.Id("_").Op("=").Id(clause.Var),
		}, body...)
		run = append(run, jen.Goto().Id(doneLabel))
		if kind == nil {
			catchBlock = append(catchBlock, jen.Block(run...))
			continue
		}
		matchedVar := g.localName("matched")
		errVar := g.localName("err")
		test := append(kindPre,
			jen.List(jen.Id(matchedVar), jen.Id(errVar)).Op(":=").Qual(pathObject, "ErrorMatches").Call(jen.Id(caughtVar), kind),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.If(jen.Id(matchedVar)).Block(run...),
		)
		catchBlock = append(catchBlock, jen.Block(test...))
	}

	var decls []jen.Code
	decls = append(decls, jen.Var().Id(excVar).Error())
	if !catchAll {
		// No clause matched: hand the original error to the enclosing
		// handler, positioned at the statement that failed.
		decls = append(decls, jen.Var().Id(atVar).Int())
		saved := g.errorPos
		switch len(sites) {
		case 0:
			catchBlock = append(catchBlock, jen.Id("_").Op("=").Id(atVar), onError(excVar))
		case 1:
			g.errorPos = sites[0]
			catchBlock = append(catchBlock, jen.Id("_").Op("=").Id(atVar), onError(excVar))
		default:
			var cases []jen.Code
			for i, pos := range sites {
				g.errorPos = pos
				if i == len(sites)-1 {
					cases = append(cases, jen.Default().Block(onError(excVar)))
				} else {
					cases = append(cases, jen.Case(jen.Lit(i)).Block(onError(excVar)))
				}
			}
			catchBlock = append(catchBlock, jen.Switch(jen.Id(atVar)).Block(cases...))
		}
		g.errorPos = saved
	}

	return append(decls,
		jen.Block(tryBody...),
		jen.If(jen.Id(excVar).Op("!=").Nil()).Block(jen.Goto().Id(catchLabel)),
		jen.Goto().Id(doneLabel),
		jen.Id(catchLabel).Op(":"),
		jen.Block(catchBlock...),
		jen.Id(doneLabel).Op(":"),
		jen.Id("_").Op("=").Id(excVar),
	), nil
}

func (g *generator) transpileStatement(stmt ast.Statement, onError errHandler) ([]jen.Code, error) {
	// Import produces no output; attaching a //line directive would orphan
	// it onto the following statement and skew its mapped line number.
	if _, isImport := stmt.(*ast.Import); isImport {
		return nil, nil
	}

	var prelude []jen.Code
	if d := lineDirective(stmt.Position()); d != nil {
		prelude = append(prelude, d)
	}

	var codes []jen.Code
	var err error
	switch v := stmt.(type) {
	case *ast.Declare:
		codes, err = g.transpileDeclare(v, onError)
	case *ast.Assign:
		codes, err = g.transpileAssign(v, onError)
	case *ast.Destructure:
		codes, err = g.transpileDestructure(v, onError, func(store ast.Statement) ([]jen.Code, error) {
			return g.transpileStatement(store, onError)
		})
	case *ast.SetIndex:
		codes, err = g.transpileSetIndex(v, onError)
	case *ast.SetAttr:
		codes, err = g.transpileSetAttr(v, onError)
	case *ast.CallExpression:
		var argPreStmts []jen.Code
		var call *jen.Statement
		argPreStmts, call, err = g.transpileCallExpression(v, onError)
		if err == nil {
			errVar := g.localName("err")
			codes = append(argPreStmts,
				jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			)
		}
	case *ast.FunctionDefine:
		codes, err = g.transpileFunctionDefine(v, onError)
	case *ast.TypeDefine:
		codes, err = g.transpileTypeDefine(v, onError)
	case *ast.IfElse:
		codes, err = g.transpileIfElse(v, onError)
	case *ast.While:
		codes, err = g.transpileWhile(v, onError)
	case *ast.For:
		codes, err = g.transpileFor(v, onError)
	case *ast.Break:
		codes, err = g.transpileBreak(v)
	case *ast.Continue:
		codes, err = g.transpileContinue(v)
	case *ast.Return:
		codes, err = g.transpileReturn(v, onError)
	case *ast.Raise:
		codes, err = g.transpileRaise(v, onError)
	case *ast.Defer:
		codes, err = g.transpileDefer(v, onError)
	case *ast.TryCatch:
		codes, err = g.transpileTryCatch(v, onError)
	case *ast.Export:
		codes, err = g.transpileExport(v)
	case *ast.BinaryOperation:
		var pre []jen.Code
		pre, _, err = g.transpileBinaryOperation(v, onError)
		codes = pre
	case *ast.UnaryOperation:
		var pre []jen.Code
		pre, _, err = g.transpileUnaryOperation(v, onError)
		codes = pre
	case *ast.MemberExpression:
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = g.transpileMemberExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	case *ast.IndexExpression:
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = g.transpileIndexExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	case ast.Expression:
		// Any other bare expression statement: evaluate it for its side
		// effects and discard the value.
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = g.transpileExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	default:
		return nil, fmt.Errorf("%s: cannot transpile statement of type %T", stmt.Position(), stmt)
	}
	if err != nil {
		return nil, err
	}
	return append(prelude, codes...), nil
}

func (g *generator) transpileStatements(stmts []ast.Statement, onError errHandler) ([]jen.Code, error) {
	// Each statement list renders as a Go block, so names declared inside it
	// must not shadow built-ins beyond the block's end.
	defer g.pushUserScope()()
	var result []jen.Code
	for _, stmt := range stmts {
		saved := g.errorPos
		g.errorPos = stmt.Position()
		codes, err := g.transpileStatement(stmt, onError)
		g.errorPos = saved
		if err != nil {
			return nil, err
		}
		result = append(result, codes...)
	}
	return result, nil
}
