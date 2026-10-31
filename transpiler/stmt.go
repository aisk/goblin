package transpiler

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/dave/jennifer/jen"
)

func (ctx *transpileContext) transpileDeclare(decl *ast.Declare, onError errHandler) ([]jen.Code, error) {
	if t := ctx.typeOf(decl.Name); t.native() {
		pre, value, err := ctx.emitNative(decl.Value, onError)
		if err != nil {
			return nil, err
		}
		ctx.declareUserName(decl.Name)
		declStmt := jen.Var().Id(decl.Name).Id(goTypeOf(t)).Op("=").Add(value)
		declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
		return append(pre, declStmt), nil
	}

	preStmts, value, err := ctx.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	ctx.declareUserName(decl.Name)
	declStmt := jen.Var().Id(decl.Name).Qual(pathObject, "Object").Op("=").Add(value)
	declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
	return append(preStmts, declStmt), nil
}

func (ctx *transpileContext) transpileAssign(decl *ast.Assign, onError errHandler) ([]jen.Code, error) {
	if ctx.typeOf(decl.Target).native() {
		pre, value, err := ctx.emitNative(decl.Value, onError)
		if err != nil {
			return nil, err
		}
		assignStmt := jen.Id(decl.Target).Op("=").Add(value)
		assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
		return append(pre, assignStmt), nil
	}

	preStmts, value, err := ctx.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	assignStmt := jen.Id(decl.Target).Op("=").Add(value)
	assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
	return append(preStmts, assignStmt), nil
}

