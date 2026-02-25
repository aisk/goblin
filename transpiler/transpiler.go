package transpiler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
	"github.com/dave/jennifer/jen"
)

const (
	pathBase                    = "github.com/aisk/goblin"
	pathObject                  = pathBase + "/object"
	pathExtension               = pathBase + "/extension"
	defaultGoblinRuntimeVersion = "v0.0.0-20260224172520-e2bc1cc1d8a5"
)

type moduleInfo struct {
	varName      string
	executorFunc string
}

var knownModules = map[string]moduleInfo{
	"os":     {varName: "os_module", executorFunc: "ExecuteOs"},
	"random": {varName: "random_module", executorFunc: "ExecuteRandom"},
}

// transpileContext holds state for a single Transpile call.
type transpileContext struct {
	localNameCounter int
	moduleImports    map[string]string   // module name -> Go variable name
	importing        map[string]struct{} // paths currently being transpiled (cycle detection)
	imported         map[string]struct{} // paths already transpiled (dedup)
	moduleFuncs      []jen.Code          // top-level module executor functions (single-file mode)
	// For directory mode:
	goModuleName string
	outputDir    string
}

func newTranspileContext() *transpileContext {
	return &transpileContext{
		localNameCounter: 0,
		moduleImports:    make(map[string]string),
		importing:        make(map[string]struct{}),
		imported:         make(map[string]struct{}),
		moduleFuncs:      nil,
	}
}

func (ctx *transpileContext) localName(prefix string) string {
	name := fmt.Sprintf("_%s_%d", prefix, ctx.localNameCounter)
	ctx.localNameCounter++
	return name
}

// errHandler generates the error-handling code for a given error variable name.
type errHandler func(errVar string) jen.Code

func isPathImport(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.Contains(path, "/")
}

func pathToFuncName(path string) string {
	s := strings.TrimPrefix(path, "./")
	s = strings.TrimPrefix(s, "../")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return "_execute_" + s
}

