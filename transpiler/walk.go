package transpiler

import "github.com/aisk/goblin/ast"

// walkNodes visits every statement and expression node under stmts in
// pre-order. With nested set it descends into the bodies of function
// definitions, function literals, type methods and trait default methods;
// otherwise it visits those nodes but stays out of their bodies. The method
// nodes of a type or trait are not visited themselves, only their bodies.
// Parameter and field defaults are never entered: they belong to the
// enclosing scope and callers handle them explicitly. The callee identifier
// of a call through a plain name is not visited on its own, since the call
// node stands for it, which is how a visitor tells a call from a use of the
// name as a value.
func walkNodes(stmts []ast.Statement, nested bool, visit func(node ast.Statement)) {
	var f func(ast.Statement) bool
	bodies := func(fns []*ast.FunctionDefine) {
		if nested {
			for _, fn := range fns {
				ast.InspectList(fn.Body, f)
			}
		}
	}
	f = func(node ast.Statement) bool {
		visit(node)
		switch n := node.(type) {
		case *ast.FunctionDefine:
			bodies([]*ast.FunctionDefine{n})
			return false
		case *ast.FunctionLiteral:
			if nested {
				ast.InspectList(n.Body, f)
			}
			return false
		case *ast.TypeDefine:
			bodies(n.AllMethods())
			return false
		case *ast.TraitDefine:
			bodies(n.Methods)
			return false
		case *ast.CallExpression:
			if _, plain := n.CalleeName(); plain {
				ast.InspectArgs(n.Args, f)
				return false
			}
		}
		return true
	}
	ast.InspectList(stmts, f)
}

// walkExpr visits every node of an expression tree, function literal bodies
// included.
func walkExpr(expr ast.Expression, visit func(node ast.Statement)) {
	walkNodes([]ast.Statement{expr}, true, visit)
}

// plainCall matches a call through a plain name and returns the name and
// arguments.
func plainCall(node ast.Statement) (name string, args []ast.CallArgument, ok bool) {
	call, isCall := node.(*ast.CallExpression)
	if !isCall {
		return "", nil, false
	}
	name, ok = call.CalleeName()
	return name, call.Args, ok
}
