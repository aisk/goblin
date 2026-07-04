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
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

const (
	pathBase                    = "github.com/aisk/goblin"
	pathObject                  = pathBase + "/object"
	pathExtension               = pathBase + "/extension"
	defaultGoblinRuntimeVersion = "v0.0.0-20260224172520-e2bc1cc1d8a5"
)

type moduleInfo struct {
	executorPath string
	varName      string
	executorFunc string
}

var knownModules = map[string]moduleInfo{
	"os":     {executorPath: pathExtension, varName: "os_module", executorFunc: "ExecuteOs"},
	"random": {executorPath: pathExtension, varName: "random_module", executorFunc: "ExecuteRandom"},
	"math":   {executorPath: pathExtension, varName: "math_module", executorFunc: "ExecuteMath"},
	"http":   {executorPath: pathExtension + "/http", varName: "http_module", executorFunc: "Execute"},
	"fs":     {executorPath: pathExtension + "/fs", varName: "fs_module", executorFunc: "Execute"},
	"mime":   {executorPath: pathExtension, varName: "mime_module", executorFunc: "ExecuteMime"},
	"json":   {executorPath: pathExtension, varName: "json_module", executorFunc: "ExecuteJson"},
	"path":   {executorPath: pathExtension + "/path", varName: "path_module", executorFunc: "Execute"},
	"time":   {executorPath: pathExtension + "/time", varName: "time_module", executorFunc: "Execute"},
}

// transpileContext holds state for a single Transpile call.
type transpileContext struct {
	localNameCounter int
	moduleImports    map[string]string   // module name -> Go variable name
	importing        map[string]struct{} // paths currently being transpiled (cycle detection)
	imported         map[string]struct{} // paths already transpiled (dedup)
	moduleFuncs      []jen.Code          // top-level module executor functions (single-file mode)
	topDecls         []jen.Code          // top-level type declarations and methods
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
		topDecls:         nil,
	}
}

func (ctx *transpileContext) localName(prefix string) string {
	name := fmt.Sprintf("_%s_%d", prefix, ctx.localNameCounter)
	ctx.localNameCounter++
	return name
}

func (ctx *transpileContext) goTypeName(name string) string {
	return name
}

func exportedName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// reservedGoMethodNames are the object.Object interface methods (plus the
// optional setter interfaces) generated on every user type's struct. A
// user-defined goblin method whose exported name collides with one of these
// must be mangled so both can coexist on the same receiver.
var reservedGoMethodNames = map[string]bool{
	"String": true, "Bool": true, "Compare": true, "Add": true,
	"Minus": true, "Multiply": true, "Divide": true, "And": true,
	"Or": true, "Not": true, "Iter": true, "Index": true,
	"GetAttr": true, "SetAttr": true, "SetIndex": true,
}

