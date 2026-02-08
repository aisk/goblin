package transpiler

import (
	"fmt"
	"io"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
	"github.com/dave/jennifer/jen"
)

const (
	pathBase      = "github.com/aisk/goblin"
	pathObject    = pathBase + "/object"
	pathExtension = pathBase + "/extension"
)

var moduleImports = map[string]string{
	"os": "os_module",
}

var localNameCounter = 0

func localName(prefix string) string {
	name := fmt.Sprintf("_%s_%d", prefix, localNameCounter)
	localNameCounter++
	return name
}

// errHandler generates the error-handling code for a given error variable name.
// In Execute(), it generates: return errVar
// In user-defined functions, it generates: return nil, errVar
type errHandler func(errVar string) jen.Code

func Transpile(mod *ast.Module, output io.Writer) error {
	localNameCounter = 0
	f := jen.NewFile(mod.Name)

	exportsVar := localName("exports")

	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Id(errVar))
	}

	stmts, err := transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return err
	}

	body := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
		jen.Id("os_module").Op(":=").Qual(pathExtension, "OsModule"),
		jen.Id(exportsVar).Index(jen.Lit("os")).Op("=").Id("os_module"),
	}
	body = append(body, stmts...)
	body = append(body,
		jen.Return(
			jen.Op("&").Qual(pathObject, "Module").Values(
				jen.Id("Members").Op(":").Id(exportsVar),
			),
			jen.Nil(),
		),
	)

	f.Func().Id("Execute").Params().Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(body...)
	f.Func().Id("main").Params().Block(
		jen.List(jen.Id("_"), jen.Id("err")).Op(":=").Id("Execute").Call(),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Panic(jen.Id("err")),
		),
	)
	return f.Render(output)
}

func transpileObject(obj object.Object) (*jen.Statement, error) {
	switch v := obj.(type) {
	case object.Bool:
		if v.Bool() {
			return jen.Qual(pathObject, "True"), nil
		}
		return jen.Qual(pathObject, "False"), nil
	case object.Unit:
		return jen.Qual(pathObject, "Nil"), nil
	case object.Integer:
		i := jen.Qual(pathObject, "Integer").Call(jen.Lit(int64(v)))
		return i, nil
	case object.Float:
		f := jen.Qual(pathObject, "Float").Call(jen.Lit(float64(v)))
		return f, nil
	case object.String:
		s := jen.Qual(pathObject, "String").Call(jen.Lit(string(v)))
		return s, nil
	}
	return nil, object.NotImplementedError
}

func transpileListLiteral(list *ast.ListLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	preStmts, elements, err := transpileExpressions(list.Elements, onError)
	if err != nil {
		return nil, nil, err
	}

	return preStmts, jen.Op("&").Qual(pathObject, "List").Values(
		jen.Id("Elements").Op(":").Index().Qual(pathObject, "Object").Values(elements...),
	), nil
}

