package transpiler

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/dave/jennifer/jen"
)

// emitExecuteBody builds the body of a module executor function: the exports
// prologue, registry Load calls for stdlib and path imports, the transpiled
// module statements, and the module-value return. Module state — imported
// module values, module variables, function values — lives in package-level
// variables (declared via ctx.topDecls); the executor only assigns them, in
// source order. The caller must already have swapped ctx.moduleImports to
// imports. pathLoader supplies the loader argument for path-import Load
// calls. When moduleErrPrefix is non-empty, transpile errors are wrapped as
// "transpile error in module <prefix>: ..."; otherwise they are returned bare.
func (ctx *transpileContext) emitExecuteBody(mod *ast.Module, imports map[string]string, moduleErrPrefix string, pathLoader func(key string) jen.Code) ([]jen.Code, error) {
	exportsVar := ctx.localName("exports")

	onError := func(errVar string) jen.Code {
		return tracedReturn(jen.Nil(), errVar, sourceModuleName(modulePosition(mod)), "<module>", ctx.framePos(modulePosition(mod)))
	}

	stmts, err := ctx.transpileModuleStatements(mod.Body, onError, exportsVar)
	if err != nil {
		if moduleErrPrefix != "" {
			return nil, fmt.Errorf("transpile error in module %s: %v", moduleErrPrefix, err)
		}
		return nil, err
	}

	body := []jen.Code{
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// loadModule declares the package-level variable holding a loaded module
	// (once per generated file — the same module imported elsewhere reuses it)
	// and emits its registry Load call. Reloading an already-loaded module is
	// a registry cache hit, so a shared variable is reassigned the same value.
	loadModule := func(varName string, loadArgs ...jen.Code) {
		if _, declared := ctx.loadedModuleVars[varName]; !declared {
			ctx.loadedModuleVars[varName] = struct{}{}
			ctx.topDecls = append(ctx.topDecls, jen.Var().Id(varName).Qual(pathObject, "Object"))
		}
		errVar := ctx.localName("err")
		body = append(body,
			jen.Var().Id(errVar).Error(),
			jen.List(jen.Id(varName), jen.Id(errVar)).Op("=").Id("_registry").Dot("Load").Call(loadArgs...),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
	}

	// Builtin module imports via registry
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || imp.IsPathImport() {
			continue
		}
		loadModule(imports[imp.Name], jen.Lit(imp.Path), jen.Qual(moduleExecutorPath(imp.Path), "Execute"))
	}

	// Path module imports via registry
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !imp.IsPathImport() {
			continue
		}
		absPath, err := ctx.resolveImportPath(imp.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %v", imp.Path, err)
		}
		key := ctx.moduleKey(absPath)
		loadModule(imports[imp.Name], jen.Lit(key), pathLoader(key))
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
	return body, nil
}

func (ctx *transpileContext) transpileExport(export *ast.Export, exportsVar string) ([]jen.Code, error) {
	// A type's name is its Go struct; the value to export is the constructor
	// variable, which moduleBinding maps it to (imported modules likewise).
	value := jen.Id(export.Name)
	if mapped, ok := ctx.moduleBinding(export.Name); ok {
		value = jen.Id(mapped)
	}
	return []jen.Code{
		jen.Id(exportsVar).Index(jen.Lit(export.Name)).Op("=").Add(value),
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
func (ctx *transpileContext) transpileModuleStatements(stmts []ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	// A module's top level is a function body too — it becomes Execute() — so
	// its locals are eligible for the same specialisation.
	defer ctx.pushUserScope()()

	savedDirectFns, savedDirectCtors, savedModuleScopeIdx := ctx.directFns, ctx.directCtors, ctx.moduleScopeIdx
	ctx.moduleScopeIdx = len(ctx.userScopes) - 1
	defer func() {
		ctx.directFns, ctx.directCtors, ctx.moduleScopeIdx = savedDirectFns, savedDirectCtors, savedModuleScopeIdx
	}()

	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case *ast.TypeDefine:
			ctx.moduleImports[v.Name] = v.Name + "Constructor"
		case *ast.FunctionDefine, *ast.TraitDefine:
			// Function and trait names are hoisted (mirroring the
			// interpreter), so they shadow built-ins for the whole module body.
			ctx.declareUserName(statementName(v))
		}
	}

	// Direct-call lowering candidates are fixed before any body transpiles, so
	// recursive and forward calls lower too. Hoisted names are declared above
	// for the same reason, and enterScope runs after both so the inference
	// pass sees the final shadowing picture.
	//
	// Whether `range` means the builtin is decided once for the whole module:
	// the signature inference types every body up front, and it has to see
	// exactly the environments the generator will, so both work from this one
	// answer. A module that binds the name `range` anywhere gives up range
	// lowering everywhere.
	savedRange, savedSigs := ctx.rangeNative, ctx.sigs
	defer func() { ctx.rangeNative, ctx.sigs = savedRange, savedSigs }()
	_, rangeImported := ctx.moduleImports["range"]
	ctx.rangeNative = !rangeImported && !ctx.isUserName("range") && !declaresNameAnywhere(stmts, "range")
	ctx.directFns, ctx.directCtors = ctx.collectDirectFns(stmts)
	ctx.sigs = inferSignatures(stmts, ctx.directFns, ctx.rangeNative)
	defer ctx.enterScope(stmts, nil, tyDynamic)()

	var funcAssigns, body []jen.Code
	declare := func(name string) {
		ctx.topDecls = append(ctx.topDecls,
			jen.Var().Id(name).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Nil"))
	}
	savedPos := ctx.errorPos
	defer func() { ctx.errorPos = savedPos }()
	for _, stmt := range stmts {
		ctx.errorPos = stmt.Position()
		switch v := stmt.(type) {
		case *ast.FunctionDefine:
			declare(v.Name)
			if info, ok := ctx.directFns[v.Name]; ok {
				assigns, err := ctx.buildDirectFunction(info, v)
				if err != nil {
					return nil, err
				}
				funcAssigns = append(funcAssigns, assigns...)
				continue
			}
			funcValue, err := ctx.buildFunctionValue(v.Name, v.Position(), v.Parameters, v.Body)
			if err != nil {
				return nil, err
			}
			funcAssigns = append(funcAssigns, jen.Id(v.Name).Op("=").Add(funcValue))
		case *ast.TraitDefine:
			// Traits and types are bound with the functions, in source order,
			// as the interpreter binds them when it loads a module: a type's
			// impls resolve the traits declared above it.
			declare(v.Name)
			codes, err := ctx.transpileTraitDefine(v, onError)
			if err != nil {
				return nil, err
			}
			funcAssigns = append(funcAssigns, codes...)
		case *ast.TypeDefine:
			codes, err := ctx.transpileTypeDefine(v, onError)
			if err != nil {
				return nil, err
			}
			funcAssigns = append(funcAssigns, codes...)
		case *ast.Declare:
			// Split `var x = expr` into a package-level declaration and an
			// in-place assignment so functions may reference the variable
			// regardless of where their definition appears in the source.
			// A specialised variable is declared with its native type; it can
			// never be captured by a closure, since being named inside a nested
			// scope is exactly what disqualifies it from specialisation.
			if t := ctx.typeOf(v.Name); t.native() {
				ctx.topDecls = append(ctx.topDecls, jen.Var().Id(v.Name).Id(goTypeOf(t)))

				pre, value, err := ctx.emitNative(v.Value, onError)
				if err != nil {
					return nil, err
				}
				ctx.declareUserName(v.Name)
				body = append(body, pre...)
				body = append(body, jen.Id(v.Name).Op("=").Add(value))
				continue
			}

			declare(v.Name)
			preStmts, value, err := ctx.transpileExpression(v.Value, onError)
			if err != nil {
				return nil, err
			}
			ctx.declareUserName(v.Name)
			body = append(body, preStmts...)
			body = append(body, jen.Id(v.Name).Op("=").Add(value))
		default:
			codes, err := ctx.transpileStatement(stmt, onError, exportsVar)
			if err != nil {
				return nil, err
			}
			body = append(body, codes...)
		}
	}
	return append(funcAssigns, body...), nil
}
