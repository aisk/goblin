package transpiler

import (
	"github.com/aisk/goblin/ast"
	"github.com/dave/jennifer/jen"
)

// Goblin evaluates the operands of an expression strictly left to right. A
// transpiled operand is a list of hoisted statements plus a Go expression for
// its value, and an expression with several operands runs every operand's
// statements before the Go expression that combines their values. A value
// that is a read of a variable is therefore taken only after the statements
// of the operands to its right have run, and a call among those may have
// changed the variable: `[x, bump()]` would see bump's assignment to x.
//
// sequence restores the order. An operand whose value code could read
// something a later operand changes is captured into a local right after its
// own statements, but only when a later operand can have side effects at all,
// so the common cases (identifiers and native arithmetic around them) keep
// their plain Go expressions.

// operand is one transpiled operand. native is the operand's static type when
// its code is an unboxed Go value, tyDynamic when it is an object.Object.
type operand struct {
	expr   ast.Expression
	pre    []jen.Code
	code   *jen.Statement
	native staticType
}

// sequence joins operands in source order: their statements in order, and
// their value codes, each captured into a local when a later operand could
// change what it reads.
func (g *generator) sequence(ops []operand) ([]jen.Code, []*jen.Statement) {
	var pre []jen.Code
	codes := make([]*jen.Statement, len(ops))
	effectsAfter := false
	capture := make([]bool, len(ops))
	for i := len(ops) - 1; i >= 0; i-- {
		capture[i] = effectsAfter && !g.stableCode(ops[i].expr)
		effectsAfter = effectsAfter || g.mayHaveEffects(ops[i].expr)
	}
	for i, op := range ops {
		pre = append(pre, op.pre...)
		codes[i] = op.code
		if capture[i] {
			pre, codes[i] = g.captureValue(pre, op.code, op.native)
		}
	}
	return pre, codes
}

// captureValue binds code to a fresh local, so it is read now.
func (g *generator) captureValue(pre []jen.Code, code *jen.Statement, native staticType) ([]jen.Code, *jen.Statement) {
	name := g.localName("val")
	typ := jen.Qual(pathObject, "Object")
	if native.native() {
		typ = jen.Id(goTypeOf(native))
	}
	return append(pre, jen.Var().Id(name).Add(typ).Op("=").Add(code)), jen.Id(name)
}

// codesOf converts value codes for jen calls that take ...jen.Code.
func codesOf(codes []*jen.Statement) []jen.Code {
	out := make([]jen.Code, len(codes))
	for i, c := range codes {
		out[i] = c
	}
	return out
}

// mayHaveEffects reports whether evaluating expr could change a variable or
// run user code. Reading names and literals cannot, nor can native arithmetic
// and comparisons, which never dispatch; a call can, and so can any boxed
// operation, which may reach a user type's trait methods.
func (g *generator) mayHaveEffects(expr ast.Expression) bool {
	switch e := expr.(type) {
	case nil:
		return false
	case *ast.Literal, *ast.Identifier, *ast.FunctionLiteral, *ast.Unpacked:
		return false
	case *ast.ListLiteral:
		for _, element := range e.Elements {
			if g.mayHaveEffects(element) {
				return true
			}
		}
		return false
	case *ast.MemberExpression:
		if info, ok := g.selfMember(e.Object, e.Property); ok && info.fields[e.Property] {
			return false
		}
	}
	if g.nativeTypeOf(expr).native() {
		calls := false
		ast.Inspect(expr, func(node ast.Statement) bool {
			if _, ok := node.(*ast.CallExpression); ok {
				calls = true
			}
			return !calls
		})
		return calls
	}
	return true
}

// stableCode reports whether the value code of expr stays the same whatever
// runs after it: a constant, or a local the expression's own statements
// assigned once. Anything else, such as a variable read or an operation
// built inline from one, is not.
func (g *generator) stableCode(expr ast.Expression) bool {
	switch e := expr.(type) {
	case nil, *ast.Literal, *ast.FunctionLiteral, *ast.Unpacked:
		return true
	case *ast.CallExpression:
		return true
	case *ast.Identifier, *ast.ListLiteral:
		return false
	case *ast.MemberExpression:
		info, ok := g.selfMember(e.Object, e.Property)
		return !(ok && info.fields[e.Property])
	}
	// Native operations are built inline from their operands; boxed ones
	// leave their result in a local.
	return !g.nativeTypeOf(expr).native()
}

// argsMayHaveEffects reports whether evaluating a call's arguments could
// change what its callee expression read.
func (g *generator) argsMayHaveEffects(args []ast.CallArgument) bool {
	for _, arg := range args {
		if arg.Kind != ast.CallArgumentPositional && arg.Kind != ast.CallArgumentKeyword {
			// Unpacking iterates or reads a dict, which may run user code.
			return true
		}
		if g.mayHaveEffects(arg.Expr) {
			return true
		}
	}
	return false
}

// nativeIf is t when an operand is native, tyDynamic when it is boxed.
func nativeIf(native bool, t staticType) staticType {
	if native {
		return t
	}
	return tyDynamic
}