func transpileIndexExpression(expr *ast.IndexExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	objPre, obj, err := transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}
	idxPre, idx, err := transpileExpression(expr.Index, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := localName("tmp")
	errVar := localName("err")
	preStmts := append(objPre, idxPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(obj).Dot("Index").Call(idx),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func transpileDictLiteral(dict *ast.DictLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	var preStmts []jen.Code
	var entries []jen.Code

	for _, elem := range dict.Elements {
		keyPre, key, err := transpileExpression(elem.Key, onError)
		if err != nil {
			return nil, nil, err
		}
		valuePre, value, err := transpileExpression(elem.Value, onError)
		if err != nil {
			return nil, nil, err
		}
		preStmts = append(preStmts, keyPre...)
		preStmts = append(preStmts, valuePre...)
		entries = append(entries, jen.Values(jen.Id("Key").Op(":").Add(key), jen.Id("Value").Op(":").Add(value)))
	}

	dictVar := localName("dict")
	preStmts = append(preStmts,
		jen.Id(dictVar).Op(":=").Op("&").Qual(pathObject, "Dict").Values(
			jen.Id("Entries").Op(":").Index().Qual(pathObject, "DictEntry").Values(entries...),
			jen.Id("KeyIndex").Op(":").Make(jen.Map(jen.String()).Int()),
		),
	)

	for i := range dict.Elements {
		preStmts = append(preStmts,
			jen.Id(dictVar).Dot("KeyIndex").Index(jen.Id(dictVar).Dot("Entries").Index(jen.Lit(i)).Dot("Key").Dot("String").Call()).Op("=").Lit(i),
		)
	}

	return preStmts, jen.Id(dictVar), nil
}

func transpileMemberExpression(expr *ast.MemberExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	objPre, obj, err := transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := localName("attr")
	errVar := localName("err")
	preStmts := append(objPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(obj).Dot("GetAttr").Call(jen.Lit(expr.Property)),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func transpileExpression(expr ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	switch v := expr.(type) {
	case *ast.Literal:
		obj, err := transpileObject(v.Value)
		if err != nil {
			return nil, nil, err
		}
		return nil, obj, nil
	case *ast.Identifier:
		if moduleVar, ok := moduleImports[v.Name]; ok {
			return nil, jen.Id(moduleVar), nil
		}
		return nil, jen.Id(v.Name), nil
	case *ast.FunctionCall:
		argPreStmts, call, err := transpileFunctionCall(v, onError)
		if err != nil {
			return nil, nil, err
		}
		tmpVar := localName("tmp")
		errVar := localName("err")
		preStmts := append(argPreStmts,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Id(tmpVar), nil
	case *ast.CallExpression:
		argPreStmts, call, err := transpileCallExpression(v, onError)
		if err != nil {
			return nil, nil, err
		}
		tmpVar := localName("tmp")
		errVar := localName("err")
		preStmts := append(argPreStmts,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Id(tmpVar), nil
	case *ast.BinaryOperation:
		return transpileBinaryOperation(v, onError)
	case *ast.UnaryOperation:
		return transpileUnaryOperation(v, onError)
	case *ast.ListLiteral:
		return transpileListLiteral(v, onError)
	case *ast.DictLiteral:
		return transpileDictLiteral(v, onError)
	case *ast.IndexExpression:
		return transpileIndexExpression(v, onError)
	case *ast.MemberExpression:
		return transpileMemberExpression(v, onError)
	}
	return nil, nil, object.NotImplementedError
}

func transpileExpressions(exprs []ast.Expression, onError errHandler) ([]jen.Code, []jen.Code, error) {
	var allPreStmts []jen.Code
	var results []jen.Code
	for _, expr := range exprs {
		pre, r, err := transpileExpression(expr, onError)
		if err != nil {
			return nil, nil, err
		}
		allPreStmts = append(allPreStmts, pre...)
		results = append(results, r)
	}
	return allPreStmts, results, nil
}

func isBuiltinFunction(name string) bool {
	_, ok := extension.BuiltinsModule.Members[name]
	return ok
}

func transpileDeclare(decl *ast.Declare, onError errHandler) ([]jen.Code, error) {
	preStmts, value, err := transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	declStmt := jen.Var().Id(decl.Name).Qual(pathObject, "Object").Op("=").Add(value)
	declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
	return append(preStmts, declStmt), nil
}

func transpileAssign(decl *ast.Assign, onError errHandler) ([]jen.Code, error) {
	preStmts, value, err := transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	assignStmt := jen.Id(decl.Target).Op("=").Add(value)
	assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
	return append(preStmts, assignStmt), nil
}

func transpileIfElse(ifelse *ast.IfElse, onError errHandler) ([]jen.Code, error) {
	condPreStmts, cond, err := transpileExpression(ifelse.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := transpileStatements(ifelse.IfBody, onError, "")
	if err != nil {
		return nil, err
	}
	elseBody, err := transpileStatements(ifelse.ElseBody, onError, "")
	if err != nil {
		return nil, err
	}
	ifStmt := jen.If(cond.Dot("Bool").Call()).Block(body...).Else().Block(elseBody...)
	return append(condPreStmts, ifStmt), nil
}

func transpileWhile(while_ *ast.While, onError errHandler) ([]jen.Code, error) {
	condPreStmts, cond, err := transpileExpression(while_.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := transpileStatements(while_.Body, onError, "")
	if err != nil {
		return nil, err
	}

	if len(condPreStmts) > 0 {
		// Complex condition with preStmts: convert to for { preStmts; if !cond { break }; body }
		loopBody := append(condPreStmts,
			jen.If(jen.Op("!").Add(cond).Dot("Bool").Call()).Block(jen.Break()),
		)
		loopBody = append(loopBody, body...)
		return []jen.Code{jen.For().Block(loopBody...)}, nil
	}

	return []jen.Code{jen.For(cond.Dot("Bool").Call()).Block(body...)}, nil
}

func transpileBreak(break_ *ast.Break) ([]jen.Code, error) {
	return []jen.Code{jen.Break()}, nil
}

func transpileFor(for_ *ast.For, onError errHandler) ([]jen.Code, error) {
	iterPreStmts, iterator, err := transpileExpression(for_.Iterator, onError)
	if err != nil {
		return nil, err
	}
	body, err := transpileStatements(for_.Body, onError, "")
	if err != nil {
		return nil, err
	}

	iterVar := localName("iter")
	elementsVar := localName("elements")
	errVar := localName("err")

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

func transpileFunctionCall(call *ast.FunctionCall, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	argPreStmts, l, err := transpileExpressions(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}
	args := jen.Qual(pathObject, "Args").Values(l...)
	kwargs := jen.Nil()

	var callee *jen.Statement
	if isBuiltinFunction(call.Name) {
		callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(call.Name))
	} else {
		callee = jen.Id(call.Name)
	}

	return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args, kwargs), nil
}

func transpileCallExpression(call *ast.CallExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	argPreStmts, l, err := transpileExpressions(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}
	args := jen.Qual(pathObject, "Args").Values(l...)
	kwargs := jen.Nil()

	if ident, ok := call.Callee.(*ast.Identifier); ok {
		var callee *jen.Statement
		if isBuiltinFunction(ident.Name) {
			callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(ident.Name))
		} else {
			callee = jen.Id(ident.Name)
		}
		return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args, kwargs), nil
	}

	if member, ok := call.Callee.(*ast.MemberExpression); ok {
		objPre, obj, err := transpileExpression(member.Object, onError)
		if err != nil {
			return nil, nil, err
		}
		attrVar := localName("attr")
		errVar := localName("err")
		preStmts := append(objPre, argPreStmts...)
		preStmts = append(preStmts,
			jen.List(jen.Id(attrVar), jen.Id(errVar)).Op(":=").Add(obj).Dot("GetAttr").Call(jen.Lit(member.Property)),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Qual(pathObject, "Call").Call(jen.Id(attrVar), args, kwargs), nil
	}

	calleePre, callee, err := transpileExpression(call.Callee, onError)
	if err != nil {
		return nil, nil, err
	}
	preStmts := append(calleePre, argPreStmts...)
	return preStmts, jen.Qual(pathObject, "Call").Call(callee, args, kwargs), nil
}

func transpileFunctionDefine(fn *ast.FunctionDefine, onError errHandler) ([]jen.Code, error) {
	argsName := localName("args")
	kwargsName := localName("kwargs")

	argsDefine := []jen.Code{}
	for i, param := range fn.Parameters {
		def := jen.Var().Id(param).Op("=").Id(argsName).Index(jen.Lit(i))
		def.Op(";").Id("_").Op("=").Id(param)
		argsDefine = append(argsDefine, def)
	}

	fnOnError := func(errVar string) jen.Code {
		return jen.Return(jen.List(jen.Nil(), jen.Id(errVar)))
	}

	body, err := transpileStatements(fn.Body, fnOnError, "")
	if err != nil {
		return nil, err
	}

	body = append(argsDefine, body...)

	closure := jen.Func().Params(
		jen.Id(argsName).Qual(pathObject, "Args"), jen.Id(kwargsName).Qual(pathObject, "KwArgs"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Id("error")),
	).Block(body...)

	result := jen.Var().Id(fn.Name).Qual(pathObject, "Object").Op("=").Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(fn.Name),
		jen.Id("Fn").Op(":").Add(closure),
	)

	result.Op(";").Id("_").Op("=").Id(fn.Name)

	return []jen.Code{result}, nil
}

func transpileReturn(return_ *ast.Return, onError errHandler) ([]jen.Code, error) {
	preStmts, r, err := transpileExpression(return_.Value, onError)
	if err != nil {
		return nil, err
	}
	return append(preStmts, jen.Return(jen.List(r, jen.Nil()))), nil
}

func isComparisonOperator(op string) bool {
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		return true
	}
	return false
}

func transpileComparisonOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, err := transpileExpression(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	cmpVar := localName("cmp")
	errVar := localName("err")
	tmpVar := localName("tmp")

	preStmts := append(lhsPre, rhsPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(cmpVar), jen.Id(errVar)).Op(":=").Add(lhs).Dot("Compare").Call(rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Bool").Call(
			jen.Id(cmpVar).Op(operation.Operator).Lit(0),
		),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func transpileBinaryOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if isComparisonOperator(operation.Operator) {
		return transpileComparisonOperation(operation, onError)
	}

	lhsPre, lhs, err := transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, err := transpileExpression(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	var methodName string
	switch operation.Operator {
	case "+":
		methodName = "Add"
	case "-":
		methodName = "Minus"
	case "*":
		methodName = "Multiply"
	case "/":
		methodName = "Divide"
	case "&&":
		methodName = "And"
	case "||":
		methodName = "Or"
	default:
		return nil, nil, fmt.Errorf("unsupported binary operator: %s", operation.Operator)
	}

	tmpVar := localName("tmp")
	errVar := localName("err")
	preStmts := append(lhsPre, rhsPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(lhs).Dot(methodName).Call(rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func transpileUnaryOperation(operation *ast.UnaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	operandPre, operand, err := transpileExpression(operation.Operand, onError)
	if err != nil {
		return nil, nil, err
	}

	var methodName string
	switch operation.Operator {
	case "!":
		methodName = "Not"
	default:
		return nil, nil, fmt.Errorf("unsupported unary operator: %s", operation.Operator)
	}

	tmpVar := localName("tmp")
	errVar := localName("err")
	preStmts := append(operandPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(operand).Dot(methodName).Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func transpileExport(export *ast.Export, exportsVar string) ([]jen.Code, error) {
	return []jen.Code{
		jen.Id(exportsVar).Index(jen.Lit(export.Name)).Op("=").Id(export.Name),
	}, nil
}

func transpileStatement(stmt ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	switch v := stmt.(type) {
	case *ast.Declare:
		return transpileDeclare(v, onError)
	case *ast.Assign:
		return transpileAssign(v, onError)
	case *ast.FunctionCall:
		argPreStmts, call, err := transpileFunctionCall(v, onError)
		if err != nil {
			return nil, err
		}
		errVar := localName("err")
		stmts := append(argPreStmts,
			jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return stmts, nil
	case *ast.CallExpression:
		argPreStmts, call, err := transpileCallExpression(v, onError)
		if err != nil {
			return nil, err
		}
		errVar := localName("err")
		stmts := append(argPreStmts,
			jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return stmts, nil
	case *ast.FunctionDefine:
		return transpileFunctionDefine(v, onError)
	case *ast.IfElse:
		return transpileIfElse(v, onError)
	case *ast.While:
		return transpileWhile(v, onError)
	case *ast.For:
		return transpileFor(v, onError)
	case *ast.Break:
		return transpileBreak(v)
	case *ast.Return:
		return transpileReturn(v, onError)
	case *ast.Export:
		return transpileExport(v, exportsVar)
	case *ast.BinaryOperation:
		pre, _, err := transpileBinaryOperation(v, onError)
		return pre, err
	case *ast.UnaryOperation:
		pre, _, err := transpileUnaryOperation(v, onError)
		return pre, err
	case *ast.MemberExpression:
		pre, _, err := transpileMemberExpression(v, onError)
		return pre, err
	}
	return nil, object.NotImplementedError
}

func transpileStatements(stmts []ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	var result []jen.Code
	for _, stmt := range stmts {
		codes, err := transpileStatement(stmt, onError, exportsVar)
		if err != nil {
			return nil, err
		}
		result = append(result, codes...)
	}
	return result, nil
}
