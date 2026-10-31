package ast

// Inspect traverses the tree rooted at node in depth-first pre-order, calling
// f for every statement and expression node. When f returns false, the
// node's children are skipped. Nil nodes are not visited.
//
// Children are visited in source order:
//   - FunctionDefine and FunctionLiteral: parameter defaults, then the body
//   - TypeDefine: field defaults, then every method (AllMethods) as a
//     FunctionDefine node
//   - TraitDefine: every method as a FunctionDefine node (a required method
//     has no body)
//   - CallExpression: the callee, then the argument expressions
//
// Parameter and field defaults belong to the scope enclosing the function or
// type, not to its body, so scope-aware passes usually stop at function
// nodes (return false) and walk the parts they care about themselves.
func Inspect(node Statement, f func(Statement) bool) {
	if node == nil || !f(node) {
		return
	}
	switch n := node.(type) {
	case *Declare:
		Inspect(n.Value, f)
	case *Assign:
		Inspect(n.Value, f)
	case *SetIndex:
		Inspect(n.Object, f)
		Inspect(n.Index, f)
		Inspect(n.Value, f)
	case *SetAttr:
		Inspect(n.Object, f)
		Inspect(n.Value, f)
	case *IfElse:
		Inspect(n.Condition, f)
		InspectList(n.IfBody, f)
		InspectList(n.ElseBody, f)
	case *While:
		Inspect(n.Condition, f)
		InspectList(n.Body, f)
	case *For:
		Inspect(n.Iterator, f)
		InspectList(n.Body, f)
	case *TryCatch:
		InspectList(n.TryBody, f)
		InspectList(n.CatchBody, f)
	case *Return:
		Inspect(n.Value, f)
	case *Raise:
		Inspect(n.Value, f)
	case *FunctionDefine:
		inspectParams(n.Parameters, f)
		InspectList(n.Body, f)
	case *FunctionLiteral:
		inspectParams(n.Parameters, f)
		InspectList(n.Body, f)
	case *TypeDefine:
		for _, field := range n.Fields {
			Inspect(field.DefaultValue, f)
		}
		for _, method := range n.AllMethods() {
			Inspect(method, f)
		}
	case *TraitDefine:
		for _, method := range n.Methods {
			Inspect(method, f)
		}
	case *BinaryOperation:
		Inspect(n.LHS, f)
		Inspect(n.RHS, f)
	case *UnaryOperation:
		Inspect(n.Operand, f)
	case *IndexExpression:
		Inspect(n.Object, f)
		Inspect(n.Index, f)
	case *MemberExpression:
		Inspect(n.Object, f)
	case *ListLiteral:
		for _, item := range n.Elements {
			Inspect(item, f)
		}
	case *DictLiteral:
		for _, pair := range n.Elements {
			Inspect(pair.Key, f)
			Inspect(pair.Value, f)
		}
	case *CallExpression:
		Inspect(n.Callee, f)
		InspectArgs(n.Args, f)
	}
}

// InspectList calls Inspect on every statement of a list.
func InspectList(stmts []Statement, f func(Statement) bool) {
	for _, stmt := range stmts {
		Inspect(stmt, f)
	}
}

// InspectArgs calls Inspect on the expression of every call argument.
func InspectArgs(args []CallArgument, f func(Statement) bool) {
	for _, arg := range args {
		Inspect(arg.Expr, f)
	}
}

func inspectParams(params []*Parameter, f func(Statement) bool) {
	for _, p := range params {
		Inspect(p.Default, f)
	}
}

// Any reports whether pred holds for some node under stmts, stopping at the
// first match. With descend false, the bodies of nested functions, function
// literals, types and traits are not entered (the nodes themselves are still
// tested).
func Any(stmts []Statement, descend bool, pred func(Statement) bool) bool {
	found := false
	InspectList(stmts, func(n Statement) bool {
		if found {
			return false
		}
		if pred(n) {
			found = true
			return false
		}
		return descend || !OpensScope(n)
	})
	return found
}

// OpensScope reports whether n introduces function bodies of its own: a
// function definition, a function literal, a type (its methods) or a trait
// (its default methods).
func OpensScope(n Statement) bool {
	switch n.(type) {
	case *FunctionDefine, *FunctionLiteral, *TypeDefine, *TraitDefine:
		return true
	}
	return false
}

// WalkStatements calls f for every statement of one function body: the
// statements of stmts and, recursively, of the blocks of if, while, for and
// try statements. It does not look inside expressions and never enters the
// body of a nested function, type or trait, which are scopes of their own.
func WalkStatements(stmts []Statement, f func(Statement)) {
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		f(stmt)
		switch s := stmt.(type) {
		case *IfElse:
			WalkStatements(s.IfBody, f)
			WalkStatements(s.ElseBody, f)
		case *While:
			WalkStatements(s.Body, f)
		case *For:
			WalkStatements(s.Body, f)
		case *TryCatch:
			WalkStatements(s.TryBody, f)
			WalkStatements(s.CatchBody, f)
		}
	}
}
