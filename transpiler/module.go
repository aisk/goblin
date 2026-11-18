package transpiler

import (
	"fmt"
	"path/filepath"

	"github.com/aisk/goblin/ast"
	"github.com/dave/jennifer/jen"
)

// newGenerator prepares the generation of mod, read from absPath. key is the
// module's key in the program and label its import path, both empty for the
// entry module.
func newGenerator(p *program, mod *ast.Module, absPath, key, label string) *generator {
	g := &generator{
		prog:       p,
		mod:        mod,
		key:        key,
		label:      label,
		baseDir:    filepath.Dir(absPath),
		moduleVars: map[string]string{},
	}
	// Imported modules live in package-level variables, so package-level
	// functions can reach them too.
	for _, imp := range moduleImports(mod) {
		if imp.IsPathImport() {
			g.moduleVars[imp.Name] = "_mod_" + keyToIdent(p.moduleKey(resolveImport(g.baseDir, imp.Path)))
		} else {
			g.moduleVars[imp.Name] = "_" + moduleVarName(imp.Path)
		}
	}
	return g
}

// file renders the module as a Go file. The entry module is package main
// with its own registry and a main function; an imported module is a package
// whose Execute takes the program's registry.
func (g *generator) file() (*jen.File, error) {
	entry := g.key == ""
	pkg := "main"
	if !entry {
		pkg = packageName(g.key)
	}
	f := jen.NewFile(pkg)
	f.Var().Id("builtin").Op("=").Qual(pathExtension, "BuiltinsModule")
	imports := moduleImports(g.mod)
	for _, imp := range imports {
		if imp.IsPathImport() {
			key := g.prog.moduleKey(resolveImport(g.baseDir, imp.Path))
			f.ImportAlias(g.prog.importPath(key), "_pkg_"+keyToIdent(key))
		}
	}
	var params []jen.Code
	if entry {
		if len(imports) > 0 {
			f.Var().Id("_registry").Op("=").Qual(pathObject, "NewRegistry").Call()
		}
	} else {
		params = append(params, jen.Id("_registry").Op("*").Qual(pathObject, "Registry"))
	}

	body, err := g.executeBody()
	if err != nil {
		if !entry {
			return nil, fmt.Errorf("transpile error in module %s: %v", g.label, err)
		}
		return nil, err
	}
	for _, decl := range g.topDecls {
		f.Add(decl)
	}
	f.Func().Id("Execute").Params(params...).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(body...)
	if entry {
		f.Func().Id("main").Params().Block(mainBody()...)
	}
	return f, nil
}