// methodWrapperName returns the Go method name for a user-defined goblin
// method, mangled to avoid colliding with the interface methods generated on
// the same struct (e.g. goblin `add` -> Go `Add_`).
func methodWrapperName(name string) string {
	n := exportedName(name)
	if reservedGoMethodNames[n] {
		return n + "_"
	}
	return n
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
	if err := semantic.CheckModule(mod); err != nil {
		return err
	}

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

	exportsVar := ctx.localName("exports")

	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Qual("github.com/pkg/errors", "WithStack").Call(jen.Id(errVar)))
	}

	stmts, err := ctx.transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return err
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
	}

	for _, fn := range ctx.moduleFuncs {
		f.Add(fn)
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
					jen.Qual(info.executorPath, info.executorFunc),
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
			jen.Qual("fmt", "Fprintf").Call(jen.Qual("os", "Stderr"), jen.Lit("%+v\n"), jen.Id("err")),
			jen.Qual("os", "Exit").Call(jen.Lit(1)),
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

	l, err := lexer.NewLexerFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read module %s: %v", importPath, err)
	}
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		return fmt.Errorf("parse error in module %s: %v", importPath, err)
	}

	mod, ok := st.(*ast.Module)
	if !ok {
		return fmt.Errorf("internal error: unexpected AST type for module %s", importPath)
	}
	if err := semantic.CheckModule(mod); err != nil {
		return fmt.Errorf("semantic error in module %s: %v", importPath, err)
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
		return jen.Return(jen.Nil(), jen.Qual("github.com/pkg/errors", "WithStack").Call(jen.Id(errVar)))
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
					jen.Qual(info.executorPath, info.executorFunc),
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

	dictVar := ctx.localName("dict")
	preStmts = append(preStmts,
		jen.Id(dictVar).Op(":=").Qual(pathObject, "NewDict").Call(),
	)

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
		preStmts = append(preStmts, jen.Id(dictVar).Dot("Set").Call(key, value))
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
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(obj)).Dot("GetAttr").Call(jen.Lit(expr.Property)),
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
		if isBuiltinFunction(v.Name) {
			return nil, jen.Id("builtin").Dot("Members").Index(jen.Lit(v.Name)), nil
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
	case *ast.FunctionLiteral:
		return ctx.transpileFunctionLiteral(v)
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

func (ctx *transpileContext) transpileCallArguments(args []ast.CallArgument, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	// Fast path: when every argument is a plain positional (no *, **, or keyword
	// argument), the result is a simple slice literal. This avoids emitting the
	// positional/keyword/callArgs temporaries and per-argument append statements
	// that the general path below needs, which is the common case for most calls.
	allPositional := true
	for _, arg := range args {
		if arg.Kind != ast.CallArgumentPositional {
			allPositional = false
			break
		}
	}
	if allPositional {
		var preStmts []jen.Code
		argExprs := make([]jen.Code, 0, len(args))
		for _, arg := range args {
			argPreStmts, argExpr, err := ctx.transpileExpression(arg.Expr, onError)
			if err != nil {
				return nil, nil, err
			}
			preStmts = append(preStmts, argPreStmts...)
			argExprs = append(argExprs, argExpr)
		}
		callArgs := jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
			jen.Id("Positional"): jen.Qual(pathObject, "Args").Values(argExprs...),
		})
		return preStmts, callArgs, nil
	}

	positionalVar := ctx.localName("positional")
	keywordVar := ctx.localName("keyword")
	callArgsVar := ctx.localName("callArgs")
	allPreStmts := []jen.Code{
		jen.Id(positionalVar).Op(":=").Qual(pathObject, "Args").Values(),
		jen.Id(keywordVar).Op(":=").Qual(pathObject, "Kwargs").Values(),
	}

	for _, arg := range args {
		argPreStmts, argExpr, err := ctx.transpileExpression(arg.Expr, onError)
		if err != nil {
			return nil, nil, err
		}
		allPreStmts = append(allPreStmts, argPreStmts...)

		switch arg.Kind {
		case ast.CallArgumentPositional:
			allPreStmts = append(allPreStmts,
				jen.Id(positionalVar).Op("=").Append(jen.Id(positionalVar), argExpr),
			)
		case ast.CallArgumentStarred:
			iterVar := ctx.localName("iter")
			errVar := ctx.localName("err")
			allPreStmts = append(allPreStmts,
				jen.List(jen.Id(iterVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(argExpr)).Dot("Iter").Call(),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id(positionalVar).Op("=").Append(jen.Id(positionalVar), jen.Id(iterVar).Op("...")),
			)
		case ast.CallArgumentKeyword:
			okVar := ctx.localName("ok")
			errVar := ctx.localName("err")
			allPreStmts = append(allPreStmts,
				jen.List(jen.Id("_"), jen.Id(okVar)).Op(":=").Id(keywordVar).Index(jen.Lit(arg.Name)),
				jen.If(jen.Id(okVar)).Block(
					jen.Id(errVar).Op(":=").Qual("fmt", "Errorf").Call(
						jen.Lit("got multiple values for argument '%s'"),
						jen.Lit(arg.Name),
					),
					onError(errVar),
				),
				jen.Id(keywordVar).Index(jen.Lit(arg.Name)).Op("=").Add(argExpr),
			)
		case ast.CallArgumentKeywordUnpack:
			unpackObjVar := ctx.localName("unpackObj")
			dictVar := ctx.localName("dict")
			okVar := ctx.localName("ok")
			entryVar := ctx.localName("entry")
			keyVar := ctx.localName("key")
			keyObjVar := ctx.localName("keyObj")
			existsVar := ctx.localName("exists")
			errVar := ctx.localName("err")

			allPreStmts = append(allPreStmts,
				jen.Id(unpackObjVar).Op(":=").Add(argExpr),
				jen.List(jen.Id(dictVar), jen.Id(okVar)).Op(":=").Id(unpackObjVar).Assert(jen.Op("*").Qual(pathObject, "Dict")),
				jen.If(jen.Op("!").Id(okVar)).Block(
					jen.Id(errVar).Op(":=").Qual("fmt", "Errorf").Call(
						jen.Lit("keyword unpack argument must be a dict, got %T"),
						jen.Id(unpackObjVar),
					),
					onError(errVar),
				),
				jen.For(jen.List(jen.Id("_"), jen.Id(entryVar)).Op(":=").Op("range").Id(dictVar).Dot("Entries")).Block(
					jen.Id(keyObjVar).Op(":=").Id(entryVar).Dot("Key"),
					jen.List(jen.Id(keyVar), jen.Id(okVar)).Op(":=").Id(keyObjVar).Assert(jen.Qual(pathObject, "String")),
					jen.If(jen.Op("!").Id(okVar)).Block(
						jen.Id(errVar).Op(":=").Qual("fmt", "Errorf").Call(
							jen.Lit("keyword argument name must be a string, got %T"),
							jen.Id(keyObjVar),
						),
						onError(errVar),
					),
					jen.List(jen.Id("_"), jen.Id(existsVar)).Op(":=").Id(keywordVar).Index(jen.String().Call(jen.Id(keyVar))),
					jen.If(jen.Id(existsVar)).Block(
						jen.Id(errVar).Op(":=").Qual("fmt", "Errorf").Call(
							jen.Lit("got multiple values for argument '%s'"),
							jen.Id(keyVar),
						),
						onError(errVar),
					),
					jen.Id(keywordVar).Index(jen.String().Call(jen.Id(keyVar))).Op("=").Id(entryVar).Dot("Value"),
				),
			)
		}
	}

	allPreStmts = append(allPreStmts,
		jen.Id(callArgsVar).Op(":=").Qual(pathObject, "CallArgs").Values(jen.Dict{
			jen.Id("Positional"): jen.Id(positionalVar),
			jen.Id("Keyword"):    jen.Id(keywordVar),
		}),
	)

	return allPreStmts, jen.Id(callArgsVar), nil
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

func (ctx *transpileContext) transpileSetIndex(s *ast.SetIndex, onError errHandler) ([]jen.Code, error) {
	objPre, obj, err := ctx.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	idxPre, idx, err := ctx.transpileExpression(s.Index, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := ctx.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	errVar := ctx.localName("err")
	stmts := append(objPre, idxPre...)
	stmts = append(stmts, valPre...)
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "SetItem").Call(obj, idx, val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (ctx *transpileContext) transpileSetAttr(s *ast.SetAttr, onError errHandler) ([]jen.Code, error) {
	objPre, obj, err := ctx.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := ctx.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	errVar := ctx.localName("err")
	stmts := append(objPre, valPre...)
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "SetAttribute").Call(obj, jen.Lit(s.Property), val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
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
	condVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	stmts := append([]jen.Code{}, condPreStmts...)
	stmts = append(stmts,
		jen.List(jen.Id(condVar), jen.Id(errVar)).Op(":=").Qual(pathObject, "Truthy").Call(cond),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.If(jen.Id(condVar)).Block(body...).Else().Block(elseBody...),
	)
	return stmts, nil
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

	condVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	loopBody := append([]jen.Code{}, condPreStmts...)
	loopBody = append(loopBody,
		jen.List(jen.Id(condVar), jen.Id(errVar)).Op(":=").Qual(pathObject, "Truthy").Call(cond),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.If(jen.Op("!").Id(condVar)).Block(jen.Break()),
	)
	loopBody = append(loopBody, body...)
	return []jen.Code{jen.For().Block(loopBody...)}, nil
}

func (ctx *transpileContext) transpileBreak(break_ *ast.Break) ([]jen.Code, error) {
	return []jen.Code{jen.Break()}, nil
}

func (ctx *transpileContext) transpileContinue(continue_ *ast.Continue) ([]jen.Code, error) {
	return []jen.Code{jen.Continue()}, nil
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
	argPreStmts, args, err := ctx.transpileCallArguments(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}

	var callee *jen.Statement
	if isBuiltinFunction(call.Name) {
		callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(call.Name))
	} else if mapped, ok := ctx.moduleImports[call.Name]; ok {
		callee = jen.Id(mapped)
	} else {
		callee = jen.Id(call.Name)
	}

	return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
}

func (ctx *transpileContext) transpileCallExpression(call *ast.CallExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	argPreStmts, args, err := ctx.transpileCallArguments(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}

	if ident, ok := call.Callee.(*ast.Identifier); ok {
		var callee *jen.Statement
		if isBuiltinFunction(ident.Name) {
			callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(ident.Name))
		} else if mapped, ok := ctx.moduleImports[ident.Name]; ok {
			callee = jen.Id(mapped)
		} else {
			callee = jen.Id(ident.Name)
		}
		return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
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
			jen.List(jen.Id(attrVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(obj)).Dot("GetAttr").Call(jen.Lit(member.Property)),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Qual(pathObject, "Call").Call(jen.Id(attrVar), args), nil
	}

	calleePre, callee, err := ctx.transpileExpression(call.Callee, onError)
	if err != nil {
		return nil, nil, err
	}
	preStmts := append(calleePre, argPreStmts...)
	return preStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
}

// emitParameterBinding emits the statements that bind a CallArgs value (the
// local named callArgsName) to one Go variable per parameter. It splits params
// into fixed / *varargs / **kwargs, calls object.BindArguments, and unpacks the
// resulting map. name is used only for BindArguments diagnostics, and fnOnError
// builds the error-return emitted when binding fails. It is shared by named
// functions, anonymous literals, and type methods.
func (ctx *transpileContext) emitParameterBinding(name string, params []*ast.Parameter, callArgsName string, fnOnError errHandler) []jen.Code {
	var varArgsParam *ast.Parameter
	var kwArgsParam *ast.Parameter
	fixedParams := make([]*ast.Parameter, 0, len(params))
	for _, param := range params {
		switch {
		case param.VarArgs:
			varArgsParam = param
		case param.KwArgs:
			kwArgsParam = param
		default:
			fixedParams = append(fixedParams, param)
		}
	}

	fixedParamNames := make([]jen.Code, 0, len(fixedParams))
	for _, param := range fixedParams {
		fixedParamNames = append(fixedParamNames, jen.Lit(param.Name))
	}

	boundName := ctx.localName("bound")
	errVar := ctx.localName("err")
	varArgsName := ""
	if varArgsParam != nil {
		varArgsName = varArgsParam.Name
	}
	kwArgsName := ""
	if kwArgsParam != nil {
		kwArgsName = kwArgsParam.Name
	}

	stmts := []jen.Code{
		jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(name),
			jen.Index().String().Values(fixedParamNames...),
			jen.Lit(varArgsName),
			jen.Lit(kwArgsName),
			jen.Id(callArgsName),
		),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(fnOnError(errVar)),
		jen.Id("_").Op("=").Id(boundName),
	}

	emit := func(param *ast.Parameter) {
		stmts = append(stmts,
			jen.Var().Id(param.Name).Qual(pathObject, "Object").Op("=").Id(boundName).Index(jen.Lit(param.Name)),
			jen.Id("_").Op("=").Id(param.Name),
		)
	}
	for _, param := range fixedParams {
		emit(param)
	}
	if varArgsParam != nil {
		emit(varArgsParam)
	}
	if kwArgsParam != nil {
		emit(kwArgsParam)
	}

	return stmts
}

// buildFunctionValue emits an `&object.Function{...}` expression that wraps a
// closure binding the given parameters and running the given body. It is shared
// by named function definitions and anonymous function literals. name is used
// only for the runtime function's repr and for BindArguments diagnostics.
func (ctx *transpileContext) buildFunctionValue(name string, params []*ast.Parameter, body []ast.Statement) (*jen.Statement, error) {
	callArgsName := ctx.localName("callArgs")

	fnOnError := func(errVar string) jen.Code {
		return jen.Return(jen.List(jen.Nil(), jen.Id(errVar)))
	}

	argsDefine := ctx.emitParameterBinding(name, params, callArgsName, fnOnError)

	bodyCode, err := ctx.transpileStatements(body, fnOnError, "")
	if err != nil {
		return nil, err
	}

	bodyCode = append(argsDefine, bodyCode...)

	closure := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Id("error")),
	).Block(bodyCode...)

	return jen.Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(name),
		jen.Id("Fn").Op(":").Add(closure),
	), nil
}

