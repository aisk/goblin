package transpiler

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

func (ctx *transpileContext) transpileTypeDefine(typeDef *ast.TypeDefine, onError errHandler) ([]jen.Code, error) {
	ctorVarName := typeDef.Name + "Constructor"
	ctx.moduleImports[typeDef.Name] = ctorVarName
	goTypeName := ctx.goTypeName(typeDef.Name)
	receiverName := receiverGoName

	structFields := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		structFields = append(structFields, jen.Id(fieldGoName(field.Name)).Qual(pathObject, "Object"))
	}

	ctx.topDecls = append(ctx.topDecls, jen.Type().Id(goTypeName).Struct(structFields...))

	// typeVar holds the type's object.UserType: its impls, built when the type
	// statement runs, since the traits they name are runtime values.
	typeVar := ctx.localName("type_" + typeDef.Name)
	ctx.topDecls = append(ctx.topDecls, jen.Var().Id(typeVar).Op("*").Qual(pathObject, "UserType"))

	receiverParam := func() jen.Code { return jen.Id(receiverName).Op("*").Id(goTypeName) }
	obj := func() *jen.Statement { return jen.Qual(pathObject, "Object") }
	// userCall builds `return object.<helper>(receiver, args...)`: every
	// operator and conversion dispatches through the type's impls in the
	// object package, exactly as the interpreter's instances do.
	userCall := func(helper string, args ...jen.Code) jen.Code {
		return jen.Return(jen.Qual(pathObject, helper).Call(append([]jen.Code{jen.Id(receiverName)}, args...)...))
	}
	protoDecls := make([]jen.Code, 0, 24)
	method := func(name string, params []jen.Code, results jen.Code, body ...jen.Code) {
		protoDecls = append(protoDecls, jen.Func().Params(receiverParam()).Id(name).Params(params...).Add(results).Block(body...))
	}
	other := func(name string) []jen.Code { return []jen.Code{jen.Id(name).Add(obj())} }

	// TypeName() string — the declared Goblin name, so diagnostics never leak
	// the generated Go type (the interpreter reports the same name here).
	method("TypeName", nil, jen.String(), jen.Return(jen.Lit(typeDef.Name)))
	method("UserType", nil, jen.Op("*").Qual(pathObject, "UserType"), jen.Return(jen.Id(typeVar)))
	fieldValues := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldValues = append(fieldValues, jen.Id(receiverName).Dot(fieldGoName(field.Name)))
	}
	method("FieldValues", nil, jen.Index().Add(obj()), jen.Return(jen.Index().Add(obj()).Values(fieldValues...)))
	method("String", nil, jen.String(), userCall("UserString"))
	method("ToString", nil, jen.Parens(jen.List(jen.String(), jen.Error())), userCall("UserToString"))
	method("ToBool", nil, jen.Parens(jen.List(jen.Bool(), jen.Error())), userCall("UserToBool"))
	method("Hash", nil, jen.Parens(jen.List(jen.Uint64(), jen.Error())), userCall("UserHash"))
	method("Equals", other("other"), jen.Parens(jen.List(jen.Bool(), jen.Error())), userCall("UserEquals", jen.Id("other")))
	method("Compare", other("other"), jen.Parens(jen.List(jen.Int(), jen.Error())), userCall("UserCompare", jen.Id("other")))

	// A binary operator whose arithmetic trait this type implements calls the
	// forward method's wrapper directly, skipping the impl lookup and the
	// argument slice the generic dispatch allocates. A missing operator goes
	// through the shared helper, which raises its error.
	implWrappers := make(map[*ast.FunctionDefine]string)
	for _, block := range typeDef.Impls {
		for _, m := range block.Methods {
			implWrappers[m] = ctx.localName("impl_" + m.Name)
		}
	}
	// forward maps each implemented built-in trait to the wrapper of its first
	// method; for the arithmetic traits that is the forward operator.
	forward := map[*object.Trait]string{}
	for _, block := range typeDef.Impls {
		trait := ctx.builtinTrait(block.Trait)
		for _, m := range block.Methods {
			if trait != nil && m.Name == trait.Methods[0].Name {
				forward[trait] = implWrappers[m]
			}
		}
	}
	for _, op := range []struct {
		goMethod, index string
		trait           *object.Trait
	}{
		{"Add", "ArithAdd", object.AddTrait},
		{"Minus", "ArithSub", object.SubTrait},
		{"Multiply", "ArithMul", object.MulTrait},
		{"Divide", "ArithDiv", object.DivTrait},
		{"Modulo", "ArithMod", object.ModTrait},
	} {
		if wrapper, ok := forward[op.trait]; ok {
			method(op.goMethod, other("other"), jen.Parens(jen.List(obj(), jen.Error())),
				jen.Return(jen.Id(receiverName).Dot(wrapper).Call(jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
					jen.Id("Positional"): jen.Qual(pathObject, "Args").Values(jen.Id("other")),
				}))))
			continue
		}
		method(op.goMethod, other("other"), jen.Parens(jen.List(obj(), jen.Error())),
			userCall("UserArith", jen.Qual(pathObject, op.index), jen.Id("other")))
	}
	for _, op := range []struct{ goMethod, index string }{
		{"RAdd", "ArithAdd"},
		{"RMinus", "ArithSub"},
		{"RMultiply", "ArithMul"},
		{"RDivide", "ArithDiv"},
		{"RModulo", "ArithMod"},
	} {
		method(op.goMethod, other("left"), jen.Parens(jen.List(obj(), jen.Bool(), jen.Error())),
			userCall("UserReflected", jen.Qual(pathObject, op.index), jen.Id("left")))
	}
	method("Iter", nil, jen.Parens(jen.List(jen.Index().Add(obj()), jen.Error())), userCall("UserIter"))
	method("Index", other("index"), jen.Parens(jen.List(obj(), jen.Error())), userCall("UserIndex", jen.Id("index")))
	method("SetIndex", []jen.Code{jen.Id("index").Add(obj()), jen.Id("value").Add(obj())}, jen.Parens(jen.List(jen.Bool(), jen.Error())),
		userCall("UserSetIndex", jen.Id("index"), jen.Id("value")))

	ctx.topDecls = append(ctx.topDecls, protoDecls...)

	getAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+len(typeDef.Methods)+2)
	attributeNames := make([]jen.Code, 0, len(typeDef.Fields)+len(typeDef.Methods)+2)
	seenAttributes := make(map[string]bool, cap(attributeNames))
	for _, field := range typeDef.Fields {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Return(jen.Id(receiverName).Dot(fieldGoName(field.Name)), jen.Nil()),
			),
		)
		if !seenAttributes[field.Name] {
			attributeNames = append(attributeNames, jen.Lit(field.Name))
			seenAttributes[field.Name] = true
		}
	}
	for _, method := range typeDef.Methods {
		wrapperName := methodWrapperName(method.Name)
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit(method.Name)).Block(
				jen.Return(
					jen.Op("&").Qual(pathObject, "Function").Values(
						jen.Id("Name").Op(":").Lit(method.Name),
						jen.Id("Fn").Op(":").Id(receiverName).Dot(wrapperName),
					),
					jen.Nil(),
				),
			),
		)
		if !seenAttributes[method.Name] {
			attributeNames = append(attributeNames, jen.Lit(method.Name))
			seenAttributes[method.Name] = true
		}
	}
	userDefinedConstructor := false
	for _, field := range typeDef.Fields {
		if field.Name == "constructor" {
			userDefinedConstructor = true
			break
		}
	}
	if !userDefinedConstructor {
		for _, method := range typeDef.Methods {
			if method.Name == "constructor" {
				userDefinedConstructor = true
				break
			}
		}
	}
	if !userDefinedConstructor {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit("constructor")).Block(
				jen.Return(jen.Id(ctorVarName), jen.Nil()),
			),
		)
		attributeNames = append(attributeNames, jen.Lit("constructor"))
		seenAttributes["constructor"] = true
	}
	if !seenAttributes["attributes"] {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit("attributes")).Block(
				jen.Return(jen.Qual(pathObject, "AttributesFunction").Call(jen.Id(receiverName)), jen.Nil()),
			),
		)
		attributeNames = append(attributeNames, jen.Lit("attributes"))
		seenAttributes["attributes"] = true
	}
	if !seenAttributes["traits"] {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit("traits")).Block(
				jen.Return(jen.Qual(pathObject, "TraitsFunction").Call(jen.Id(receiverName)), jen.Nil()),
			),
		)
		attributeNames = append(attributeNames, jen.Lit("traits"))
	}
	getAttrCases = append(getAttrCases,
		jen.Default().Block(
			jen.Return(
				jen.Nil(),
				jen.Qual(pathObject, "NewAttributeError").Call(jen.Lit("%s has no attribute '%s'"), jen.Lit(typeDef.Name), jen.Id("name")),
			),
		),
	)

	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("GetAttr").Params(
			jen.Id("name").String(),
		).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(
			jen.Switch(jen.Id("name")).Block(getAttrCases...),
		),
	)
	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("Attributes").Params().Index().String().Block(
			jen.Return(jen.Index().String().Values(attributeNames...)),
		),
	)

	// CallMethod satisfies object.MethodCaller, so `p.step()` calls the
	// method body directly instead of allocating the bound function GetAttr
	// hands out. Only real methods are answered here: a field shadowing a
	// method name, "constructor" and "attributes" all fall through to
	// GetAttr, which keeps their behavior exactly as it was.
	fieldNamed := make(map[string]bool, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNamed[field.Name] = true
	}
	callMethodCases := make([]jen.Code, 0, len(typeDef.Methods))
	for _, method := range typeDef.Methods {
		if fieldNamed[method.Name] {
			continue
		}
		callMethodCases = append(callMethodCases,
			jen.Case(jen.Lit(method.Name)).Block(
				jen.List(jen.Id("_value"), jen.Id("_err")).Op(":=").Id(receiverName).Dot(methodWrapperName(method.Name)).Call(jen.Id("_args")),
				jen.Return(jen.Id("_value"), jen.True(), jen.Id("_err")),
			),
		)
	}
	if len(callMethodCases) > 0 {
		ctx.topDecls = append(ctx.topDecls,
			jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("CallMethod").Params(
				jen.Id("name").String(), jen.Id("_args").Qual(pathObject, "CallArgs"),
			).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Bool(), jen.Error())).Block(
				jen.Switch(jen.Id("name")).Block(callMethodCases...),
				jen.Return(jen.Nil(), jen.False(), jen.Nil()),
			),
		)
	}

	setAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+1)
	for _, field := range typeDef.Fields {
		setAttrCases = append(setAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Id(receiverName).Dot(fieldGoName(field.Name)).Op("=").Id("value"),
				jen.Return(jen.True(), jen.Nil()),
			),
		)
	}
	setAttrCases = append(setAttrCases,
		jen.Default().Block(
			jen.Return(
				jen.True(),
				jen.Qual(pathObject, "NewAttributeError").Call(jen.Lit("%s has no attribute '%s'"), jen.Lit(typeDef.Name), jen.Id("name")),
			),
		),
	)
	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("SetAttr").Params(
			jen.Id("name").String(), jen.Id("value").Qual(pathObject, "Object"),
		).Parens(jen.List(jen.Bool(), jen.Error())).Block(
			jen.Switch(jen.Id("name")).Block(setAttrCases...),
		),
	)

	fieldSet := make(map[string]bool, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldSet[field.Name] = true
	}
	methodWrappers := make(map[string]string, len(typeDef.Methods))
	for _, method := range typeDef.Methods {
		methodWrappers[method.Name] = methodWrapperName(method.Name)
	}

	// Ordinary methods and impl methods compile alike, to a Go method taking
	// the arguments after self. Impl methods get their own wrapper names, so
	// they never collide with an ordinary method or with each other.
	type methodDef struct {
		def     *ast.FunctionDefine
		wrapper string
	}
	methodDefs := make([]methodDef, 0, len(typeDef.AllMethods()))
	for _, method := range typeDef.Methods {
		methodDefs = append(methodDefs, methodDef{method, methodWrapperName(method.Name)})
	}
	for _, block := range typeDef.Impls {
		for _, method := range block.Methods {
			methodDefs = append(methodDefs, methodDef{method, implWrappers[method]})
		}
	}

	for _, md := range methodDefs {
		method, wrapperName := md.def, md.wrapper

		// A method body is a scope of its own, with its own inferred
		// environment, like any other function body.
		popMethodScope := ctx.pushUserScope()
		for _, param := range method.Parameters {
			ctx.declareUserName(param.Name)
		}
		restoreMethodTypes := ctx.enterScope(method.Body, nil, tyDynamic)
		var self *selfInfo
		if !bodyRebinds(method.Body, "self") {
			self = &selfInfo{receiver: receiverName, fields: fieldSet, methods: methodWrappers}
		}
		restoreSelf := ctx.withSelfType(self)

		callArgsName := ctx.localName("callArgs")
		methodModule := sourceModuleName(method.Position())
		qualifiedName := typeDef.Name + "." + method.Name
		fnOnError := func(errVar string) jen.Code {
			return tracedReturn(jen.Nil(), errVar, methodModule, qualifiedName, ctx.framePos(method.Position()))
		}
		savedPos := ctx.errorPos
		ctx.errorPos = method.Position()

		var bodyPrefix []jen.Code
		defaultsDecl, defaultsName, err := ctx.emitParamDefaults(method.Parameters[1:])
		if err != nil {
			return nil, err
		}
		if defaultsDecl != nil {
			bodyPrefix = append(bodyPrefix, defaultsDecl)
		}
		bodyPrefix = append(bodyPrefix,
			ctx.emitParameterBinding(qualifiedName, method.Parameters[1:], defaultsName, callArgsName, fnOnError)...,
		)
		bodyPrefix = append(bodyPrefix,
			jen.Var().Id("self").Qual(pathObject, "Object").Op("=").Id(receiverName),
			jen.Id("_").Op("=").Id("self"),
		)

		methodBody, err := ctx.transpileStatements(method.Body, fnOnError, "")
		ctx.errorPos = savedPos
		restoreSelf()
		restoreMethodTypes()
		popMethodScope()
		if err != nil {
			return nil, err
		}

		ctx.topDecls = append(ctx.topDecls,
			jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id(wrapperName).Params(
				jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
			).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(
				append(bodyPrefix, methodBody...)...,
			),
		)
	}

	// The impls are registered when the type statement runs: a trait is a
	// runtime value, and an impl may name one from an imported module.
	fieldNameLits := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNameLits = append(fieldNameLits, jen.Lit(field.Name))
	}
	registration := []jen.Code{
		jen.Id(typeVar).Op("=").Qual(pathObject, "NewUserType").Call(jen.Lit(typeDef.Name), jen.Index().String().Values(fieldNameLits...)),
	}
	for _, block := range typeDef.Impls {
		pre, trait, err := ctx.transpileTraitRef(block.Trait, "impl "+block.Trait.String(), onError)
		if err != nil {
			return nil, err
		}
		registration = append(registration, pre...)
		methods := make([]jen.Code, 0, len(block.Methods))
		for _, method := range block.Methods {
			argsName := ctx.localName("implArgs")
			call := jen.Id(argsName).Dot("Positional").Index(jen.Lit(0)).Assert(jen.Op("*").Id(goTypeName)).
				Dot(implWrappers[method]).Call(jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
				jen.Id("Positional"): jen.Id(argsName).Dot("Positional").Index(jen.Lit(1), jen.Empty()),
			}))
			methods = append(methods, jen.Values(jen.Dict{
				jen.Id("Name"):  jen.Lit(method.Name),
				jen.Id("Arity"): jen.Lit(len(method.Parameters)),
				jen.Id("Fn"): jen.Op("&").Qual(pathObject, "Function").Values(jen.Dict{
					jen.Id("Name"): jen.Lit(method.Name),
					jen.Id("Fn"): jen.Func().Params(jen.Id(argsName).Qual(pathObject, "CallArgs")).
						Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(jen.Return(call)),
				}),
			}))
		}
		registration = append(registration,
			jen.Id(typeVar).Dot("Implement").Call(trait, jen.Index().Qual(pathObject, "ImplMethod").Values(methods...)),
		)
	}
	sealErr := ctx.localName("err")
	registration = append(registration,
		jen.If(jen.Id(sealErr).Op(":=").Id(typeVar).Dot("Seal").Call(), jen.Id(sealErr).Op("!=").Nil()).Block(onError(sealErr)),
	)

	callArgsName := ctx.localName("callArgs")
	boundName := ctx.localName("bound")
	errVar := ctx.localName("err")

	fieldNames := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNames = append(fieldNames, jen.Lit(field.Name))
	}

	// Field defaults reach BindArguments as lazy closures, so a default is
	// evaluated only for the call that omits its field, exactly like a
	// function parameter default.
	defaultsArg := jen.Nil()
	hasDefault := false
	for _, field := range typeDef.Fields {
		if field.HasDefault() {
			hasDefault = true
			break
		}
	}
	if hasDefault {
		entries := make([]jen.Code, 0, len(typeDef.Fields))
		for _, field := range typeDef.Fields {
			if !field.HasDefault() {
				entries = append(entries, jen.Nil())
				continue
			}
			defaultPre, defaultValue, err := ctx.transpileExpression(field.DefaultValue, func(errVar string) jen.Code {
				return jen.Return(jen.Nil(), jen.Id(errVar))
			})
			if err != nil {
				return nil, err
			}
			entries = append(entries, jen.Func().Params().Parens(jen.List(
				jen.Qual(pathObject, "Object"), jen.Id("error"),
			)).Block(append(defaultPre, jen.Return(defaultValue, jen.Nil()))...))
		}
		defaultsArg = jen.Index().Qual(pathObject, "ParamDefault").Values(entries...)
	}

	fastValues := make([]jen.Code, 0, len(typeDef.Fields))
	slowValues := make([]jen.Code, 0, len(typeDef.Fields))
	for index, field := range typeDef.Fields {
		fastValues = append(fastValues,
			jen.Id(fieldGoName(field.Name)).Op(":").Id(callArgsName).Dot("Positional").Index(jen.Lit(index)))
		slowValues = append(slowValues,
			jen.Id(fieldGoName(field.Name)).Op(":").Id(boundName).Index(jen.Lit(index)))
	}

	// A construction that supplies every field positionally needs no binding
	// at all: the field order is known here. Every other shape goes through
	// BindArguments, so its diagnostics stay identical.
	constructorBody := []jen.Code{
		jen.If(
			jen.Len(jen.Id(callArgsName).Dot("Keyword")).Op("==").Lit(0).
				Op("&&").
				Len(jen.Id(callArgsName).Dot("Positional")).Op("==").Lit(len(typeDef.Fields)),
		).Block(
			jen.Return(jen.Op("&").Id(goTypeName).Values(fastValues...), jen.Nil()),
		),
		jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(typeDef.Name),
			jen.Index().String().Values(fieldNames...),
			defaultsArg,
			jen.Lit(""),
			jen.Lit(""),
			jen.Id(callArgsName),
		),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(
			jen.Return(jen.Nil(), jen.Id(errVar)),
		),
		jen.Id("_").Op("=").Id(boundName),
		jen.Return(jen.Op("&").Id(goTypeName).Values(slowValues...), jen.Nil()),
	}

	constructorClosure := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(
		jen.List(jen.Qual(pathObject, "Object"), jen.Error()),
	).Block(constructorBody...)

	ctx.topDecls = append(ctx.topDecls,
		jen.Var().Id(ctorVarName).Qual(pathObject, "Object"),
	)

	// The direct constructor lowered call sites use (see collectDirectFns):
	// one Go parameter per field, returning the struct literal.
	if info, ok := ctx.directCtors[typeDef.Name]; ok {
		params := make([]jen.Code, 0, len(typeDef.Fields))
		values := make([]jen.Code, 0, len(typeDef.Fields))
		for _, field := range typeDef.Fields {
			params = append(params, jen.Id(field.Name).Qual(pathObject, "Object"))
			values = append(values, jen.Id(fieldGoName(field.Name)).Op(":").Id(field.Name))
		}
		ctx.topDecls = append(ctx.topDecls,
			jen.Func().Id(info.goName).Params(params...).Parens(jen.List(
				jen.Qual(pathObject, "Object"), jen.Error(),
			)).Block(jen.Return(jen.Op("&").Id(goTypeName).Values(values...), jen.Nil())),
		)
	}

	constructor := jen.Id(ctorVarName).Op("=").Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(typeDef.Name),
		jen.Id("Fn").Op(":").Add(constructorClosure),
	)

	return append(registration, constructor), nil
}