func Transpile(mod *ast.Module, output io.Writer) error {
	ctx := newTranspileContext()

	// Collect imports
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			if isPathImport(imp.Path) {
				ctx.moduleImports[imp.Name] = imp.Name
			} else {
				info, exists := knownModules[imp.Path]
				if !exists {
					return fmt.Errorf("unknown module: %s", imp.Path)
				}
				ctx.moduleImports[imp.Name] = info.varName
			}
		}
	}

	// Process path imports: parse and transpile each .goblin module
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		if err := ctx.transpilePathModule(imp.Path); err != nil {
			return err
		}
	}

	f := jen.NewFile(mod.Name)

	// Emit registry global variable
	hasImports := false
	for _, stmt := range mod.Body {
		if _, ok := stmt.(*ast.Import); ok {
			hasImports = true
			break
		}
	}
	if hasImports {
		f.Var().Id("_registry").Op("=").Qual(pathObject, "NewRegistry").Call()
	}

	// Emit module executor functions
	for _, fn := range ctx.moduleFuncs {
		f.Add(fn)
	}

	exportsVar := ctx.localName("exports")

	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Id(errVar))
	}

	stmts, err := ctx.transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return err
	}

	body := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports via registry
	for name, info := range knownModules {
		if _, ok := ctx.moduleImports[name]; ok {
			errVar := ctx.localName("err")
			body = append(body,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(pathExtension, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports via registry
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		errVar := ctx.localName("err")
		body = append(body,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
				jen.Lit(imp.Path),
				jen.Id(pathToFuncName(imp.Path)),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
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

// transpilePathModule parses and transpiles a .goblin file at the given path,
// generating a top-level executor function.
func (ctx *transpileContext) transpilePathModule(importPath string) error {
	absPath, err := filepath.Abs(importPath + ".goblin")
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %v", importPath, err)
	}

	// Skip already transpiled modules
	if _, ok := ctx.imported[absPath]; ok {
		return nil
	}

	// Circular import detection
	if _, ok := ctx.importing[absPath]; ok {
		return fmt.Errorf("circular import detected: %s", importPath)
	}
	ctx.importing[absPath] = struct{}{}
	defer delete(ctx.importing, absPath)

	source, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read module %s: %v", importPath, err)
	}

	l := lexer.NewLexer(source)
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		return fmt.Errorf("parse error in module %s: %v", importPath, err)
	}

	mod, ok := st.(*ast.Module)
	if !ok {
		return fmt.Errorf("internal error: unexpected AST type for module %s", importPath)
	}

	// Collect sub-module imports
	subModuleImports := make(map[string]string)
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			if isPathImport(imp.Path) {
				subModuleImports[imp.Name] = imp.Name
				if err := ctx.transpilePathModule(imp.Path); err != nil {
					return err
				}
			} else {
				info, exists := knownModules[imp.Path]
				if !exists {
					return fmt.Errorf("unknown module in %s: %s", importPath, imp.Path)
				}
				subModuleImports[imp.Name] = info.varName
			}
		}
	}

	// Save and restore module imports for this scope
	savedImports := ctx.moduleImports
	ctx.moduleImports = subModuleImports
	defer func() { ctx.moduleImports = savedImports }()

	exportsVar := ctx.localName("exports")

	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Id(errVar))
	}

	stmts, err := ctx.transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return fmt.Errorf("transpile error in module %s: %v", importPath, err)
	}

	funcBody := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports for this sub-module via registry
	for name, info := range knownModules {
		if _, ok := subModuleImports[name]; ok {
			errVar := ctx.localName("err")
			funcBody = append(funcBody,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(pathExtension, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports for this sub-module
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		errVar := ctx.localName("err")
		funcBody = append(funcBody,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
				jen.Lit(imp.Path),
				jen.Id(pathToFuncName(imp.Path)),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
	}

	funcBody = append(funcBody, stmts...)
	funcBody = append(funcBody,
		jen.Return(
			jen.Op("&").Qual(pathObject, "Module").Values(
				jen.Id("Members").Op(":").Id(exportsVar),
			),
			jen.Nil(),
		),
	)

	funcName := pathToFuncName(importPath)
	fn := jen.Func().Id(funcName).Params().Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(funcBody...)

	ctx.moduleFuncs = append(ctx.moduleFuncs, fn)
	ctx.imported[absPath] = struct{}{}
	return nil
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

func (ctx *transpileContext) transpileListLiteral(list *ast.ListLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	preStmts, elements, err := ctx.transpileExpressions(list.Elements, onError)
	if err != nil {
		return nil, nil, err
	}

	return preStmts, jen.Op("&").Qual(pathObject, "List").Values(
		jen.Id("Elements").Op(":").Index().Qual(pathObject, "Object").Values(elements...),
	), nil
}

func (ctx *transpileContext) transpileIndexExpression(expr *ast.IndexExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	objPre, obj, err := ctx.transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}
	idxPre, idx, err := ctx.transpileExpression(expr.Index, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(objPre, idxPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(obj).Dot("Index").Call(idx),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func (ctx *transpileContext) transpileDictLiteral(dict *ast.DictLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	var preStmts []jen.Code
	var entries []jen.Code

	for _, elem := range dict.Elements {
		keyPre, key, err := ctx.transpileExpression(elem.Key, onError)
		if err != nil {
			return nil, nil, err
		}
		valuePre, value, err := ctx.transpileExpression(elem.Value, onError)
		if err != nil {
			return nil, nil, err
		}
		preStmts = append(preStmts, keyPre...)
		preStmts = append(preStmts, valuePre...)
		entries = append(entries, jen.Values(jen.Id("Key").Op(":").Add(key), jen.Id("Value").Op(":").Add(value)))
	}

	dictVar := ctx.localName("dict")
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

func (ctx *transpileContext) transpileMemberExpression(expr *ast.MemberExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	objPre, obj, err := ctx.transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := ctx.localName("attr")
	errVar := ctx.localName("err")
	preStmts := append(objPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(obj).Dot("GetAttr").Call(jen.Lit(expr.Property)),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func (ctx *transpileContext) transpileExpression(expr ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	switch v := expr.(type) {
	case *ast.Literal:
		obj, err := transpileObject(v.Value)
		if err != nil {
			return nil, nil, err
		}
		return nil, obj, nil
	case *ast.Identifier:
		if moduleVar, ok := ctx.moduleImports[v.Name]; ok {
			return nil, jen.Id(moduleVar), nil
		}
		return nil, jen.Id(v.Name), nil
	case *ast.FunctionCall:
		argPreStmts, call, err := ctx.transpileFunctionCall(v, onError)
		if err != nil {
			return nil, nil, err
		}
		tmpVar := ctx.localName("tmp")
		errVar := ctx.localName("err")
		preStmts := append(argPreStmts,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Id(tmpVar), nil
	case *ast.CallExpression:
		argPreStmts, call, err := ctx.transpileCallExpression(v, onError)
		if err != nil {
			return nil, nil, err
		}
		tmpVar := ctx.localName("tmp")
		errVar := ctx.localName("err")
		preStmts := append(argPreStmts,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Id(tmpVar), nil
	case *ast.BinaryOperation:
		return ctx.transpileBinaryOperation(v, onError)
	case *ast.UnaryOperation:
		return ctx.transpileUnaryOperation(v, onError)
	case *ast.ListLiteral:
		return ctx.transpileListLiteral(v, onError)
	case *ast.DictLiteral:
		return ctx.transpileDictLiteral(v, onError)
	case *ast.IndexExpression:
		return ctx.transpileIndexExpression(v, onError)
	case *ast.MemberExpression:
		return ctx.transpileMemberExpression(v, onError)
	}
	return nil, nil, object.NotImplementedError
}

func (ctx *transpileContext) transpileExpressions(exprs []ast.Expression, onError errHandler) ([]jen.Code, []jen.Code, error) {
	var allPreStmts []jen.Code
	var results []jen.Code
	for _, expr := range exprs {
		pre, r, err := ctx.transpileExpression(expr, onError)
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

func (ctx *transpileContext) transpileDeclare(decl *ast.Declare, onError errHandler) ([]jen.Code, error) {
	preStmts, value, err := ctx.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	declStmt := jen.Var().Id(decl.Name).Qual(pathObject, "Object").Op("=").Add(value)
	declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
	return append(preStmts, declStmt), nil
}

func (ctx *transpileContext) transpileAssign(decl *ast.Assign, onError errHandler) ([]jen.Code, error) {
	preStmts, value, err := ctx.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	assignStmt := jen.Id(decl.Target).Op("=").Add(value)
	assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
	return append(preStmts, assignStmt), nil
}

func (ctx *transpileContext) transpileIfElse(ifelse *ast.IfElse, onError errHandler) ([]jen.Code, error) {
	condPreStmts, cond, err := ctx.transpileExpression(ifelse.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(ifelse.IfBody, onError, "")
	if err != nil {
		return nil, err
	}
	elseBody, err := ctx.transpileStatements(ifelse.ElseBody, onError, "")
	if err != nil {
		return nil, err
	}
	ifStmt := jen.If(cond.Dot("Bool").Call()).Block(body...).Else().Block(elseBody...)
	return append(condPreStmts, ifStmt), nil
}

func (ctx *transpileContext) transpileWhile(while_ *ast.While, onError errHandler) ([]jen.Code, error) {
	condPreStmts, cond, err := ctx.transpileExpression(while_.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(while_.Body, onError, "")
	if err != nil {
		return nil, err
	}

	if len(condPreStmts) > 0 {
		loopBody := append(condPreStmts,
			jen.If(jen.Op("!").Add(cond).Dot("Bool").Call()).Block(jen.Break()),
		)
		loopBody = append(loopBody, body...)
		return []jen.Code{jen.For().Block(loopBody...)}, nil
	}

	return []jen.Code{jen.For(cond.Dot("Bool").Call()).Block(body...)}, nil
}

func (ctx *transpileContext) transpileBreak(break_ *ast.Break) ([]jen.Code, error) {
	return []jen.Code{jen.Break()}, nil
}

func (ctx *transpileContext) transpileFor(for_ *ast.For, onError errHandler) ([]jen.Code, error) {
	iterPreStmts, iterator, err := ctx.transpileExpression(for_.Iterator, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(for_.Body, onError, "")
	if err != nil {
		return nil, err
	}

	iterVar := ctx.localName("iter")
	elementsVar := ctx.localName("elements")
	errVar := ctx.localName("err")

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

func (ctx *transpileContext) transpileFunctionCall(call *ast.FunctionCall, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	argPreStmts, l, err := ctx.transpileExpressions(call.Args, onError)
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

func (ctx *transpileContext) transpileCallExpression(call *ast.CallExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	argPreStmts, l, err := ctx.transpileExpressions(call.Args, onError)
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
		objPre, obj, err := ctx.transpileExpression(member.Object, onError)
		if err != nil {
			return nil, nil, err
		}
		attrVar := ctx.localName("attr")
		errVar := ctx.localName("err")
		preStmts := append(objPre, argPreStmts...)
		preStmts = append(preStmts,
			jen.List(jen.Id(attrVar), jen.Id(errVar)).Op(":=").Add(obj).Dot("GetAttr").Call(jen.Lit(member.Property)),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Qual(pathObject, "Call").Call(jen.Id(attrVar), args, kwargs), nil
	}

	calleePre, callee, err := ctx.transpileExpression(call.Callee, onError)
	if err != nil {
		return nil, nil, err
	}
	preStmts := append(calleePre, argPreStmts...)
	return preStmts, jen.Qual(pathObject, "Call").Call(callee, args, kwargs), nil
}

func (ctx *transpileContext) transpileFunctionDefine(fn *ast.FunctionDefine, onError errHandler) ([]jen.Code, error) {
	argsName := ctx.localName("args")
	kwargsName := ctx.localName("kwargs")

	argsDefine := []jen.Code{}
	for i, param := range fn.Parameters {
		def := jen.Var().Id(param).Op("=").Id(argsName).Index(jen.Lit(i))
		def.Op(";").Id("_").Op("=").Id(param)
		argsDefine = append(argsDefine, def)
	}

	fnOnError := func(errVar string) jen.Code {
		return jen.Return(jen.List(jen.Nil(), jen.Id(errVar)))
	}

	body, err := ctx.transpileStatements(fn.Body, fnOnError, "")
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

func (ctx *transpileContext) transpileReturn(return_ *ast.Return, onError errHandler) ([]jen.Code, error) {
	preStmts, r, err := ctx.transpileExpression(return_.Value, onError)
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

func (ctx *transpileContext) transpileComparisonOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, err := ctx.transpileExpression(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	cmpVar := ctx.localName("cmp")
	errVar := ctx.localName("err")
	tmpVar := ctx.localName("tmp")

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

func (ctx *transpileContext) transpileBinaryOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if isComparisonOperator(operation.Operator) {
		return ctx.transpileComparisonOperation(operation, onError)
	}

	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, err := ctx.transpileExpression(operation.RHS, onError)
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

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(lhsPre, rhsPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(lhs).Dot(methodName).Call(rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func (ctx *transpileContext) transpileUnaryOperation(operation *ast.UnaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	operandPre, operand, err := ctx.transpileExpression(operation.Operand, onError)
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

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(operandPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(operand).Dot(methodName).Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func (ctx *transpileContext) transpileExport(export *ast.Export, exportsVar string) ([]jen.Code, error) {
	return []jen.Code{
		jen.Id(exportsVar).Index(jen.Lit(export.Name)).Op("=").Id(export.Name),
	}, nil
}

func (ctx *transpileContext) transpileStatement(stmt ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	switch v := stmt.(type) {
	case *ast.Declare:
		return ctx.transpileDeclare(v, onError)
	case *ast.Assign:
		return ctx.transpileAssign(v, onError)
	case *ast.FunctionCall:
		argPreStmts, call, err := ctx.transpileFunctionCall(v, onError)
		if err != nil {
			return nil, err
		}
		errVar := ctx.localName("err")
		stmts := append(argPreStmts,
			jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return stmts, nil
	case *ast.CallExpression:
		argPreStmts, call, err := ctx.transpileCallExpression(v, onError)
		if err != nil {
			return nil, err
		}
		errVar := ctx.localName("err")
		stmts := append(argPreStmts,
			jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return stmts, nil
	case *ast.FunctionDefine:
		return ctx.transpileFunctionDefine(v, onError)
	case *ast.IfElse:
		return ctx.transpileIfElse(v, onError)
	case *ast.While:
		return ctx.transpileWhile(v, onError)
	case *ast.For:
		return ctx.transpileFor(v, onError)
	case *ast.Break:
		return ctx.transpileBreak(v)
	case *ast.Return:
		return ctx.transpileReturn(v, onError)
	case *ast.Export:
		return ctx.transpileExport(v, exportsVar)
	case *ast.Import:
		return nil, nil
	case *ast.BinaryOperation:
		pre, _, err := ctx.transpileBinaryOperation(v, onError)
		return pre, err
	case *ast.UnaryOperation:
		pre, _, err := ctx.transpileUnaryOperation(v, onError)
		return pre, err
	case *ast.MemberExpression:
		pre, _, err := ctx.transpileMemberExpression(v, onError)
		return pre, err
	}
	return nil, object.NotImplementedError
}

func (ctx *transpileContext) transpileStatements(stmts []ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	var result []jen.Code
	for _, stmt := range stmts {
		codes, err := ctx.transpileStatement(stmt, onError, exportsVar)
		if err != nil {
			return nil, err
		}
		result = append(result, codes...)
	}
	return result, nil
}

// pathToPackageName returns the last path segment as the package name.
func pathToPackageName(importPath string) string {
	return filepath.Base(importPath)
}

// pathToRelDir strips the leading "./" prefix from a relative import path.
func pathToRelDir(importPath string) string {
	s := strings.TrimPrefix(importPath, "./")
	s = strings.TrimPrefix(s, "../")
	return s
}

// detectGoblinRoot walks up from the current working directory (then the
// executable path) looking for a go.mod that declares github.com/aisk/goblin.
func detectGoblinRoot() string {
	// Try walking up from cwd
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 10; i++ {
			data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil && strings.Contains(string(data), "module github.com/aisk/goblin") {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Try walking up from executable
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 5; i++ {
			data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil && strings.Contains(string(data), "module github.com/aisk/goblin") {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return ""
}

// generateGoMod writes the go.mod file for the output directory.
func generateGoMod(outputDir, moduleName string) error {
	goblinRoot := detectGoblinRoot()
	content := generateGoModContent(moduleName, defaultGoblinRuntimeVersion, goblinRoot)
	return os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(content), 0644)
}

func generateGoModContent(moduleName, runtimeVersion, goblinRoot string) string {
	if goblinRoot != "" {
		return fmt.Sprintf(
			"module %s\n\ngo 1.19\n\nrequire github.com/aisk/goblin %s\n\nreplace github.com/aisk/goblin => %s\n",
			moduleName, runtimeVersion, goblinRoot,
		)
	}
	return fmt.Sprintf(
		"module %s\n\ngo 1.19\n\nrequire github.com/aisk/goblin %s\n",
		moduleName, runtimeVersion,
	)
}

// TranspileToDir transpiles a goblin module into a Go module directory structure.
// The entry-point module becomes output/main.go; each imported path module becomes
// its own package under outputDir.
func TranspileToDir(mod *ast.Module, sourceFile, outputDir string) error {
	base := filepath.Base(sourceFile)
	moduleName := strings.TrimSuffix(base, ".goblin")

	ctx := newTranspileContext()
	ctx.goModuleName = moduleName
	ctx.outputDir = outputDir

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Recursively transpile each path import into its own package file.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		if err := ctx.transpilePathModuleToFile(imp.Path); err != nil {
			return err
		}
	}

	if err := ctx.generateMainFile(mod); err != nil {
		return err
	}

	return generateGoMod(outputDir, moduleName)
}

// transpilePathModuleToFile parses a .goblin file at importPath and writes it
// as a separate Go package file under ctx.outputDir.
func (ctx *transpileContext) transpilePathModuleToFile(importPath string) error {
	absPath, err := filepath.Abs(importPath + ".goblin")
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %v", importPath, err)
	}

	if _, ok := ctx.imported[absPath]; ok {
		return nil
	}
	if _, ok := ctx.importing[absPath]; ok {
		return fmt.Errorf("circular import detected: %s", importPath)
	}
	ctx.importing[absPath] = struct{}{}
	defer delete(ctx.importing, absPath)

	source, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read module %s: %v", importPath, err)
	}

	l := lexer.NewLexer(source)
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		return fmt.Errorf("parse error in module %s: %v", importPath, err)
	}

	mod, ok := st.(*ast.Module)
	if !ok {
		return fmt.Errorf("internal error: unexpected AST type for module %s", importPath)
	}

	// Process sub-imports first (depth-first).
	subModuleImports := make(map[string]string)
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok {
			continue
		}
		if isPathImport(imp.Path) {
			subModuleImports[imp.Name] = imp.Name
			if err := ctx.transpilePathModuleToFile(imp.Path); err != nil {
				return err
			}
		} else {
			info, exists := knownModules[imp.Path]
			if !exists {
				return fmt.Errorf("unknown module in %s: %s", importPath, imp.Path)
			}
			subModuleImports[imp.Name] = info.varName
		}
	}

	pkgName := pathToPackageName(importPath)
	relDir := pathToRelDir(importPath)
	pkgDir := filepath.Join(ctx.outputDir, relDir)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return fmt.Errorf("failed to create package directory %s: %v", pkgDir, err)
	}

	f := jen.NewFile(pkgName)

	// Register import aliases so jennifer uses _pkg_X for sub-path-imports.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		subPkgName := pathToPackageName(imp.Path)
		subRelDir := pathToRelDir(imp.Path)
		subImportPath := ctx.goModuleName + "/" + subRelDir
		f.ImportAlias(subImportPath, "_pkg_"+subPkgName)
	}

	savedImports := ctx.moduleImports
	ctx.moduleImports = subModuleImports
	defer func() { ctx.moduleImports = savedImports }()

	exportsVar := ctx.localName("exports")
	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Id(errVar))
	}

	stmts, err := ctx.transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return fmt.Errorf("transpile error in module %s: %v", importPath, err)
	}

	funcBody := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports via registry parameter.
	for name, info := range knownModules {
		if _, ok := subModuleImports[name]; ok {
			errVar := ctx.localName("err")
			funcBody = append(funcBody,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(pathExtension, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports via closure that passes registry down.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		subRelDir := pathToRelDir(imp.Path)
		subImportPath := ctx.goModuleName + "/" + subRelDir
		errVar := ctx.localName("err")
		funcBody = append(funcBody,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("registry").Dot("Load").Call(
				jen.Lit(imp.Path),
				jen.Func().Params().Parens(jen.List(
					jen.Qual(pathObject, "Object"), jen.Error(),
				)).Block(
					jen.Return(jen.Qual(subImportPath, "Execute").Call(jen.Id("registry"))),
				),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
	}

	funcBody = append(funcBody, stmts...)
	funcBody = append(funcBody,
		jen.Return(
			jen.Op("&").Qual(pathObject, "Module").Values(
				jen.Id("Members").Op(":").Id(exportsVar),
			),
			jen.Nil(),
		),
	)

	f.Func().Id("Execute").Params(
		jen.Id("registry").Op("*").Qual(pathObject, "Registry"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(funcBody...)

	outFile := filepath.Join(pkgDir, pkgName+".go")
	fh, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", outFile, err)
	}
	defer fh.Close()

	if err := f.Render(fh); err != nil {
		return fmt.Errorf("failed to render file %s: %v", outFile, err)
	}

	ctx.imported[absPath] = struct{}{}
	return nil
}

// generateMainFile generates output/main.go for the top-level module.
func (ctx *transpileContext) generateMainFile(mod *ast.Module) error {
	f := jen.NewFile("main")

	// Register import aliases for path imports.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		pkgName := pathToPackageName(imp.Path)
		relDir := pathToRelDir(imp.Path)
		importPath := ctx.goModuleName + "/" + relDir
		f.ImportAlias(importPath, "_pkg_"+pkgName)
	}

	// Emit _registry global if there are any imports.
	for _, stmt := range mod.Body {
		if _, ok := stmt.(*ast.Import); ok {
			f.Var().Id("_registry").Op("=").Qual(pathObject, "NewRegistry").Call()
			break
		}
	}

	// Build module imports map for this scope.
	mainModuleImports := make(map[string]string)
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok {
			continue
		}
		if isPathImport(imp.Path) {
			mainModuleImports[imp.Name] = imp.Name
		} else {
			info, exists := knownModules[imp.Path]
			if !exists {
				return fmt.Errorf("unknown module: %s", imp.Path)
			}
			mainModuleImports[imp.Name] = info.varName
		}
	}

	savedImports := ctx.moduleImports
	ctx.moduleImports = mainModuleImports
	defer func() { ctx.moduleImports = savedImports }()

	exportsVar := ctx.localName("exports")
	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Id(errVar))
	}

	stmts, err := ctx.transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return err
	}

	body := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports via _registry global.
	for name, info := range knownModules {
		if _, ok := mainModuleImports[name]; ok {
			errVar := ctx.localName("err")
			body = append(body,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(pathExtension, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports via closure passing _registry down.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		relDir := pathToRelDir(imp.Path)
		importPath := ctx.goModuleName + "/" + relDir
		errVar := ctx.localName("err")
		body = append(body,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
				jen.Lit(imp.Path),
				jen.Func().Params().Parens(jen.List(
					jen.Qual(pathObject, "Object"), jen.Error(),
				)).Block(
					jen.Return(jen.Qual(importPath, "Execute").Call(jen.Id("_registry"))),
				),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
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

	outFile := filepath.Join(ctx.outputDir, "main.go")
	fh, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create main.go: %v", err)
	}
	defer fh.Close()

	return f.Render(fh)
}