func (ctx *transpileContext) transpileSetIndex(s *ast.SetIndex, onError errHandler) ([]jen.Code, error) {
	objPre, obj, err := ctx.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	idxPre, idx, native, err := ctx.transpileIndexOperand(s.Index, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := ctx.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	setter := "SetIndex"
	if native {
		setter = "SetIndexInt"
	}
	errVar := ctx.localName("err")
	stmts := append(objPre, idxPre...)
	stmts = append(stmts, valPre...)
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, setter).Call(obj, idx, val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (ctx *transpileContext) transpileSetAttr(s *ast.SetAttr, onError errHandler) ([]jen.Code, error) {
	// `self.field = v` inside a method is a plain Go field store.
	if info, ok := ctx.selfMember(s.Object, s.Property); ok && info.fields[s.Property] {
		valPre, val, err := ctx.transpileExpression(s.Value, onError)
		if err != nil {
			return nil, err
		}
		return append(valPre, jen.Id(info.receiver).Dot(fieldGoName(s.Property)).Op("=").Add(val)), nil
	}

	objPre, obj, err := ctx.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := ctx.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	errVar := ctx.localName("err")
	stmts := append(objPre, valPre...)
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "SetAttr").Call(obj, jen.Lit(s.Property), val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (ctx *transpileContext) transpileIfElse(ifelse *ast.IfElse, onError errHandler) ([]jen.Code, error) {
	pre, cond, err := ctx.transpileCondition(ifelse.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(ifelse.IfBody, onError, "")
	if err != nil {
		return nil, err
	}
	elseBody, err := ctx.transpileStatements(ifelse.ElseBody, onError, "")
	if err != nil {
		return nil, err
	}
	return append(pre, jen.If(cond).Block(body...).Else().Block(elseBody...)), nil
}

func (ctx *transpileContext) transpileWhile(while_ *ast.While, onError errHandler) ([]jen.Code, error) {
	condPre, cond, err := ctx.transpileCondition(while_.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(while_.Body, onError, "")
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

func (ctx *transpileContext) transpileBreak(break_ *ast.Break) ([]jen.Code, error) {
	return []jen.Code{jen.Break()}, nil
}

func (ctx *transpileContext) transpileContinue(continue_ *ast.Continue) ([]jen.Code, error) {
	return []jen.Code{jen.Continue()}, nil
}

func (ctx *transpileContext) transpileFor(for_ *ast.For, onError errHandler) ([]jen.Code, error) {
	// `for x in range(a, b)` runs as a native counting loop when `range` is
	// known to be the builtin, instead of materialising a list of boxed
	// integers only to iterate it once. The condition must stay in lockstep
	// with collectDeclarations, which types the loop variable accordingly.
	if ctx.rangeNative {
		if startExpr, endExpr, ok := rangeCallParts(for_.Iterator); ok {
			return ctx.transpileRangeFor(for_, startExpr, endExpr, onError)
		}
	}

	iterPreStmts, iterator, err := ctx.transpileExpression(for_.Iterator, onError)
	if err != nil {
		return nil, err
	}
	popScope := ctx.pushUserScope()
	ctx.declareUserName(for_.Variable)
	body, err := ctx.transpileStatements(for_.Body, onError, "")
	popScope()
	if err != nil {
		return nil, err
	}

	iterVar := ctx.localName("iter")
	elementsVar := ctx.localName("elements")
	errVar := ctx.localName("err")

	forLoopBody := []jen.Code{
		jen.Id(for_.Variable).Op(":=").Id(iterVar),
		jen.Id("_").Op("=").Id(for_.Variable),
	}
	forLoopBody = append(forLoopBody, body...)

	result := append(iterPreStmts,
		jen.List(jen.Id(elementsVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(iterator)).Dot("Iter").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.For(jen.List(jen.Id("_"), jen.Id(iterVar)).Op(":=").Op("range").Id(elementsVar)).Block(forLoopBody...),
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
func (ctx *transpileContext) transpileRangeFor(for_ *ast.For, startExpr, endExpr ast.Expression, onError errHandler) ([]jen.Code, error) {
	counterVar := ctx.localName("range_i")
	endVar := ctx.localName("range_end")

	var pre []jen.Code
	var startCode *jen.Statement
	if ctx.nativeTypeOf(startExpr) == tyInt && ctx.nativeTypeOf(endExpr) == tyInt {
		startPre, start, err := ctx.emitNative(startExpr, onError)
		if err != nil {
			return nil, err
		}
		endPre, end, err := ctx.emitNative(endExpr, onError)
		if err != nil {
			return nil, err
		}
		pre = append(append(pre, startPre...), endPre...)
		pre = append(pre, jen.Id(endVar).Op(":=").Add(end))
		startCode = start
	} else {
		startPre, start, err := ctx.transpileExpression(startExpr, onError)
		if err != nil {
			return nil, err
		}
		endPre, end, err := ctx.transpileExpression(endExpr, onError)
		if err != nil {
			return nil, err
		}
		startVar := ctx.localName("range_start")
		errVar := ctx.localName("err")
		pre = append(append(append(pre, startPre...), endPre...),
			jen.List(jen.Id(startVar), jen.Id(endVar), jen.Id(errVar)).Op(":=").Qual(pathExtension, "RangeBounds").Call(start, end),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		startCode = jen.Id(startVar)
	}

	popScope := ctx.pushUserScope()
	ctx.declareUserName(for_.Variable)
	body, err := ctx.transpileStatements(for_.Body, onError, "")
	popScope()
	if err != nil {
		return nil, err
	}

	loopBody := make([]jen.Code, 0, len(body)+2)
	if ctx.typeOf(for_.Variable) == tyInt {
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

func (ctx *transpileContext) transpileReturn(return_ *ast.Return, onError errHandler) ([]jen.Code, error) {
	if ctx.retType.native() {
		// The signature inference only makes a return type native when every
		// reachable return agrees with it; the parser's implicit `return nil`
		// is then unreachable and only has to keep Go's terminating-statement
		// rule satisfied.
		if ctx.nativeTypeOf(return_.Value) == ctx.retType {
			pre, value, err := ctx.emitNative(return_.Value, onError)
			if err != nil {
				return nil, err
			}
			return append(pre, jen.Return(jen.List(value, jen.Nil()))), nil
		}
		if isImplicitReturn(return_) {
			return []jen.Code{jen.Return(jen.List(zeroOf(ctx.retType), jen.Nil()))}, nil
		}
		return nil, fmt.Errorf("%s: internal error: return does not match the inferred native return type", return_.Position())
	}

	preStmts, r, err := ctx.transpileExpression(return_.Value, onError)
	if err != nil {
		return nil, err
	}
	return append(preStmts, jen.Return(jen.List(r, jen.Nil()))), nil
}

// transpileRaise turns `raise expr` into an error (the raised Error value, or a
// type error if expr is not an Error) that is then routed through the active
// error handler (a top-level `return`, or a jump to the enclosing `try`'s catch
// label).
func (ctx *transpileContext) transpileRaise(raise *ast.Raise, onError errHandler) ([]jen.Code, error) {
	preStmts, val, err := ctx.transpileExpression(raise.Value, onError)
	if err != nil {
		return nil, err
	}
	errVar := ctx.localName("err")
	code := append(preStmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "Raise").Call(val),
		onError(errVar),
	)
	return code, nil
}

// transpileTryCatch lowers `try { ... } catch e { ... }` using goto/labels so
// that `return`/`break`/`continue` inside the try body still target the real
// enclosing function/loop (a closure or defer/recover boundary would capture
// them). Inside the try body the error handler stores the error and jumps to
// the catch label; the unconditional post-block check guarantees the label is
// referenced even when the body has no fallible operation.
func (ctx *transpileContext) transpileTryCatch(tc *ast.TryCatch, onError errHandler) ([]jen.Code, error) {
	excVar := ctx.localName("exc")
	catchLabel := ctx.localName("catch")
	doneLabel := ctx.localName("done")

	tryOnError := func(errVar string) jen.Code {
		return jen.Block(
			jen.Id(excVar).Op("=").Id(errVar),
			jen.Goto().Id(catchLabel),
		)
	}

	tryBody, err := ctx.transpileStatements(tc.TryBody, tryOnError, "")
	if err != nil {
		return nil, err
	}
	// The catch variable lives in its own scope covering the catch body, and
	// shadows builtins there like any user name (the interpreter defines it in
	// a child environment).
	popCatchScope := ctx.pushUserScope()
	ctx.declareUserName(tc.CatchVar)
	catchBody, err := ctx.transpileStatements(tc.CatchBody, onError, "")
	popCatchScope()
	if err != nil {
		return nil, err
	}

	catchBlock := []jen.Code{
		jen.Id(tc.CatchVar).Op(":=").Qual(pathObject, "ErrorValue").Call(jen.Id(excVar)),
		jen.Id("_").Op("=").Id(tc.CatchVar),
	}
	catchBlock = append(catchBlock, catchBody...)

	return []jen.Code{
		jen.Var().Id(excVar).Error(),
		jen.Block(tryBody...),
		jen.If(jen.Id(excVar).Op("!=").Nil()).Block(jen.Goto().Id(catchLabel)),
		jen.Goto().Id(doneLabel),
		jen.Id(catchLabel).Op(":"),
		jen.Block(catchBlock...),
		jen.Id(doneLabel).Op(":"),
		jen.Id("_").Op("=").Id(excVar),
	}, nil
}

func (ctx *transpileContext) transpileStatement(stmt ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
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
		codes, err = ctx.transpileDeclare(v, onError)
	case *ast.Assign:
		codes, err = ctx.transpileAssign(v, onError)
	case *ast.SetIndex:
		codes, err = ctx.transpileSetIndex(v, onError)
	case *ast.SetAttr:
		codes, err = ctx.transpileSetAttr(v, onError)
	case *ast.CallExpression:
		var argPreStmts []jen.Code
		var call *jen.Statement
		argPreStmts, call, err = ctx.transpileCallExpression(v, onError)
		if err == nil {
			errVar := ctx.localName("err")
			codes = append(argPreStmts,
				jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			)
		}
	case *ast.FunctionDefine:
		codes, err = ctx.transpileFunctionDefine(v, onError)
	case *ast.TypeDefine:
		codes, err = ctx.transpileTypeDefine(v, onError)
	case *ast.IfElse:
		codes, err = ctx.transpileIfElse(v, onError)
	case *ast.While:
		codes, err = ctx.transpileWhile(v, onError)
	case *ast.For:
		codes, err = ctx.transpileFor(v, onError)
	case *ast.Break:
		codes, err = ctx.transpileBreak(v)
	case *ast.Continue:
		codes, err = ctx.transpileContinue(v)
	case *ast.Return:
		codes, err = ctx.transpileReturn(v, onError)
	case *ast.Raise:
		codes, err = ctx.transpileRaise(v, onError)
	case *ast.TryCatch:
		codes, err = ctx.transpileTryCatch(v, onError)
	case *ast.Export:
		codes, err = ctx.transpileExport(v, exportsVar)
	case *ast.BinaryOperation:
		var pre []jen.Code
		pre, _, err = ctx.transpileBinaryOperation(v, onError)
		codes = pre
	case *ast.UnaryOperation:
		var pre []jen.Code
		pre, _, err = ctx.transpileUnaryOperation(v, onError)
		codes = pre
	case *ast.MemberExpression:
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = ctx.transpileMemberExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	case *ast.IndexExpression:
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = ctx.transpileIndexExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	case ast.Expression:
		// Any other bare expression statement: evaluate it for its side
		// effects and discard the value.
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = ctx.transpileExpression(v, onError)
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

func (ctx *transpileContext) transpileStatements(stmts []ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	// Each statement list renders as a Go block, so names declared inside it
	// must not shadow built-ins beyond the block's end.
	defer ctx.pushUserScope()()
	var result []jen.Code
	for _, stmt := range stmts {
		saved := ctx.errorPos
		ctx.errorPos = stmt.Position()
		codes, err := ctx.transpileStatement(stmt, onError, exportsVar)
		ctx.errorPos = saved
		if err != nil {
			return nil, err
		}
		result = append(result, codes...)
	}
	return result, nil
}