// receiverGoName is the receiver of every generated method. It has the shape
// of a transpiler scratch name, which the checker reserves, so no parameter
// can shadow it.
const receiverGoName = "_recv_0"

// fieldGoName is the Go struct field holding a Goblin field. The prefix keeps
// fields apart from the generated methods: those start uppercase, and impl
// and scratch names start with an underscore.
func fieldGoName(name string) string {
	return "f_" + name
}

// builtinTrait reports the built-in trait a reference statically names: a bare
// built-in trait name that no module-level declaration or import shadows at
// this point. It returns nil for anything else.
func (ctx *transpileContext) builtinTrait(ref *ast.TraitRef) *object.Trait {
	if ref.Module != "" || ctx.isUserName(ref.Name) {
		return nil
	}
	if _, bound := ctx.moduleBinding(ref.Name); bound {
		return nil
	}
	return object.BuiltinTraits[ref.Name]
}

// transpileTraitRef evaluates the trait an impl block or a dependency list
// names to a *object.Trait expression.
// site says where the reference appears, for the runtime messages.
func (ctx *transpileContext) transpileTraitRef(ref *ast.TraitRef, site string, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if trait := ctx.builtinTrait(ref); trait != nil {
		return nil, jen.Qual(pathObject, trait.Name+"Trait"), nil
	}
	name := ref.Name
	if ref.Module != "" {
		name = ref.Module
	}
	ident, err := ast.NewIdentifier(&token.Token{Lit: []byte(name), Pos: ref.Pos})
	if err != nil {
		return nil, nil, err
	}
	pre, value, err := ctx.transpileExpression(ident.(*ast.Identifier), onError)
	if err != nil {
		return nil, nil, err
	}
	traitVar := ctx.localName("trait")
	errVar := ctx.localName("err")
	var call *jen.Statement
	if ref.Module != "" {
		call = jen.Qual(pathObject, "ModuleTrait").Call(value, jen.Lit(ref.Name), jen.Lit(site), jen.Lit(ref.String()))
	} else {
		call = jen.Qual(pathObject, "AsTrait").Call(value, jen.Lit(site), jen.Lit(ref.String()))
	}
	pre = append(pre,
		jen.List(jen.Id(traitVar), jen.Id(errVar)).Op(":=").Add(call),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return pre, jen.Id(traitVar), nil
}

// transpileTraitDefine emits the assignment of a trait declaration's trait
// object. Default methods compile to function values taking self first, like
// the interpreter's closures.
func (ctx *transpileContext) transpileTraitDefine(def *ast.TraitDefine, onError errHandler) ([]jen.Code, error) {
	var pre []jen.Code
	deps := make([]jen.Code, 0, len(def.Deps))
	for _, ref := range def.Deps {
		depPre, dep, err := ctx.transpileTraitRef(ref, "trait "+def.Name, onError)
		if err != nil {
			return nil, err
		}
		pre = append(pre, depPre...)
		deps = append(deps, dep)
	}
	methods := make([]jen.Code, 0, len(def.Methods))
	for _, method := range def.Methods {
		fields := jen.Dict{
			jen.Id("Name"):  jen.Lit(method.Name),
			jen.Id("Arity"): jen.Lit(len(method.Parameters)),
		}
		if method.Body == nil {
			fields[jen.Id("Required")] = jen.True()
		} else {
			fn, err := ctx.buildFunctionValue(def.Name+"."+method.Name, method.Position(), method.Parameters, method.Body)
			if err != nil {
				return nil, err
			}
			fields[jen.Id("Default")] = fn
		}
		methods = append(methods, jen.Values(fields))
	}
	return append(pre, jen.Id(def.Name).Op("=").Qual(pathObject, "NewTrait").Call(
		jen.Lit(def.Name),
		jen.Index().Op("*").Qual(pathObject, "Trait").Values(deps...),
		jen.Index().Qual(pathObject, "TraitMethod").Values(methods...),
	)), nil
}
