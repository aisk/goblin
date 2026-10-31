package transpiler

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
	"github.com/dave/jennifer/jen"
)

// zeroOf is the Go zero value of a native type, or nil for a boxed result;
// error paths return it alongside the error.
func zeroOf(t staticType) *jen.Statement {
	switch t {
	case tyInt, tyFloat:
		return jen.Lit(0)
	case tyBool:
		return jen.False()
	}
	return jen.Nil()
}

// box wraps a native Go expression back into an object.Object.
func box(code *jen.Statement, t staticType) *jen.Statement {
	switch t {
	case tyInt:
		return jen.Qual(pathObject, "Integer").Call(code)
	case tyFloat:
		return jen.Qual(pathObject, "Float").Call(code)
	case tyBool:
		return jen.Qual(pathObject, "Bool").Call(code)
	}
	return code
}

// emitNative renders an expression as a plain Go expression. It mirrors
// staticTypeOf exactly and must only be called when that reported a native
// type; anything else is a bug in one of the two, and is reported as such
// rather than silently miscompiled.
//
// Native expressions have no side effects and, with the exception of division
// and modulo, cannot fail — so they collapse into a single Go expression with
// no error plumbing. Division and modulo need a zero check, and that is a
// statement, hence the leading []jen.Code most callers will find empty.
func (g *generator) emitNative(expr ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	switch e := expr.(type) {
	case *ast.Literal:
		switch v := e.Value.(type) {
		case object.Integer:
			return nil, jen.Lit(int64(v)), nil
		case object.Float:
			// Not a bare literal: Go would fold constant float expressions
			// exactly instead of rounding each operation the way the runtime
			// does (see object.FloatValue).
			return nil, jen.Qual(pathObject, "FloatValue").Call(jen.Lit(float64(v))), nil
		case object.Bool:
			return nil, jen.Lit(bool(v)), nil
		}

	case *ast.Identifier:
		return nil, jen.Id(e.Name), nil

	case *ast.CallExpression:
		// A call to a closed direct function with a native return type. The
		// call and its error check are hoisted as preceding statements and
		// the expression is the result variable.
		name, args, _ := plainCall(expr)
		pre, call, ok, err := g.tryDirectCall(name, args, onError)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("%s: internal error: call to %s typed native but not lowered", expr.Position(), name)
		}
		tmpVar := g.localName("tmp")
		errVar := g.localName("err")
		pre = append(pre,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return pre, jen.Id(tmpVar), nil

	case *ast.UnaryOperation:
		pre, operand, err := g.emitNative(e.Operand, onError)
		if err != nil {
			return nil, nil, err
		}
		if e.Operator == "-" && g.nativeTypeOf(e.Operand) == tyInt {
			pre, result := g.checkedInt(pre, "CheckedNeg", onError, operand)
			return pre, result, nil
		}
		return pre, jen.Parens(jen.Op(e.Operator).Add(operand)), nil

	case *ast.BinaryOperation:
		if e.Operator == ast.And || e.Operator == ast.Or {
			return g.emitShortCircuit(e, onError)
		}
		lhsPre, lhs, err := g.emitNative(e.LHS, onError)
		if err != nil {
			return nil, nil, err
		}
		rhsPre, rhs, err := g.emitNative(e.RHS, onError)
		if err != nil {
			return nil, nil, err
		}
		pre := append(lhsPre, rhsPre...)

		// Go has no implicit numeric conversion, so a mixed int/float operand
		// pair — which Goblin widens to float — needs the int side converted
		// explicitly. This covers comparisons too: Integer.Equals(Float) and
		// Integer.Compare(Float) both compare as float64.
		lhsType := g.nativeTypeOf(e.LHS)
		rhsType := g.nativeTypeOf(e.RHS)
		if numeric(lhsType) && numeric(rhsType) && lhsType != rhsType {
			if lhsType == tyInt {
				lhs = jen.Float64().Call(lhs)
			} else {
				rhs = jen.Float64().Call(rhs)
			}
		}

		if e.Operator == "/" {
			// Bind the divisor so it is evaluated once, then reproduce
			// Goblin's ZeroDivisionError instead of Go's panic (integers) or
			// ±Inf (floats).
			divisor := g.localName("div")
			errVar := g.localName("err")
			pre = append(pre,
				jen.Id(divisor).Op(":=").Add(rhs),
				jen.If(jen.Id(divisor).Op("==").Lit(0)).Block(
					jen.Id(errVar).Op(":=").Qual(pathObject, "NewZeroDivisionError").Call(jen.Lit("division by zero")),
					onError(errVar),
				),
			)
			if lhsType == tyInt && rhsType == tyInt {
				pre, result := g.checkedInt(pre, "CheckedDiv", onError, lhs, jen.Id(divisor))
				return pre, result, nil
			}
			return pre, jen.Float64().Call(lhs.Op("/").Id(divisor)), nil
		}

		if e.Operator == "%" {
			// Modulo needs a zero check like division, and for floats it must
			// go through math.Mod since Go's % only works on integers.
			divisor := g.localName("mod")
			errVar := g.localName("err")
			pre = append(pre,
				jen.Id(divisor).Op(":=").Add(rhs),
				jen.If(jen.Id(divisor).Op("==").Lit(0)).Block(
					jen.Id(errVar).Op(":=").Qual(pathObject, "NewZeroDivisionError").Call(jen.Lit("modulo by zero")),
					onError(errVar),
				),
			)
			if lhsType == tyInt && rhsType == tyInt {
				return pre, jen.Parens(lhs.Op("%").Id(divisor)), nil
			}
			// Mixed numeric operands were widened above, so both arguments are
			// float64 whenever this is not the integer-only case.
			return pre, jen.Parens(jen.Qual("math", "Mod").Call(lhs, jen.Id(divisor))), nil
		}

		if fn, ok := checkedIntOps[e.Operator]; ok {
			if lhsType == tyInt && rhsType == tyInt {
				pre, result := g.checkedInt(pre, fn, onError, lhs, rhs)
				return pre, result, nil
			}
			// The explicit conversion rounds the result to float64, which
			// stops Go from fusing it with a neighbouring operation into an
			// FMA (as it may on arm64): the runtime rounds every operation.
			return pre, jen.Float64().Call(lhs.Op(e.Operator).Add(rhs)), nil
		}

		// Comparisons are spelled the same in Goblin and Go, so the operator
		// carries over verbatim.
		return pre, jen.Parens(lhs.Op(e.Operator).Add(rhs)), nil
	}

	return nil, nil, fmt.Errorf("internal error: %T is not a native expression", expr)
}

