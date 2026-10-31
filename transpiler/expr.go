package transpiler

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
	"github.com/dave/jennifer/jen"
)

func transpileObject(obj object.Object) (*jen.Statement, error) {
	switch v := obj.(type) {
	case object.Bool:
		if bool(v) {
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
	return nil, fmt.Errorf("cannot transpile literal of type %s", obj.TypeName())
}

func (g *generator) transpileListLiteral(list *ast.ListLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	preStmts, elements, err := g.transpileExpressions(list.Elements, onError)
	if err != nil {
		return nil, nil, err
	}

	return preStmts, jen.Op("&").Qual(pathObject, "List").Values(
		jen.Id("Elements").Op(":").Index().Qual(pathObject, "Object").Values(elements...),
	), nil
}

func (g *generator) transpileIndexExpression(expr *ast.IndexExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	objPre, obj, err := g.transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}
	idxPre, idx, native, err := g.transpileIndexOperand(expr.Index, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := g.localName("tmp")
	errVar := g.localName("err")
	preStmts := append(objPre, idxPre...)
	var call *jen.Statement
	if native {
		call = jen.Qual(pathObject, "IndexInt").Call(obj, idx)
	} else {
		call = jen.Add(obj).Dot("Index").Call(idx)
	}
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileIndexOperand renders the index of `obj[index]` or `obj[index] = v`.
// An index that is statically an integer stays a native int64 and native
// reports true, so the caller can reach for object.IndexInt / SetIndexInt and
// skip both the boxing (an allocation for anything above 255) and the
// interface dispatch; anything else is boxed as usual.
func (g *generator) transpileIndexOperand(index ast.Expression, onError errHandler) (pre []jen.Code, code *jen.Statement, native bool, err error) {
	if g.nativeTypeOf(index) == tyInt {
		pre, code, err = g.emitNative(index, onError)
		return pre, code, true, err
	}
	pre, code, err = g.transpileExpression(index, onError)
	return pre, code, false, err
}

func (g *generator) transpileDictLiteral(dict *ast.DictLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	var preStmts []jen.Code

	dictVar := g.localName("dict")
	preStmts = append(preStmts,
		jen.Id(dictVar).Op(":=").Qual(pathObject, "NewDict").Call(),
	)

	for _, elem := range dict.Elements {
		keyPre, key, err := g.transpileExpression(elem.Key, onError)
		if err != nil {
			return nil, nil, err
		}
		valuePre, value, err := g.transpileExpression(elem.Value, onError)
		if err != nil {
			return nil, nil, err
		}
		preStmts = append(preStmts, keyPre...)
		preStmts = append(preStmts, valuePre...)
		errVar := g.localName("err")
		preStmts = append(preStmts,
			jen.If(
				jen.Id(errVar).Op(":=").Id(dictVar).Dot("Set").Call(key, value),
				jen.Id(errVar).Op("!=").Nil(),
			).Block(onError(errVar)),
		)
	}

	return preStmts, jen.Id(dictVar), nil
}

func (g *generator) transpileMemberExpression(expr *ast.MemberExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	// `self.field` inside a method is a plain Go field read on the receiver:
	// no GetAttr dispatch, no error path.
	if info, ok := g.selfMember(expr.Object, expr.Property); ok && info.fields[expr.Property] {
		return nil, jen.Id(info.receiver).Dot(fieldGoName(expr.Property)), nil
	}

	objPre, obj, err := g.transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := g.localName("attr")
	errVar := g.localName("err")
	preStmts := append(objPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(obj)).Dot("GetAttr").Call(jen.Lit(expr.Property)),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func (g *generator) transpileExpression(expr ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	// If the whole subtree evaluates natively, emit it as one Go expression and
	// box the result once, instead of one interface call and one allocation per
	// operator. Callers that can consume the unboxed value (declarations,
	// assignments, conditions) check for this themselves before calling here.
	if t := g.nativeTypeOf(expr); t.native() {
		pre, code, err := g.emitNative(expr, onError)
		if err != nil {
			return nil, nil, err
		}
		return pre, box(code, t), nil
	}

	switch v := expr.(type) {
	case *ast.Literal:
		obj, err := transpileObject(v.Value)
		if err != nil {
			return nil, nil, err
		}
		return nil, obj, nil
	case *ast.Identifier:
		if moduleVar, ok := g.moduleBinding(v.Name); ok {
			return nil, jen.Id(moduleVar), nil
		}
		if !g.isUserName(v.Name) && isBuiltinFunction(v.Name) {
			return nil, jen.Id("builtin").Dot("Members").Index(jen.Lit(v.Name)), nil
		}
		return nil, jen.Id(v.Name), nil
	case *ast.CallExpression:
		argPreStmts, call, err := g.transpileCallExpression(v, onError)
		if err != nil {
			return nil, nil, err
		}
		tmpVar := g.localName("tmp")
		errVar := g.localName("err")
		preStmts := append(argPreStmts,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Id(tmpVar), nil
	case *ast.BinaryOperation:
		return g.transpileBinaryOperation(v, onError)
	case *ast.UnaryOperation:
		return g.transpileUnaryOperation(v, onError)
	case *ast.ListLiteral:
		return g.transpileListLiteral(v, onError)
	case *ast.DictLiteral:
		return g.transpileDictLiteral(v, onError)
	case *ast.IndexExpression:
		return g.transpileIndexExpression(v, onError)
	case *ast.MemberExpression:
		return g.transpileMemberExpression(v, onError)
	case *ast.FunctionLiteral:
		return g.transpileFunctionLiteral(v)
	}
	return nil, nil, fmt.Errorf("%s: cannot transpile expression of type %T", expr.Position(), expr)
}

func (g *generator) transpileExpressions(exprs []ast.Expression, onError errHandler) ([]jen.Code, []jen.Code, error) {
	var allPreStmts []jen.Code
	var results []jen.Code
	for _, expr := range exprs {
		pre, r, err := g.transpileExpression(expr, onError)
		if err != nil {
			return nil, nil, err
		}
		allPreStmts = append(allPreStmts, pre...)
		results = append(results, r)
	}
	return allPreStmts, results, nil
}

func (g *generator) transpileCallArguments(args []ast.CallArgument, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	// Fast path: without * and ** the whole argument list is known here, so it
	// can be one composite literal instead of a callArgs temporary plus an
	// append or AddKeyword statement per argument. Beyond the statements it
	// saves, this is what keeps the argument slice on the stack: a value that
	// is only ever built and passed does not escape, while one that is
	// assigned to a local and mutated does.
	//
	// Duplicate keyword names are the one case that still needs AddKeyword,
	// which raises the same error both backends raise.
	staticShape := true
	seenKeyword := make(map[string]bool, len(args))
	for _, arg := range args {
		switch arg.Kind {
		case ast.CallArgumentPositional:
		case ast.CallArgumentKeyword:
			if seenKeyword[arg.Name] {
				staticShape = false
			}
			seenKeyword[arg.Name] = true
		default:
			staticShape = false
		}
		if !staticShape {
			break
		}
	}
	if staticShape {
		var preStmts []jen.Code
		positional := make([]jen.Code, 0, len(args))
		keyword := make([]jen.Code, 0, len(args))
		for _, arg := range args {
			argPreStmts, argExpr, err := g.transpileExpression(arg.Expr, onError)
			if err != nil {
				return nil, nil, err
			}
			preStmts = append(preStmts, argPreStmts...)
			if arg.Kind == ast.CallArgumentKeyword {
				keyword = append(keyword, jen.Values(
					jen.Id("Name").Op(":").Lit(arg.Name),
					jen.Id("Value").Op(":").Add(argExpr),
				))
				continue
			}
			positional = append(positional, argExpr)
		}
		fields := jen.Dict{}
		if len(positional) > 0 {
			fields[jen.Id("Positional")] = jen.Qual(pathObject, "Args").Values(positional...)
		}
		if len(keyword) > 0 {
			fields[jen.Id("Keyword")] = jen.Qual(pathObject, "Kwargs").Values(keyword...)
		}
		return preStmts, jen.Qual(pathObject, "CallArgs").Values(fields), nil
	}

	callArgsVar := g.localName("callArgs")
	allPreStmts := []jen.Code{
		jen.Id(callArgsVar).Op(":=").Qual(pathObject, "CallArgs").Values(),
	}

	for _, arg := range args {
		argPreStmts, argExpr, err := g.transpileExpression(arg.Expr, onError)
		if err != nil {
			return nil, nil, err
		}
		allPreStmts = append(allPreStmts, argPreStmts...)

		switch arg.Kind {
		case ast.CallArgumentPositional:
			allPreStmts = append(allPreStmts,
				jen.Id(callArgsVar).Dot("Positional").Op("=").Append(jen.Id(callArgsVar).Dot("Positional"), argExpr),
			)
		case ast.CallArgumentStarred:
			iterVar := g.localName("iter")
			errVar := g.localName("err")
			allPreStmts = append(allPreStmts,
				jen.List(jen.Id(iterVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(argExpr)).Dot("Iter").Call(),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id(callArgsVar).Dot("Positional").Op("=").Append(jen.Id(callArgsVar).Dot("Positional"), jen.Id(iterVar).Op("...")),
			)
		case ast.CallArgumentKeyword:
			// Duplicate detection lives in AddKeyword, shared with the
			// interpreter.
			errVar := g.localName("err")
			allPreStmts = append(allPreStmts,
				jen.If(
					jen.Id(errVar).Op(":=").Id(callArgsVar).Dot("AddKeyword").Call(jen.Lit(arg.Name), argExpr),
					jen.Id(errVar).Op("!=").Nil(),
				).Block(onError(errVar)),
			)
		case ast.CallArgumentKeywordUnpack:
			// Dict/key validation and duplicate detection live in
			// UnpackKeywords, shared with the interpreter.
			errVar := g.localName("err")
			allPreStmts = append(allPreStmts,
				jen.If(
					jen.Id(errVar).Op(":=").Id(callArgsVar).Dot("UnpackKeywords").Call(argExpr),
					jen.Id(errVar).Op("!=").Nil(),
				).Block(onError(errVar)),
			)
		}
	}

	return allPreStmts, jen.Id(callArgsVar), nil
}

func isBuiltinFunction(name string) bool {
	_, ok := extension.BuiltinsModule.Members[name]
	return ok
}

// directBuiltin names the Go function behind a built-in, so a call to it can
// be emitted as a plain Go call instead of a map lookup followed by
// object.Call through a function value. Skipping the function value is also
// what lets the argument list stay on the stack: escape analysis cannot see
// through an indirect call.
//
// fn is the same function the builtins module holds, and
// TestDirectBuiltinsMatchModule checks that pair by pointer, so a rename or a
// re-pointed member cannot make the emitted call go somewhere else.
type directBuiltin struct {
	path string
	name string
	fn   func(object.CallArgs) (object.Object, error)
}

var directBuiltins = map[string]directBuiltin{
	"print":    {pathExtension, "Print", extension.Print},
	"eprint":   {pathExtension, "Eprint", extension.Eprint},
	"spawn":    {pathExtension, "Spawn", extension.Spawn},
	"range":    {pathExtension, "Range", extension.Range},
	"max":      {pathExtension, "Max", extension.Max},
	"min":      {pathExtension, "Min", extension.Min},
	"Error":    {pathObject, "ErrorConstructor", object.ErrorConstructor},
	"Int":      {pathObject, "IntConstructor", object.IntConstructor},
	"Float":    {pathObject, "FloatConstructor", object.FloatConstructor},
	"Str":      {pathObject, "StrConstructor", object.StrConstructor},
	"Bytes":    {pathObject, "BytesConstructor", object.BytesConstructor},
	"Bool":     {pathObject, "BoolConstructor", object.BoolConstructor},
	"List":     {pathObject, "ListConstructor", object.ListConstructor},
	"Dict":     {pathObject, "DictConstructor", object.DictConstructor},
	"Chan":     {pathObject, "ChanConstructor", object.ChanConstructor},
	"Goblin":   {pathObject, "GoblinConstructor", object.GoblinConstructor},
	"Function": {pathObject, "FunctionConstructor", object.FunctionConstructor},
}

func (g *generator) transpileCallExpression(call *ast.CallExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if ident, ok := call.Callee.(*ast.Identifier); ok {
		if pre, direct, lowered, err := g.tryDirectCall(ident.Name, call.Args, onError); lowered || err != nil {
			return pre, direct, err
		}
	}

	argPreStmts, args, err := g.transpileCallArguments(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}

	if ident, ok := call.Callee.(*ast.Identifier); ok {
		var callee *jen.Statement
		if mapped, ok := g.moduleBinding(ident.Name); ok {
			callee = jen.Id(mapped)
		} else if !g.isUserName(ident.Name) && isBuiltinFunction(ident.Name) {
			if direct, ok := directBuiltins[ident.Name]; ok {
				return argPreStmts, jen.Qual(direct.path, direct.name).Call(args), nil
			}
			callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(ident.Name))
		} else {
			callee = jen.Id(ident.Name)
		}
		return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
	}

	if member, ok := call.Callee.(*ast.MemberExpression); ok {
		// `self.method(...)` inside a method calls the sibling's Go wrapper
		// directly. A field of the same name would take precedence at
		// runtime (GetAttr checks fields first), so it stays generic then.
		if info, isSelf := g.selfMember(member.Object, member.Property); isSelf && !info.fields[member.Property] {
			if wrapper, isMethod := info.methods[member.Property]; isMethod {
				return argPreStmts, jen.Id(info.receiver).Dot(wrapper).Call(args), nil
			}
		}
		objPre, obj, err := g.transpileExpression(member.Object, onError)
		if err != nil {
			return nil, nil, err
		}
		// object.CallMethod runs a method without materializing the bound
		// function that GetAttr would have to allocate. It falls back to
		// GetAttr itself for anything that is not a method, so a field
		// holding a function still works and error messages are unchanged.
		preStmts := append(objPre, argPreStmts...)
		return preStmts, jen.Qual(pathObject, "CallMethod").Call(obj, jen.Lit(member.Property), args), nil
	}

	calleePre, callee, err := g.transpileExpression(call.Callee, onError)
	if err != nil {
		return nil, nil, err
	}
	preStmts := append(calleePre, argPreStmts...)
	return preStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
}
