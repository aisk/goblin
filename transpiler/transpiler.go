package transpiler

import (
	"errors"
	"fmt"
	"io"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
	"github.com/dave/jennifer/jen"
)

const (
	pathBase    = "github.com/aisk/goblin"
	pathObject  = pathBase + "/object"
	pathBuiltin = pathBase + "/builtin"
)

var ErrNotImplemented = errors.New("not implemented")

var localNameCounter = 0

func localName(prefix string) string {
	name := fmt.Sprintf("_%s_%d", prefix, localNameCounter)
	localNameCounter++
	return name
}

func Transpile(mod *ast.Module, output io.Writer) error {
	f := jen.NewFile(mod.Name)
	stmts, err := transpileStatements(mod.Body)
	if err != nil {
		return err
	}
	f.Func().Id("main").Params().Block(stmts...)
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
	case object.String:
		s := jen.Qual(pathObject, "String").Call(jen.Lit(string(v)))
		return s, nil
	}
	return nil, ErrNotImplemented
}

func transpileExpression(expr ast.Expression) (*jen.Statement, error) {
	switch v := expr.(type) {
	case *ast.Literal:
		return transpileObject(v.Value)
	case *ast.Symbol:
		return jen.Id(v.Name), nil
	case *ast.FunctionCall:
		call, err := transpileFunctionCall(v)
		if err != nil {
			return nil, err
		}
		// hack to ignore the err
		// TODO: implemt the error mechanism
		return jen.Func().Params().Id("object.Object").Block(
			jen.List(jen.Id("result"), jen.Id("_")).Op(":=").Add(call),
			jen.Return(jen.Id("result")),
		).Call(), nil
	}
	return nil, ErrNotImplemented
}

func transpileExpressions(exprs []ast.Expression) ([]jen.Code, error) {
	result := []jen.Code{}
	for _, expr := range exprs {
		r, err := transpileExpression(expr)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func resolveFunctionName(name string) *jen.Statement {
	switch name {
	case "print":
		return jen.Qual(pathBuiltin, "Print")
	}
	return jen.Id(name)
}

func transpileDeclare(decl *ast.Declare) (*jen.Statement, error) {
	value, err := transpileExpression(decl.Value)
	if err != nil {
		return nil, err
	}
	result := jen.Var().Id(decl.Name).Id("object.Object").Op("=").Add(value)
	result.Op(";").Id("_").Op("=").Id(decl.Name)
	return result, nil
}

func transpileAssign(decl *ast.Assign) (*jen.Statement, error) {
	value, err := transpileExpression(decl.Value)
	if err != nil {
		return nil, err
	}
	result := jen.Id(decl.Target).Op("=").Add(value)
	result.Op(";").Id("_").Op("=").Id(decl.Target)
	return result, nil
}

func transpileIfElse(ifelse *ast.IfElse) (*jen.Statement, error) {
	cond, err := transpileExpression(ifelse.Condition)
	if err != nil {
		return nil, err
	}
	body, err := transpileStatements(ifelse.IfBody)
	if err != nil {
		return nil, err
	}
	elseBody, err := transpileStatements(ifelse.ElseBody)
	return jen.If(cond.Dot("Bool").Call()).Block(body...).Else().Block(elseBody...), nil
}

func transpileWhile(while_ *ast.While) (*jen.Statement, error) {
	cond, err := transpileExpression(while_.Condition)
	if err != nil {
		return nil, err
	}
	body, err := transpileStatements(while_.Body)
	if err != nil {
		return nil, err
	}
	return jen.For(cond.Dot("Bool").Call()).Block(body...), nil
}

func transpileBreak(break_ *ast.Break) (*jen.Statement, error) {
	return jen.Break(), nil
}

func transpileFunctionCall(call *ast.FunctionCall) (*jen.Statement, error) {
	l, err := transpileExpressions(call.Args)
	if err != nil {
		return nil, err
	}
	args := jen.Qual(pathObject, "Args").Values(l...)
	kwargs := jen.Nil()
	return resolveFunctionName(call.Name).Call(args, kwargs), nil
}

func transpileFunctionDefine(fn *ast.FunctionDefine) (*jen.Statement, error) {
	argsName := localName("args")
	kwargsName := localName("kwargs")

	argsDefine := []jen.Code{}
	for i, param := range fn.Parameters {
		def := jen.Var().Id(param).Op("=").Id(argsName).Index(jen.Lit(i))
		def.Op(";").Id("_").Op("=").Id(param)
		argsDefine = append(argsDefine, def)
	}

	body, err := transpileStatements(fn.Body)
	if err != nil {
		return nil, err
	}

	body = append(argsDefine, body...)

	result := jen.Id(fn.Name).Op(":=").Func().Params(
		jen.Id(argsName).Qual(pathObject, "Args"), jen.Id(kwargsName).Qual(pathObject, "KwArgs"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Id("error")),
	).Block(body...)

	result.Op(";").Id("_").Op("=").Id(fn.Name)

	return result, nil
}

func transpileReturn(return_ *ast.Return) (*jen.Statement, error) {
	r, err := transpileExpression(return_.Value)
	if err != nil {
		return nil, err
	}
	// TODO: implement the error mechanism
	return jen.Return(jen.List(r, jen.Nil())), nil
}

func transpileStatement(stmt ast.Statement) (*jen.Statement, error) {
	switch v := stmt.(type) {
	case *ast.Declare:
		return transpileDeclare(v)
	case *ast.Assign:
		return transpileAssign(v)
	case *ast.FunctionCall:
		return transpileFunctionCall(v)
	case *ast.FunctionDefine:
		return transpileFunctionDefine(v)
	case *ast.IfElse:
		return transpileIfElse(v)
	case *ast.While:
		return transpileWhile(v)
	case *ast.Break:
		return transpileBreak(v)
	case *ast.Return:
		return transpileReturn(v)
	}
	return nil, ErrNotImplemented
}

func transpileStatements(stmts []ast.Statement) ([]jen.Code, error) {
	result := []jen.Code{}
	for _, stmt := range stmts {
		s, err := transpileStatement(stmt)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}
