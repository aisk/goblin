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

func (g *generator) transpileComparisonOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	preStmts, result, err := g.transpileComparisonBool(operation, onError)
	if err != nil {
		return nil, nil, err
	}
	tmpVar := g.localName("tmp")
	preStmts = append(preStmts,
		jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Bool").Call(result),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileComparisonBool evaluates a comparison of boxed operands down to a
// Go bool expression, which the operator boxes and a condition uses as is.
func (g *generator) transpileComparisonBool(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := g.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, nativeRHS, err := g.transpileNativeIntOperand(operation.RHS, onError)
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
	resultVar := g.localName("cmp")
	errVar := g.localName("err")
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
func (g *generator) transpileNativeIntOperand(operand ast.Expression, onError errHandler) (pre []jen.Code, code *jen.Statement, variant func(string) string, err error) {
	if g.nativeTypeOf(operand) == tyInt {
		pre, code, err = g.emitNative(operand, onError)
		return pre, code, func(op string) string { return op + "Int" }, err
	}
	pre, code, err = g.transpileExpression(operand, onError)
	return pre, code, func(op string) string { return op }, err
}

// transpileCondition evaluates an if or while condition to a Go bool: a
// native bool expression as it is, a comparison of boxed operands through the
// comparison's own bool result, anything else through ToBool.
func (g *generator) transpileCondition(cond ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if g.nativeTypeOf(cond) == tyBool {
		return g.emitNative(cond, onError)
	}
	if op, ok := cond.(*ast.BinaryOperation); ok && isComparisonOperator(op.Operator) {
		return g.transpileComparisonBool(op, onError)
	}
	pre, value, err := g.transpileExpression(cond, onError)
	if err != nil {
		return nil, nil, err
	}
	condVar := g.localName("cond")
	errVar := g.localName("err")
	pre = append(pre,
		jen.List(jen.Id(condVar), jen.Id(errVar)).Op(":=").Add(value).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return pre, jen.Id(condVar), nil
}

func (g *generator) transpileBinaryOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if isComparisonOperator(operation.Operator) {
		return g.transpileComparisonOperation(operation, onError)
	}
	if operation.Operator == ast.And || operation.Operator == ast.Or {
		return g.transpileLogicalOperation(operation, onError)
	}

	lhsPre, lhs, err := g.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, nativeRHS, err := g.transpileNativeIntOperand(operation.RHS, onError)
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

	tmpVar := g.localName("tmp")
	errVar := g.localName("err")
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
func (g *generator) transpileLogicalOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := g.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	// Always transpile the RHS so diagnostics surface at compile time, but
	// emit its statements inside a guarded block so it only runs at runtime
	// when the LHS leaves the result open.
	rhsPre, rhs, err := g.transpileExpression(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	lhsTruthyVar := g.localName("cond")
	errVar := g.localName("err")
	tmpVar := g.localName("tmp")

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

func (g *generator) transpileUnaryOperation(operation *ast.UnaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	operandPre, operand, err := g.transpileExpression(operation.Operand, onError)
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

	tmpVar := g.localName("tmp")
	errVar := g.localName("err")
	preStmts := append(operandPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}
