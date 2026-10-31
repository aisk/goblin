package transpiler

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

// emitParamDefaults emits the declaration of an []object.ParamDefault literal
// holding one lazy closure per defaulted parameter (nil per required one). It
// returns nil code and an empty name when no parameter declares a default.
// Callers must place the declaration before the parameter variable
// declarations: the closures must capture the enclosing scope, not the
// parameters a default expression may share a name with.
func (g *generator) emitParamDefaults(params []*ast.Parameter) (jen.Code, string, error) {
	hasDefault := false
	for _, param := range params {
		if param.HasDefault() {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		return nil, "", nil
	}

	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Id(errVar))
	}

	var entries []jen.Code
	for _, param := range params {
		if param.VarArgs || param.KwArgs {
			continue
		}
		if !param.HasDefault() {
			entries = append(entries, jen.Nil())
			continue
		}
		pre, value, err := g.transpileExpression(param.Default, onError)
		if err != nil {
			return nil, "", err
		}
		body := append(pre, jen.Return(value, jen.Nil()))
		entries = append(entries, jen.Func().Params().Parens(jen.List(
			jen.Qual(pathObject, "Object"), jen.Id("error")),
		).Block(body...))
	}

	name := g.localName("defaults")
	decl := jen.Id(name).Op(":=").Index().Qual(pathObject, "ParamDefault").Values(entries...)
	return decl, name, nil
}

// emitParameterBinding emits the statements that bind a CallArgs value (the
// local named callArgsName) to one Go variable per parameter. It splits params
// into fixed / *varargs / **kwargs, calls object.BindArguments, and unpacks the
// slice it returns. name is used only for BindArguments diagnostics, defaultsName
// names the enclosing []object.ParamDefault emitted by emitParamDefaults (""
// when no parameter has a default), and fnOnError builds the error-return
// emitted when binding fails. It is shared by named functions, anonymous
// literals, and type methods.
func (g *generator) emitParameterBinding(name string, params []*ast.Parameter, defaultsName string, callArgsName string, fnOnError errHandler) []jen.Code {
	var varArgsParam *ast.Parameter
	var kwArgsParam *ast.Parameter
	fixedParams := make([]*ast.Parameter, 0, len(params))
	for _, param := range params {
		switch {
		case param.VarArgs:
			varArgsParam = param
		case param.KwArgs:
			kwArgsParam = param
		default:
			fixedParams = append(fixedParams, param)
		}
	}

	fixedParamNames := make([]jen.Code, 0, len(fixedParams))
	for _, param := range fixedParams {
		fixedParamNames = append(fixedParamNames, jen.Lit(param.Name))
	}

	boundName := g.localName("bound")
	errVar := g.localName("err")
	varArgsName := ""
	if varArgsParam != nil {
		varArgsName = varArgsParam.Name
	}
	kwArgsName := ""
	if kwArgsParam != nil {
		kwArgsName = kwArgsParam.Name
	}

	bindCall := func() jen.Code {
		defaultsArg := jen.Nil()
		if defaultsName != "" {
			defaultsArg = jen.Id(defaultsName)
		}
		return jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(name),
			jen.Index().String().Values(fixedParamNames...),
			defaultsArg,
			jen.Lit(varArgsName),
			jen.Lit(kwArgsName),
			jen.Id(callArgsName),
		)
	}

	// A function without varargs/kwargs that is called with exactly its fixed
	// parameters, all positional, needs no binding at all: the parameter
	// names and their positions are known here, at transpile time. The call
	// shape is not — functions are first-class values — so the check is emitted
	// into the generated code, with BindArguments kept as the fallback so
	// diagnostics for every other shape stay identical.
	if varArgsParam == nil && kwArgsParam == nil {
		stmts := make([]jen.Code, 0, len(fixedParams)*2+1)
		for _, param := range fixedParams {
			stmts = append(stmts,
				jen.Var().Id(param.Name).Qual(pathObject, "Object"),
				jen.Id("_").Op("=").Id(param.Name),
			)
		}

		fastBody := make([]jen.Code, 0, len(fixedParams))
		for i, param := range fixedParams {
			fastBody = append(fastBody,
				jen.Id(param.Name).Op("=").Id(callArgsName).Dot("Positional").Index(jen.Lit(i)),
			)
		}

		slowBody := []jen.Code{
			bindCall(),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(fnOnError(errVar)),
			// A zero-parameter function still needs the call for its arity
			// check, but then never reads the result.
			jen.Id("_").Op("=").Id(boundName),
		}
		for i, param := range fixedParams {
			slowBody = append(slowBody,
				jen.Id(param.Name).Op("=").Id(boundName).Index(jen.Lit(i)),
			)
		}

		fastCond := jen.Len(jen.Id(callArgsName).Dot("Keyword")).Op("==").Lit(0).
			Op("&&").
			Len(jen.Id(callArgsName).Dot("Positional")).Op("==").Lit(len(fixedParams))

		return append(stmts, jen.If(fastCond).Block(fastBody...).Else().Block(slowBody...))
	}

	stmts := []jen.Code{
		bindCall(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(fnOnError(errVar)),
		jen.Id("_").Op("=").Id(boundName),
	}

	// BindArguments returns one slot per fixed parameter in declaration
	// order, followed by the *varargs list and the **kwargs dict.
	slot := 0
	emit := func(param *ast.Parameter) {
		stmts = append(stmts,
			jen.Var().Id(param.Name).Qual(pathObject, "Object").Op("=").Id(boundName).Index(jen.Lit(slot)),
			jen.Id("_").Op("=").Id(param.Name),
		)
		slot++
	}
	for _, param := range fixedParams {
		emit(param)
	}
	if varArgsParam != nil {
		emit(varArgsParam)
	}
	if kwArgsParam != nil {
		emit(kwArgsParam)
	}

	return stmts
}