func (ctx *transpileContext) transpileFunctionDefine(fn *ast.FunctionDefine, onError errHandler) ([]jen.Code, error) {
	funcValue, err := ctx.buildFunctionValue(fn.Name, fn.Parameters, fn.Body)
	if err != nil {
		return nil, err
	}

	result := jen.Var().Id(fn.Name).Qual(pathObject, "Object").Op("=").Add(funcValue)
	result.Op(";").Id("_").Op("=").Id(fn.Name)

	return []jen.Code{result}, nil
}

func (ctx *transpileContext) transpileFunctionLiteral(fn *ast.FunctionLiteral) ([]jen.Code, *jen.Statement, error) {
	funcValue, err := ctx.buildFunctionValue("<lambda>", fn.Parameters, fn.Body)
	if err != nil {
		return nil, nil, err
	}
	return nil, funcValue, nil
}

func (ctx *transpileContext) transpileTypeDefine(typeDef *ast.TypeDefine, onError errHandler) ([]jen.Code, error) {
	ctorVarName := typeDef.Name + "Constructor"
	ctx.moduleImports[typeDef.Name] = ctorVarName
	goTypeName := ctx.goTypeName(typeDef.Name)
	receiverName := strings.ToLower(typeDef.Name[:1])
	if receiverName == "_" {
		receiverName = "self"
	}

	structFields := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		structFields = append(structFields, jen.Id(field.Name).Qual(pathObject, "Object"))
	}

	ctx.topDecls = append(ctx.topDecls, jen.Type().Id(goTypeName).Struct(structFields...))

	reprFormat := fmt.Sprintf("<%s@%%p>", typeDef.Name)

	// A user type may customize protocol behavior (operators, iteration,
	// string/bool conversion, indexing) by defining a method with the matching
	// conventional name. When defined, the generated interface method delegates
	// to that method's wrapper; otherwise it falls back to a type error.
	defined := make(map[string]bool, len(typeDef.Methods))
	for _, m := range typeDef.Methods {
		defined[m.Name] = true
	}
	receiverParam := func() jen.Code { return jen.Id(receiverName).Op("*").Id(goTypeName) }
	errorf := func(format string) jen.Code {
		return jen.Qual("fmt", "Errorf").Call(jen.Lit(format), jen.Lit(typeDef.Name))
	}
	// protoCall builds `receiver.Wrapper(object.CallArgs{Positional: {args}})`.
	protoCall := func(goblinName string, args ...jen.Code) *jen.Statement {
		var callArgs jen.Code
		if len(args) == 0 {
			callArgs = jen.Qual(pathObject, "CallArgs").Values()
		} else {
			callArgs = jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
				jen.Id("Positional"): jen.Qual(pathObject, "Args").Values(args...),
			})
		}
		return jen.Id(receiverName).Dot(methodWrapperName(goblinName)).Call(callArgs)
	}
	protoDecls := make([]jen.Code, 0, 13)

	// String() string  <- "str"
	reprReturn := jen.Return(jen.Qual("fmt", "Sprintf").Call(jen.Lit(reprFormat), jen.Id(receiverName)))
	strDecl := jen.Func().Params(receiverParam()).Id("String").Params().String()
	if defined["__str"] {
		strDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__str")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(reprReturn),
			jen.Return(jen.Id("_res").Dot("String").Call()),
		)
	} else {
		strDecl.Block(reprReturn)
	}
	protoDecls = append(protoDecls, strDecl)

	// Repr() (string, error)  <- "__str" (error-propagating; object.Represented)
	reprReturnNil := jen.Return(jen.Qual("fmt", "Sprintf").Call(jen.Lit(reprFormat), jen.Id(receiverName)), jen.Nil())
	reprDecl := jen.Func().Params(receiverParam()).Id("Repr").Params().Parens(jen.List(jen.String(), jen.Error()))
	if defined["__str"] {
		reprDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__str")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.Lit(""), jen.Id("_err"))),
			jen.Return(jen.Id("_res").Dot("String").Call(), jen.Nil()),
		)
	} else {
		reprDecl.Block(reprReturnNil)
	}
	protoDecls = append(protoDecls, reprDecl)

	// Bool() bool  <- "__bool"
	boolDecl := jen.Func().Params(receiverParam()).Id("Bool").Params().Bool()
	if defined["__bool"] {
		boolDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__bool")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.True())),
			jen.Return(jen.Id("_res").Dot("Bool").Call()),
		)
	} else {
		boolDecl.Block(jen.Return(jen.True()))
	}
	protoDecls = append(protoDecls, boolDecl)

	// Truthy() (bool, error)  <- "__bool" (error-propagating; object.Truthful)
	truthyDecl := jen.Func().Params(receiverParam()).Id("Truthy").Params().Parens(jen.List(jen.Bool(), jen.Error()))
	if defined["__bool"] {
		truthyDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__bool")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.False(), jen.Id("_err"))),
			jen.Return(jen.Id("_res").Dot("Bool").Call(), jen.Nil()),
		)
	} else {
		truthyDecl.Block(jen.Return(jen.True(), jen.Nil()))
	}
	protoDecls = append(protoDecls, truthyDecl)

	// Compare(other) (int, error)  <- "__cmp" (returns Int -1/0/1)
	cmpDecl := jen.Func().Params(receiverParam()).Id("Compare").Params(
		jen.Id("other").Qual(pathObject, "Object"),
	).Parens(jen.List(jen.Int(), jen.Error()))
	if defined["__cmp"] {
		cmpDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__cmp", jen.Id("other"))),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.Lit(0), jen.Id("_err"))),
			jen.List(jen.Id("_i"), jen.Id("_ok")).Op(":=").Id("_res").Assert(jen.Qual(pathObject, "Integer")),
			jen.If(jen.Op("!").Id("_ok")).Block(
				jen.Return(jen.Lit(0), errorf("%s.__cmp must return Int")),
			),
			jen.Return(jen.Int().Parens(jen.Id("_i")), jen.Nil()),
		)
	} else {
		cmpDecl.Block(jen.Return(jen.Lit(0), errorf("cannot compare %s")))
	}
	protoDecls = append(protoDecls, cmpDecl)

	// Binary operators (other) (Object, error)
	binOps := []struct{ goMethod, goblin, errFmt string }{
		{"Add", "__add", "cannot add %s"},
		{"Minus", "__sub", "cannot subtract %s"},
		{"Multiply", "__mul", "cannot multiply %s"},
		{"Divide", "__div", "cannot divide %s"},
		{"And", "__and", "cannot perform AND on %s"},
		{"Or", "__or", "cannot perform OR on %s"},
	}
	for _, op := range binOps {
		d := jen.Func().Params(receiverParam()).Id(op.goMethod).Params(
			jen.Id("other").Qual(pathObject, "Object"),
		).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error()))
		if defined[op.goblin] {
			d.Block(jen.Return(protoCall(op.goblin, jen.Id("other"))))
		} else {
			d.Block(jen.Return(jen.Nil(), errorf(op.errFmt)))
		}
		protoDecls = append(protoDecls, d)
	}

	// Not() (Object, error)  <- "not"
	notDecl := jen.Func().Params(receiverParam()).Id("Not").Params().Parens(
		jen.List(jen.Qual(pathObject, "Object"), jen.Error()),
	)
	if defined["__not"] {
		notDecl.Block(jen.Return(protoCall("__not")))
	} else {
		notDecl.Block(jen.Return(jen.Nil(), errorf("cannot perform NOT on %s")))
	}
	protoDecls = append(protoDecls, notDecl)

	// Iter() ([]Object, error)  <- "iter" (returns an iterable)
	iterDecl := jen.Func().Params(receiverParam()).Id("Iter").Params().Parens(
		jen.List(jen.Index().Qual(pathObject, "Object"), jen.Error()),
	)
	if defined["__iter"] {
		iterDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__iter")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.Nil(), jen.Id("_err"))),
			jen.Return(jen.Id("_res").Dot("Iter").Call()),
		)
	} else {
		iterDecl.Block(jen.Return(jen.Nil(), errorf("%s does not support iteration")))
	}
	protoDecls = append(protoDecls, iterDecl)

	// Index(index) (Object, error)  <- "get_item"
	indexDecl := jen.Func().Params(receiverParam()).Id("Index").Params(
		jen.Id("index").Qual(pathObject, "Object"),
	).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error()))
	if defined["__getitem"] {
		indexDecl.Block(jen.Return(protoCall("__getitem", jen.Id("index"))))
	} else {
		indexDecl.Block(jen.Return(jen.Nil(), errorf("%s is not indexable")))
	}
	protoDecls = append(protoDecls, indexDecl)

	// SetIndex(index, value) error  <- "set_item" (object.IndexSetter)
	setIndexDecl := jen.Func().Params(receiverParam()).Id("SetIndex").Params(
		jen.Id("index").Qual(pathObject, "Object"), jen.Id("value").Qual(pathObject, "Object"),
	).Error()
	if defined["__setitem"] {
		setIndexDecl.Block(
			jen.List(jen.Id("_"), jen.Id("_err")).Op(":=").Add(protoCall("__setitem", jen.Id("index"), jen.Id("value"))),
			jen.Return(jen.Id("_err")),
		)
	} else {
		setIndexDecl.Block(jen.Return(errorf("%s does not support index assignment")))
	}
	protoDecls = append(protoDecls, setIndexDecl)

	ctx.topDecls = append(ctx.topDecls, protoDecls...)

	getAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+len(typeDef.Methods)+1)
	for _, field := range typeDef.Fields {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Return(jen.Id(receiverName).Dot(field.Name), jen.Nil()),
			),
		)
	}
	for _, method := range typeDef.Methods {
		wrapperName := methodWrapperName(method.Name)
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit(method.Name)).Block(
				jen.Return(
					jen.Op("&").Qual(pathObject, "Function").Values(
						jen.Id("Name").Op(":").Lit(method.Name),
						jen.Id("Fn").Op(":").Id(receiverName).Dot(wrapperName),
					),
					jen.Nil(),
				),
			),
		)
	}
	userDefinedConstructor := false
	for _, field := range typeDef.Fields {
		if field.Name == "constructor" {
			userDefinedConstructor = true
			break
		}
	}
	if !userDefinedConstructor {
		for _, method := range typeDef.Methods {
			if method.Name == "constructor" {
				userDefinedConstructor = true
				break
			}
		}
	}
	if !userDefinedConstructor {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit("constructor")).Block(
				jen.Return(jen.Id(ctorVarName), jen.Nil()),
			),
		)
	}
	getAttrCases = append(getAttrCases,
		jen.Default().Block(
			jen.Return(
				jen.Nil(),
				jen.Qual(pathObject, "NewAttributeError").Call(jen.Lit("%s has no attribute '%s'"), jen.Lit(typeDef.Name), jen.Id("name")),
			),
		),
	)

	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("GetAttr").Params(
			jen.Id("name").String(),
		).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(
			jen.Switch(jen.Id("name")).Block(getAttrCases...),
		),
	)

	setAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+1)
	for _, field := range typeDef.Fields {
		setAttrCases = append(setAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Id(receiverName).Dot(field.Name).Op("=").Id("value"),
				jen.Return(jen.Nil()),
			),
		)
	}
	setAttrCases = append(setAttrCases,
		jen.Default().Block(
			jen.Return(
				jen.Qual(pathObject, "NewAttributeError").Call(jen.Lit("%s has no attribute '%s'"), jen.Lit(typeDef.Name), jen.Id("name")),
			),
		),
	)
	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("SetAttr").Params(
			jen.Id("name").String(), jen.Id("value").Qual(pathObject, "Object"),
		).Error().Block(
			jen.Switch(jen.Id("name")).Block(setAttrCases...),
		),
	)

	for _, method := range typeDef.Methods {
		wrapperName := methodWrapperName(method.Name)

		callArgsName := ctx.localName("callArgs")
		fnOnError := func(errVar string) jen.Code {
			return jen.Return(jen.List(jen.Nil(), jen.Id(errVar)))
		}

		bodyPrefix := []jen.Code{
			jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
			jen.Id("_").Op("=").Id("builtin"),
		}
		bodyPrefix = append(bodyPrefix,
			ctx.emitParameterBinding(typeDef.Name+"."+method.Name, method.Parameters[1:], callArgsName, fnOnError)...,
		)
		bodyPrefix = append(bodyPrefix,
			jen.Var().Id("self").Qual(pathObject, "Object").Op("=").Id(receiverName),
			jen.Id("_").Op("=").Id("self"),
		)

		methodBody, err := ctx.transpileStatements(method.Body, fnOnError, "")
		if err != nil {
			return nil, err
		}

		ctx.topDecls = append(ctx.topDecls,
			jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id(wrapperName).Params(
				jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
			).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(
				append(bodyPrefix, methodBody...)...,
			),
		)
	}

	callArgsName := ctx.localName("callArgs")
	shadowPositionalName := ctx.localName("positional")
	shadowKeywordName := ctx.localName("keyword")
	enrichedCallArgsName := ctx.localName("enriched")
	boundName := ctx.localName("bound")
	errVar := ctx.localName("err")

	constructorSetup := []jen.Code{
		jen.Id(shadowPositionalName).Op(":=").Append(jen.Qual(pathObject, "Args").Values(), jen.Id(callArgsName).Dot("Positional").Op("...")),
		jen.Id(shadowKeywordName).Op(":=").Qual(pathObject, "Kwargs").Values(),
	}
	constructorSetup = append(constructorSetup,
		jen.For(jen.List(jen.Id("_key"), jen.Id("_value")).Op(":=").Op("range").Id(callArgsName).Dot("Keyword")).Block(
			jen.Id(shadowKeywordName).Index(jen.Id("_key")).Op("=").Id("_value"),
		),
	)

	for index, field := range typeDef.Fields {
		if !field.HasDefault() {
			continue
		}
		defaultPre, defaultValue, err := ctx.transpileExpression(field.DefaultValue, onError)
		if err != nil {
			return nil, err
		}
		constructorSetup = append(constructorSetup,
			jen.If(
				jen.Len(jen.Id(shadowPositionalName)).Op("<=").Lit(index),
			).BlockFunc(func(group *jen.Group) {
				group.List(jen.Id("_"), jen.Id("_has_"+field.Name)).Op(":=").Id(shadowKeywordName).Index(jen.Lit(field.Name))
				group.If(jen.Op("!").Id("_has_" + field.Name)).Block(append(defaultPre,
					jen.Id(shadowKeywordName).Index(jen.Lit(field.Name)).Op("=").Add(defaultValue),
				)...)
			}),
		)
	}

	fieldNames := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNames = append(fieldNames, jen.Lit(field.Name))
	}

	constructorSetup = append(constructorSetup,
		jen.Id(enrichedCallArgsName).Op(":=").Qual(pathObject, "CallArgs").Values(jen.Dict{
			jen.Id("Positional"): jen.Id(shadowPositionalName),
			jen.Id("Keyword"):    jen.Id(shadowKeywordName),
		}),
		jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(typeDef.Name),
			jen.Index().String().Values(fieldNames...),
			jen.Lit(""),
			jen.Lit(""),
			jen.Id(enrichedCallArgsName),
		),
		jen.Id("_").Op("=").Id(boundName),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(
			jen.Return(jen.Nil(), jen.Id(errVar)),
		),
	)

	instanceValues := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		instanceValues = append(instanceValues,
			jen.Id(field.Name).Op(":").Id(boundName).Index(jen.Lit(field.Name)),
		)
	}

	constructorBody := append(constructorSetup,
		jen.Id("_instance").Op(":=").Op("&").Id(goTypeName).Values(instanceValues...),
		jen.Return(jen.Id("_instance"), jen.Nil()),
	)

	constructorClosure := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(
		jen.List(jen.Qual(pathObject, "Object"), jen.Error()),
	).Block(constructorBody...)

	ctx.topDecls = append(ctx.topDecls,
		jen.Var().Id(ctorVarName).Qual(pathObject, "Object"),
	)

	constructor := jen.Id(ctorVarName).Op("=").Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(typeDef.Name),
		jen.Id("Fn").Op(":").Add(constructorClosure),
	)

	return []jen.Code{constructor}, nil
}