// checkedIntOps maps integer arithmetic operators to the object helper that
// performs them with overflow detection.
var checkedIntOps = map[string]string{"+": "CheckedAdd", "-": "CheckedSub", "*": "CheckedMul"}

// checkedInt emits a call to one of object's Checked* helpers as a hoisted
// statement that raises OverflowError when the result does not fit, and
// returns the variable holding the result.
func (g *generator) checkedInt(pre []jen.Code, fn string, onError errHandler, args ...jen.Code) ([]jen.Code, *jen.Statement) {
	result := g.localName("n")
	ok := g.localName("ok")
	errVar := g.localName("err")
	pre = append(pre,
		jen.List(jen.Id(result), jen.Id(ok)).Op(":=").Qual(pathObject, fn).Call(args...),
		jen.If(jen.Op("!").Id(ok)).Block(
			jen.Id(errVar).Op(":=").Qual(pathObject, "NewIntegerOverflow").Call(),
			onError(errVar),
		),
	)
	return pre, jen.Id(result)
}

// emitShortCircuit renders && and || natively. When the right operand needs
// hoisted statements (a zero or overflow check, a lowered call), they must
// only run when the right operand is actually evaluated, so the result lives
// in a variable assigned inside an if instead of a single Go expression.
func (g *generator) emitShortCircuit(e *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	pre, lhs, err := g.emitNative(e.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, err := g.emitNative(e.RHS, onError)
	if err != nil {
		return nil, nil, err
	}
	if len(rhsPre) == 0 {
		return pre, jen.Parens(lhs.Op(e.Operator).Add(rhs)), nil
	}
	result := g.localName("cond")
	evaluateRHS := jen.Id(result)
	if e.Operator == ast.Or {
		evaluateRHS = jen.Op("!").Id(result)
	}
	block := append(rhsPre, jen.Id(result).Op("=").Add(rhs))
	pre = append(pre,
		jen.Id(result).Op(":=").Add(lhs),
		jen.If(evaluateRHS).Block(block...),
	)
	return pre, jen.Id(result), nil
}