// buildFunctionValue emits an `&object.Function{...}` expression that wraps a
// closure binding the given parameters and running the given body. It is shared
// by named function definitions and anonymous function literals. name is used
// only for the runtime function's repr and for BindArguments diagnostics.
func (g *generator) buildFunctionValue(name string, pos token.Pos, params []*ast.Parameter, body []ast.Statement) (*jen.Statement, error) {
	// Default expressions belong to the enclosing scope, so they are transpiled
	// before entering the body's inference scope.
	defaultsDecl, defaultsName, err := g.emitParamDefaults(params)
	if err != nil {
		return nil, err
	}

	// Each function body gets its own inferred types; parameters stay boxed,
	// since a caller can pass anything.
	defer g.enterFunction(params, body, nil, tyDynamic, nil)()

	callArgsName := g.localName("callArgs")

	module := sourceModuleName(pos)
	fnOnError := func(errVar string) jen.Code {
		return tracedReturn(jen.Nil(), errVar, module, name, g.framePos(pos))
	}

	// Binding errors point at the definition, like the interpreter's frame
	// for BindArgumentsInto failures.
	savedPos := g.errorPos
	g.errorPos = pos
	argsDefine := g.emitParameterBinding(name, params, defaultsName, callArgsName, fnOnError)
	g.errorPos = savedPos
	if defaultsDecl != nil {
		argsDefine = append([]jen.Code{defaultsDecl}, argsDefine...)
	}

	bodyCode, err := g.transpileStatements(body, fnOnError)
	if err != nil {
		return nil, err
	}

	bodyCode = append(argsDefine, bodyCode...)

	closure := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Id("error")),
	).Block(bodyCode...)

	return jen.Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(name),
		jen.Id("Fn").Op(":").Add(closure),
	), nil
}