func (ctx *transpileContext) transpileReturn(return_ *ast.Return, onError errHandler) ([]jen.Code, error) {
	preStmts, r, err := ctx.transpileExpression(return_.Value, onError)
	if err != nil {
		return nil, err
	}
	return append(preStmts, jen.Return(jen.List(r, jen.Nil()))), nil
}

// transpileRaise turns `raise expr` into an error (the raised Error value, or a
// type error if expr is not an Error) that is then routed through the active
// error handler (a top-level `return`, or a jump to the enclosing `try`'s catch
// label).
func (ctx *transpileContext) transpileRaise(raise *ast.Raise, onError errHandler) ([]jen.Code, error) {
	preStmts, val, err := ctx.transpileExpression(raise.Value, onError)
	if err != nil {
		return nil, err
	}
	errVar := ctx.localName("err")
	code := append(preStmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "Raise").Call(val),
		onError(errVar),
	)
	return code, nil
}

// transpileTryCatch lowers `try { ... } catch e { ... }` using goto/labels so
// that `return`/`break`/`continue` inside the try body still target the real
// enclosing function/loop (a closure or defer/recover boundary would capture
// them). Inside the try body the error handler stores the error and jumps to
// the catch label; the unconditional post-block check guarantees the label is
// referenced even when the body has no fallible operation.
func (ctx *transpileContext) transpileTryCatch(tc *ast.TryCatch, onError errHandler) ([]jen.Code, error) {
	excVar := ctx.localName("exc")
	catchLabel := ctx.localName("catch")
	doneLabel := ctx.localName("done")

	tryOnError := func(errVar string) jen.Code {
		return jen.Block(
			jen.Id(excVar).Op("=").Id(errVar),
			jen.Goto().Id(catchLabel),
		)
	}

	tryBody, err := ctx.transpileStatements(tc.TryBody, tryOnError, "")
	if err != nil {
		return nil, err
	}
	catchBody, err := ctx.transpileStatements(tc.CatchBody, onError, "")
	if err != nil {
		return nil, err
	}

	catchBlock := []jen.Code{
		jen.Id(tc.CatchVar).Op(":=").Qual(pathObject, "ExcValue").Call(jen.Id(excVar)),
		jen.Id("_").Op("=").Id(tc.CatchVar),
	}
	catchBlock = append(catchBlock, catchBody...)

	return []jen.Code{
		jen.Var().Id(excVar).Error(),
		jen.Block(tryBody...),
		jen.If(jen.Id(excVar).Op("!=").Nil()).Block(jen.Goto().Id(catchLabel)),
		jen.Goto().Id(doneLabel),
		jen.Id(catchLabel).Op(":"),
		jen.Block(catchBlock...),
		jen.Id(doneLabel).Op(":"),
		jen.Id("_").Op("=").Id(excVar),
	}, nil
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
	if operation.Operator == ast.And || operation.Operator == ast.Or {
		return ctx.transpileLogicalOperation(operation, onError)
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

// transpileLogicalOperation lowers && and || with short-circuit evaluation: the
// RHS is only evaluated (and may therefore only error or produce side effects)
// when the LHS does not already fix the result. The result is always a Bool,
// matching the previous truthiness-based behavior.
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
	rhsVar := ctx.localName("rhs")
	rhsTruthyVar := ctx.localName("cond")
	rhsErrVar := ctx.localName("err")

	rhsBlock := append([]jen.Code{}, rhsPre...)
	rhsBlock = append(rhsBlock,
		jen.Id(rhsVar).Op(":=").Add(rhs),
		jen.List(jen.Id(rhsTruthyVar), jen.Id(rhsErrVar)).Op(":=").Qual(pathObject, "Truthy").Call(jen.Id(rhsVar)),
		jen.If(jen.Id(rhsErrVar).Op("!=").Nil()).Block(onError(rhsErrVar)),
		jen.Id(tmpVar).Op("=").Qual(pathObject, "Bool").Call(jen.Id(rhsTruthyVar)),
	)

	preStmts := append([]jen.Code{}, lhsPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(lhsTruthyVar), jen.Id(errVar)).Op(":=").Qual(pathObject, "Truthy").Call(lhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.Var().Id(tmpVar).Qual(pathObject, "Object"),
	)

	if operation.Operator == ast.And {
		preStmts = append(preStmts,
			jen.Id(tmpVar).Op("=").Qual(pathObject, "False"),
			jen.If(jen.Id(lhsTruthyVar)).Block(rhsBlock...),
		)
	} else { // ast.Or
		preStmts = append(preStmts,
			jen.Id(tmpVar).Op("=").Qual(pathObject, "True"),
			jen.If(jen.Op("!").Id(lhsTruthyVar)).Block(rhsBlock...),
		)
	}

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
		call = operand.Dot("Not").Call()
	case "+":
		call = jen.Qual(pathObject, "Integer").Call(jen.Lit(0)).Dot("Add").Call(operand)
	case "-":
		call = jen.Qual(pathObject, "Integer").Call(jen.Lit(0)).Dot("Minus").Call(operand)
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

func (ctx *transpileContext) transpileExport(export *ast.Export, exportsVar string) ([]jen.Code, error) {
	return []jen.Code{
		jen.Id(exportsVar).Index(jen.Lit(export.Name)).Op("=").Id(export.Name),
	}, nil
}

// lineDirective returns a jen.Code that renders as a Go `//line` directive
// pointing back to the original .goblin source location. Returns nil if the
// position lacks file context (zero-value Pos, synthetic nodes), in which
// case jen will skip it during rendering.
func lineDirective(pos token.Pos) jen.Code {
	src, ok := pos.Context.(token.Sourcer)
	if !ok || src == nil {
		return nil
	}
	path := src.Source()
	if path == "" || pos.Line <= 0 {
		return nil
	}
	// `go build` runs in a temp dir; absolutize so diagnostics resolve.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	col := pos.Column
	if col <= 0 {
		col = 1
	}
	return jen.Comment(fmt.Sprintf("//line %s:%d:%d", path, pos.Line, col))
}

func (ctx *transpileContext) transpileStatement(stmt ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	// Import produces no output; attaching a //line directive would orphan
	// it onto the following statement and skew its mapped line number.
	if _, isImport := stmt.(*ast.Import); isImport {
		return nil, nil
	}

	var prelude []jen.Code
	if d := lineDirective(stmt.Position()); d != nil {
		prelude = append(prelude, d)
	}

	var codes []jen.Code
	var err error
	switch v := stmt.(type) {
	case *ast.Declare:
		codes, err = ctx.transpileDeclare(v, onError)
	case *ast.Assign:
		codes, err = ctx.transpileAssign(v, onError)
	case *ast.SetIndex:
		codes, err = ctx.transpileSetIndex(v, onError)
	case *ast.SetAttr:
		codes, err = ctx.transpileSetAttr(v, onError)
	case *ast.FunctionCall:
		var argPreStmts []jen.Code
		var call *jen.Statement
		argPreStmts, call, err = ctx.transpileFunctionCall(v, onError)
		if err == nil {
			errVar := ctx.localName("err")
			codes = append(argPreStmts,
				jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			)
		}
	case *ast.CallExpression:
		var argPreStmts []jen.Code
		var call *jen.Statement
		argPreStmts, call, err = ctx.transpileCallExpression(v, onError)
		if err == nil {
			errVar := ctx.localName("err")
			codes = append(argPreStmts,
				jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			)
		}
	case *ast.FunctionDefine:
		codes, err = ctx.transpileFunctionDefine(v, onError)
	case *ast.TypeDefine:
		codes, err = ctx.transpileTypeDefine(v, onError)
	case *ast.IfElse:
		codes, err = ctx.transpileIfElse(v, onError)
	case *ast.While:
		codes, err = ctx.transpileWhile(v, onError)
	case *ast.For:
		codes, err = ctx.transpileFor(v, onError)
	case *ast.Break:
		codes, err = ctx.transpileBreak(v)
	case *ast.Continue:
		codes, err = ctx.transpileContinue(v)
	case *ast.Return:
		codes, err = ctx.transpileReturn(v, onError)
	case *ast.Raise:
		codes, err = ctx.transpileRaise(v, onError)
	case *ast.TryCatch:
		codes, err = ctx.transpileTryCatch(v, onError)
	case *ast.Export:
		codes, err = ctx.transpileExport(v, exportsVar)
	case *ast.BinaryOperation:
		var pre []jen.Code
		pre, _, err = ctx.transpileBinaryOperation(v, onError)
		codes = pre
	case *ast.UnaryOperation:
		var pre []jen.Code
		pre, _, err = ctx.transpileUnaryOperation(v, onError)
		codes = pre
	case *ast.MemberExpression:
		var pre []jen.Code
		pre, _, err = ctx.transpileMemberExpression(v, onError)
		codes = pre
	default:
		return nil, object.NotImplementedError
	}
	if err != nil {
		return nil, err
	}
	return append(prelude, codes...), nil
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
			"module %s\n\ngo 1.19\n\nrequire (\n\tgithub.com/aisk/goblin %s\n\tgithub.com/pkg/errors v0.9.1\n)\n\nreplace github.com/aisk/goblin => %s\n",
			moduleName, runtimeVersion, goblinRoot,
		)
	}
	return fmt.Sprintf(
		"module %s\n\ngo 1.19\n\nrequire (\n\tgithub.com/aisk/goblin %s\n\tgithub.com/pkg/errors v0.9.1\n)\n",
		moduleName, runtimeVersion,
	)
}