// executeBody builds the body of the module's Execute function: the exports
// prologue, registry Load calls for its imports, the module statements, and
// the module-value return. Module state (imported module values, module
// variables, function values) lives in package-level variables declared in
// topDecls; Execute only assigns them, in source order.
func (g *generator) executeBody() ([]jen.Code, error) {
	mod := g.mod
	g.exportsVar = g.localName("exports")
	g.assigned = ast.AssignedNames(mod.Body)

	onError := func(errVar string) jen.Code {
		return tracedReturn(jen.Nil(), errVar, sourceModuleName(modulePosition(mod)), "<module>", g.framePos(modulePosition(mod)))
	}

	stmts, err := g.transpileModuleStatements(mod.Body, onError)
	if err != nil {
		return nil, err
	}

	body := []jen.Code{
		jen.Id(g.exportsVar).Op(":=").Op("&").Qual(pathObject, "Module").Values(
			jen.Id("Members").Op(":").Map(jen.String()).Qual(pathObject, "Object").Values(),
		),
	}
	// loadModule declares the package-level variable holding a loaded module
	// and emits its registry Load call.
	loadModule := func(varName string, loadArgs ...jen.Code) {
		g.topDecls = append(g.topDecls, jen.Var().Id(varName).Qual(pathObject, "Object"))
		errVar := g.localName("err")
		body = append(body,
			jen.Var().Id(errVar).Error(),
			jen.List(jen.Id(varName), jen.Id(errVar)).Op("=").Id("_registry").Dot("Load").Call(loadArgs...),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
	}
	imports := moduleImports(mod)
	for _, imp := range imports {
		if !imp.IsPathImport() {
			loadModule(g.moduleVars[imp.Name], jen.Lit(imp.Path), jen.Qual(moduleExecutorPath(imp.Path), "Execute"))
		}
	}
	for _, imp := range imports {
		if imp.IsPathImport() {
			key := g.prog.moduleKey(resolveImport(g.baseDir, imp.Path))
			// The loader calls the imported package's Execute with the
			// program's shared registry.
			loader := jen.Func().Params().Parens(jen.List(
				jen.Qual(pathObject, "Object"), jen.Error(),
			)).Block(
				jen.Return(jen.Qual(g.prog.importPath(key), "Execute").Call(jen.Id("_registry"))),
			)
			loadModule(g.moduleVars[imp.Name], jen.Lit(key), loader)
		}
	}

	body = append(body, stmts...)
	body = append(body, jen.Return(jen.Id(g.exportsVar), jen.Nil()))
	return body, nil
}

// transpileExport adds a name to the module value. A name the module assigns
// somewhere is exported as a live binding, a function that reads the
// package-level variable when the member is read, so importers see later
// assignments; the interpreter's exportModule makes the same split. Any
// other name keeps the value it has here, which is final: functions and
// types are hoisted above the body, and a variable is declared before the
// export statement can name it.
func (g *generator) transpileExport(export *ast.Export, onError errHandler) ([]jen.Code, error) {
	if g.assigned[export.Name] {
		pre, value, err := g.transpileExpression(&ast.Identifier{Name: export.Name}, onError)
		if err != nil {
			return nil, err
		}
		if len(pre) > 0 {
			return nil, fmt.Errorf("export %s: reading the variable needs statements", export.Name)
		}
		return []jen.Code{
			jen.Id(g.exportsVar).Dot("Bind").Call(jen.Lit(export.Name), jen.Func().Params().Qual(pathObject, "Object").Block(jen.Return(value))),
		}, nil
	}
	// A type's name is its Go struct; the value to export is the constructor
	// variable, which moduleBinding maps it to (imported modules likewise).
	value := jen.Id(export.Name)
	if mapped, ok := g.moduleBinding(export.Name); ok {
		value = jen.Id(mapped)
	}
	return []jen.Code{
		jen.Id(g.exportsVar).Dot("Members").Index(jen.Lit(export.Name)).Op("=").Add(value),
	}, nil
}

// transpileModuleStatements transpiles a module body with hoisting, mirroring
// the interpreter, which registers all top-level function and type definitions
// before executing the module body. Every module-level binding is declared as
// a package-level variable (initialized to nil), all function values are
// assigned next, and the remaining statements run in source order. This makes
// forward references — including mutually recursive functions — work in
// generated code, and it puts module state where the other package-level
// definitions (type methods, direct functions) can reach it: those are plain
// top-level Go functions, not closures over Execute's locals. Type
// constructor names are pre-registered for the same reason.
func (g *generator) transpileModuleStatements(stmts []ast.Statement, onError errHandler) ([]jen.Code, error) {
	// A module's top level is a function body too — it becomes Execute() — so
	// its locals are eligible for the same specialisation.
	defer g.pushUserScope()()

	g.moduleScopeIdx = len(g.userScopes) - 1

	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case *ast.TypeDefine:
			g.moduleVars[v.Name] = v.Name + "Constructor"
		case *ast.FunctionDefine:
			// Function names are hoisted (mirroring the interpreter), so
			// they shadow built-ins for the whole module body.
			g.declareUserName(v.Name)
		}
	}

	// Direct-call lowering candidates are fixed before any body transpiles, so
	// recursive and forward calls lower too. Hoisted names are declared above
	// for the same reason, and the module body's type scope is entered after both so the inference
	// pass sees the final shadowing picture.
	//
	// Whether `range` means the builtin is decided once for the whole module:
	// the signature inference types every body up front, and it has to see
	// exactly the environments the generator will, so both work from this one
	// answer. A module that binds the name `range` anywhere gives up range
	// lowering everywhere.
	_, rangeImported := g.moduleVars["range"]
	g.rangeNative = !rangeImported && !g.isUserName("range") && !declaresNameAnywhere(stmts, "range")
	g.directFns, g.directCtors = g.collectDirectFns(stmts)
	g.sigs = inferSignatures(stmts, g.directFns, g.rangeNative)
	defer g.enterFunction(nil, stmts, nil, tyDynamic, nil)()
	g.nameBody(sourceModuleName(modulePosition(g.mod)), "<module>")

	var funcAssigns, body []jen.Code
	declare := func(name string) {
		g.topDecls = append(g.topDecls,
			jen.Var().Id(name).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Nil"))
	}
	// moduleDeclare splits `var x = expr` into a package-level declaration
	// and an in-place assignment so functions may reference the variable
	// regardless of where their definition appears in the source. A
	// specialised variable is declared with its native type; it can never be
	// captured by a closure, since being named inside a nested scope is
	// exactly what disqualifies it from specialisation.
	moduleDeclare := func(v *ast.Declare) ([]jen.Code, error) {
		if t := g.typeOf(v.Name); t.native() {
			g.topDecls = append(g.topDecls, jen.Var().Id(v.Name).Id(goTypeOf(t)))

			pre, value, err := g.emitNative(v.Value, onError)
			if err != nil {
				return nil, err
			}
			g.declareUserName(v.Name)
			return append(pre, jen.Id(v.Name).Op("=").Add(value)), nil
		}

		declare(v.Name)
		preStmts, value, err := g.transpileExpression(v.Value, onError)
		if err != nil {
			return nil, err
		}
		g.declareUserName(v.Name)
		return append(preStmts, jen.Id(v.Name).Op("=").Add(value)), nil
	}
	savedPos := g.errorPos
	defer func() { g.errorPos = savedPos }()
	for _, stmt := range stmts {
		g.errorPos = stmt.Position()
		switch v := stmt.(type) {
		case *ast.FunctionDefine:
			declare(v.Name)
			if info, ok := g.directFns[v.Name]; ok {
				assigns, err := g.buildDirectFunction(info, v)
				if err != nil {
					return nil, err
				}
				funcAssigns = append(funcAssigns, assigns...)
				continue
			}
			funcValue, err := g.buildFunctionValue(v.Name, v.Position(), v.Parameters, v.Body)
			if err != nil {
				return nil, err
			}
			funcAssigns = append(funcAssigns, jen.Id(v.Name).Op("=").Add(funcValue))
		case *ast.TypeDefine:
			codes, err := g.transpileTypeDefine(v, onError)
			if err != nil {
				return nil, err
			}
			funcAssigns = append(funcAssigns, codes...)
		case *ast.Declare:
			codes, err := moduleDeclare(v)
			if err != nil {
				return nil, err
			}
			body = append(body, codes...)
		case *ast.Destructure:
			// The names `var a, b = value` declares are module variables
			// like any other.
			codes, err := g.transpileDestructure(v, onError, func(store ast.Statement) ([]jen.Code, error) {
				if decl, ok := store.(*ast.Declare); ok {
					return moduleDeclare(decl)
				}
				return g.transpileStatement(store, onError)
			})
			if err != nil {
				return nil, err
			}
			body = append(body, codes...)
		default:
			codes, err := g.transpileStatement(stmt, onError)
			if err != nil {
				return nil, err
			}
			body = append(body, codes...)
		}
	}
	if g.body.defers != "" {
		// What the module body defers runs when the body ends, before
		// Execute goes on to return the module value. The definitions
		// hoisted above only bind names, so they stay outside.
		errVar := g.localName("err")
		body = []jen.Code{
			jen.Var().Id(g.body.defers).Qual(pathObject, "Defers"),
			jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Func().Params().Parens(jen.List(
				jen.Qual(pathObject, "Object"), jen.Error(),
			)).Block(append(body, jen.Return(jen.Nil(), jen.Nil()))...).Call(),
			jen.If(
				jen.Id(errVar).Op("=").Id(g.body.defers).Dot("Run").Call(jen.Id(errVar)),
				jen.Id(errVar).Op("!=").Nil(),
			).Block(jen.Return(jen.Nil(), jen.Id(errVar))),
		}
	}
	return append(funcAssigns, body...), nil
}