// buildDirectFunction generates a direct-lowered module-level function: a
// package-level Go function holding the transpiled body, with one Go
// parameter per Goblin parameter, and the returned assignment of the
// &object.Function wrapper adapting CallArgs-shaped calls onto it. The
// wrapper reproduces the fast/slow binding split buildFunctionValue
// generates, so every call shape that is not lowered — keyword arguments,
// wrong arity, the function used as a value — behaves exactly as before,
// diagnostics included.
func (g *generator) buildDirectFunction(info directFn, fn *ast.FunctionDefine) ([]jen.Code, error) {
	name, pos, params, body := fn.Name, fn.Position(), fn.Parameters, fn.Body

	// A closed function (see signatures.go) carries an inferred signature:
	// its parameters and result take the native Go types the signature
	// says, and it needs no generic wrapper since nothing can reach it
	// other than the lowered call sites. Closedness excludes parameter
	// defaults, so there are none to evaluate for it.
	sig := g.sigs[name]
	var seed map[string]staticType
	retType := tyDynamic
	var defaultsDecl jen.Code
	defaultsName := ""
	if sig != nil {
		seed = make(map[string]staticType, len(params))
		for i, param := range params {
			seed[param.Name] = sig.params[i]
		}
		retType = sig.ret
	} else {
		// Default expressions belong to the enclosing scope, so they are
		// transpiled before the parameters are declared.
		var err error
		defaultsDecl, defaultsName, err = g.emitParamDefaults(params)
		if err != nil {
			return nil, err
		}
	}

	leave := g.enterFunction(params, body, seed, retType, nil)

	module := sourceModuleName(pos)
	zero := zeroOf(retType)
	fnOnError := func(errVar string) jen.Code {
		return tracedReturn(zero, errVar, module, name, g.framePos(pos))
	}

	bodyCode, err := g.transpileStatements(body, fnOnError)
	leave()
	if err != nil {
		return nil, err
	}

	goType := func(t staticType) *jen.Statement {
		if t.native() {
			return jen.Id(goTypeOf(t))
		}
		return jen.Qual(pathObject, "Object")
	}
	paramDecls := make([]jen.Code, 0, len(params))
	for i, param := range params {
		paramType := tyDynamic
		if sig != nil {
			paramType = sig.params[i]
		}
		paramDecls = append(paramDecls, jen.Id(param.Name).Add(goType(paramType)))
	}
	g.topDecls = append(g.topDecls,
		jen.Func().Id(info.goName).Params(paramDecls...).Parens(jen.List(
			goType(retType), jen.Id("error"),
		)).Block(bodyCode...))
	if sig != nil {
		return nil, nil
	}

	callArgsName := g.localName("callArgs")
	boundName := g.localName("bound")
	errVar := g.localName("err")

	fastArgs := make([]jen.Code, 0, len(params))
	slowArgs := make([]jen.Code, 0, len(params))
	paramNames := make([]jen.Code, 0, len(params))
	for i, param := range params {
		fastArgs = append(fastArgs, jen.Id(callArgsName).Dot("Positional").Index(jen.Lit(i)))
		slowArgs = append(slowArgs, jen.Id(boundName).Index(jen.Lit(i)))
		paramNames = append(paramNames, jen.Lit(param.Name))
	}

	// Binding errors point at the definition, like buildFunctionValue's.
	savedPos := g.errorPos
	g.errorPos = pos
	bindErrReturn := fnOnError(errVar)
	g.errorPos = savedPos

	defaultsArg := jen.Nil()
	if defaultsName != "" {
		defaultsArg = jen.Id(defaultsName)
	}

	wrapperBody := []jen.Code{
		jen.If(
			jen.Len(jen.Id(callArgsName).Dot("Keyword")).Op("==").Lit(0).
				Op("&&").
				Len(jen.Id(callArgsName).Dot("Positional")).Op("==").Lit(len(params)),
		).Block(
			jen.Return(jen.Id(info.goName).Call(fastArgs...)),
		),
	}
	if defaultsDecl != nil {
		wrapperBody = append(wrapperBody, defaultsDecl)
	}
	wrapperBody = append(wrapperBody,
		jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(name),
			jen.Index().String().Values(paramNames...),
			defaultsArg,
			jen.Lit(""),
			jen.Lit(""),
			jen.Id(callArgsName),
		),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(bindErrReturn),
		// A zero-parameter function still needs the call for its arity check,
		// but then never reads the result.
		jen.Id("_").Op("=").Id(boundName),
		jen.Return(jen.Id(info.goName).Call(slowArgs...)),
	)

	wrapper := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Id("error"),
	)).Block(wrapperBody...)

	return []jen.Code{
		jen.Id(name).Op("=").Op("&").Qual(pathObject, "Function").Values(
			jen.Id("Name").Op(":").Lit(name),
			jen.Id("Fn").Op(":").Add(wrapper),
		),
	}, nil
}

func (g *generator) transpileFunctionDefine(fn *ast.FunctionDefine, onError errHandler) ([]jen.Code, error) {
	// Declared before the body transpiles so the function can shadow a
	// built-in even in recursive references to itself.
	g.declareUserName(fn.Name)
	funcValue, err := g.buildFunctionValue(fn.Name, fn.Position(), fn.Parameters, fn.Body)
	if err != nil {
		return nil, err
	}

	// Declare before assigning so the closure body can reference the
	// function's own name for recursion (a combined `var f = ...` would keep
	// f out of scope inside its own initializer under Go scoping rules).
	result := jen.Var().Id(fn.Name).Qual(pathObject, "Object")
	result.Op(";").Id(fn.Name).Op("=").Add(funcValue)
	result.Op(";").Id("_").Op("=").Id(fn.Name)

	return []jen.Code{result}, nil
}

func (g *generator) transpileFunctionLiteral(fn *ast.FunctionLiteral) ([]jen.Code, *jen.Statement, error) {
	funcValue, err := g.buildFunctionValue("<lambda>", fn.Position(), fn.Parameters, fn.Body)
	if err != nil {
		return nil, nil, err
	}
	return nil, funcValue, nil
}