// TranspileToDir transpiles a goblin module into a Go module directory structure.
// The entry-point module becomes output/main.go; each imported path module becomes
// its own package under outputDir.
func TranspileToDir(mod *ast.Module, sourceFile, outputDir string) error {
	if err := semantic.CheckModule(mod); err != nil {
		return err
	}

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

	l, err := lexer.NewLexerFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read module %s: %v", importPath, err)
	}
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		return fmt.Errorf("parse error in module %s: %v", importPath, err)
	}

	mod, ok := st.(*ast.Module)
	if !ok {
		return fmt.Errorf("internal error: unexpected AST type for module %s", importPath)
	}
	if err := semantic.CheckModule(mod); err != nil {
		return fmt.Errorf("semantic error in module %s: %v", importPath, err)
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

	savedTopDecls := ctx.topDecls
	ctx.topDecls = nil
	defer func() { ctx.topDecls = savedTopDecls }()

	exportsVar := ctx.localName("exports")
	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Qual("github.com/pkg/errors", "WithStack").Call(jen.Id(errVar)))
	}

	stmts, err := ctx.transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return fmt.Errorf("transpile error in module %s: %v", importPath, err)
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
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
					jen.Qual(info.executorPath, info.executorFunc),
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

	savedTopDecls := ctx.topDecls
	ctx.topDecls = nil
	defer func() { ctx.topDecls = savedTopDecls }()

	exportsVar := ctx.localName("exports")
	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Qual("github.com/pkg/errors", "WithStack").Call(jen.Id(errVar)))
	}

	stmts, err := ctx.transpileStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return err
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
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
					jen.Qual(info.executorPath, info.executorFunc),
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
			jen.Qual("fmt", "Fprintf").Call(jen.Qual("os", "Stderr"), jen.Lit("%+v\n"), jen.Id("err")),
			jen.Qual("os", "Exit").Call(jen.Lit(1)),
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
