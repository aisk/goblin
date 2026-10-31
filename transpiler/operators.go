package transpiler

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/dave/jennifer/jen"
)

func isComparisonOperator(op string) bool {
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		return true
	}
	return false
}

func (ctx *transpileContext) transpileComparisonOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	preStmts, result, err := ctx.transpileComparisonBool(operation, onError)
	if err != nil {
		return nil, nil, err
	}
	tmpVar := ctx.localName("tmp")
	preStmts = append(preStmts,
		jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Bool").Call(result),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileComparisonBool evaluates a comparison of boxed operands down to a
// Go bool expression, which the operator boxes and a condition uses as is.
func (ctx *transpileContext) transpileComparisonBool(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, nativeRHS, err := ctx.transpileNativeIntOperand(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}
	preStmts := append(lhsPre, rhsPre...)

	// Equality is total for the built-in types, but a user type's eq may fail;
	// like ordering, that error propagates.
	entry := map[string]string{
		ast.Equal: "Equals", ast.NotEqual: "NotEquals",
		ast.LessThan: "Less", ast.LessOrEqual: "LessEqual",
		ast.GreaterThan: "Greater", ast.GreaterOrEqual: "GreaterEqual",
	}[operation.Operator]
	resultVar := ctx.localName("cmp")
	errVar := ctx.localName("err")
	preStmts = append(preStmts,
		jen.List(jen.Id(resultVar), jen.Id(errVar)).Op(":=").Qual(pathObject, nativeRHS(entry)).Call(lhs, rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(resultVar), nil
}

// transpileNativeIntOperand renders the right operand of a binary operator
// whose left operand is boxed. A statically integer right operand stays a
// native int64 and the returned function maps an operator entry point to its
// native-right variant (Add to AddInt, and so on); otherwise the operand is
// boxed and the mapping is the identity.
func (ctx *transpileContext) transpileNativeIntOperand(operand ast.Expression, onError errHandler) (pre []jen.Code, code *jen.Statement, variant func(string) string, err error) {
	if ctx.nativeTypeOf(operand) == tyInt {
		pre, code, err = ctx.emitNative(operand, onError)
		return pre, code, func(op string) string { return op + "Int" }, err
	}
	pre, code, err = ctx.transpileExpression(operand, onError)
	return pre, code, func(op string) string { return op }, err
}

// transpileCondition evaluates an if or while condition to a Go bool: a
// native bool expression as it is, a comparison of boxed operands through the
// comparison's own bool result, anything else through ToBool.
func (ctx *transpileContext) transpileCondition(cond ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if ctx.nativeTypeOf(cond) == tyBool {
		return ctx.emitNative(cond, onError)
	}
	if op, ok := cond.(*ast.BinaryOperation); ok && isComparisonOperator(op.Operator) {
		return ctx.transpileComparisonBool(op, onError)
	}
	pre, value, err := ctx.transpileExpression(cond, onError)
	if err != nil {
		return nil, nil, err
	}
	condVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	pre = append(pre,
		jen.List(jen.Id(condVar), jen.Id(errVar)).Op(":=").Add(value).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return pre, jen.Id(condVar), nil
}

func (ctx *transpileContext) transpileBinaryOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if isComparisonOperator(operation.Operator) {
		return ctx.transpileComparisonOperation(operation, onError)
	}
	if operation.Operator == ast.And || operation.Operator == ast.Or {
		return ctx.transpileLogicalOperation(operation, onError)
	}

	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, nativeRHS, err := ctx.transpileNativeIntOperand(operation.RHS, onError)
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
	case "%":
		methodName = "Modulo"
	default:
		return nil, nil, fmt.Errorf("unsupported binary operator: %s", operation.Operator)
	}

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(lhsPre, rhsPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Qual(pathObject, nativeRHS(methodName)).Call(lhs, rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileLogicalOperation lowers && and || with short-circuit evaluation: the
// RHS is only evaluated (and may therefore only error or produce side effects)
// when the LHS does not already fix the result. The value of the expression is
// the operand that decided it rather than a coerced Bool, so `x || fallback`
// yields x when x is truthy.
func (ctx *transpileContext) transpileLogicalOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	// Always transpile the RHS so diagnostics surface at compile time, but
	// emit its statements inside a guarded block so it only runs at runtime
	// when the LHS leaves the result open.
	rhsPre, rhs, err := ctx.transpileExpression(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	lhsTruthyVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	tmpVar := ctx.localName("tmp")

	rhsBlock := append([]jen.Code{}, rhsPre...)
	rhsBlock = append(rhsBlock, jen.Id(tmpVar).Op("=").Add(rhs))

	guard := jen.Id(lhsTruthyVar)
	if operation.Operator == ast.Or {
		guard = jen.Op("!").Id(lhsTruthyVar)
	}

	preStmts := append([]jen.Code{}, lhsPre...)
	preStmts = append(preStmts,
		jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Add(lhs),
		jen.List(jen.Id(lhsTruthyVar), jen.Id(errVar)).Op(":=").Id(tmpVar).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.If(guard).Block(rhsBlock...),
	)

	return preStmts, jen.Id(tmpVar), nil
}

func (ctx *transpileContext) transpileUnaryOperation(operation *ast.UnaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	operandPre, operand, err := ctx.transpileExpression(operation.Operand, onError)
	if err != nil {
		return nil, nil, err
	}

	var call *jen.Statement
	switch operation.Operator {
	case "!":
		call = jen.Qual(pathObject, "Not").Call(operand)
	case "+":
		call = jen.Qual(pathObject, "Positive").Call(operand)
	case "-":
		call = jen.Qual(pathObject, "Negate").Call(operand)
	default:
		return nil, nil, fmt.Errorf("unsupported unary operator: %s", operation.Operator)
	}

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(operandPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}
