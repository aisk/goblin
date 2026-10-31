package transpiler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/source"
	"github.com/dave/jennifer/jen"
)

const (
	pathBase                    = "github.com/aisk/goblin"
	pathObject                  = pathBase + "/object"
	pathExtension               = pathBase + "/extension"
	defaultGoblinRuntimeVersion = "v0.0.0-20260731160124-eddbfc600c08"
)

// moduleExecutorPath returns the Go import path of a stdlib module's package.
func moduleExecutorPath(name string) string {
	return pathExtension + "/" + name
}

// moduleVarName returns the generated-code variable holding a loaded module,
// derived from the last path segment ("x/compress/gzip" -> "gzip_module").
func moduleVarName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name + "_module"
}

// resolveImportPath resolves a relative import path against the directory of
// the module currently being transpiled, mirroring how the interpreter
// resolves imports against the importing file's directory.
func (ctx *transpileContext) resolveImportPath(importPath string) (string, error) {
	p := importPath + ".goblin"
	if !filepath.IsAbs(p) && ctx.baseDir != "" {
		p = filepath.Join(ctx.baseDir, p)
	}
	return filepath.Abs(p)
}

// moduleKey returns a stable slash-separated identifier for a resolved module
// path, relative to the entry point's directory. Path components above the
// root are folded into "__up" so distinct modules never collide.
func (ctx *transpileContext) moduleKey(absPath string) string {
	p := strings.TrimSuffix(absPath, ".goblin")
	if ctx.rootDir != "" {
		if rel, err := filepath.Rel(ctx.rootDir, p); err == nil {
			p = rel
		}
	}
	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if part == ".." {
			parts[i] = "__up"
		}
	}
	return strings.Join(parts, "/")
}

// keyToIdent turns a module key into a Go identifier fragment.
func keyToIdent(key string) string {
	s := strings.ReplaceAll(key, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func keyToFuncName(key string) string {
	return "_execute_" + keyToIdent(key)
}

// collectModuleImports walks a module's imports and builds the module-name to
// Go-variable-name map both backends of the transpiler use. Modules load into
// package-level variables (so package-level functions can reference them)
// named after the module itself, which also makes the variable shared when
// several modules in one generated file import the same module. Path imports
// are handed to recurse (when non-nil) so dependent modules are transpiled
// first; stdlib imports are validated against the stdlib registry. importPath is the
// import path of the module being collected, empty for the entry module (it
// only affects error wording).
func (ctx *transpileContext) collectModuleImports(mod *ast.Module, importPath string, recurse func(string) error) (map[string]string, error) {
	imports := make(map[string]string)
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok {
			continue
		}
		if imp.IsPathImport() {
			absPath, err := ctx.resolveImportPath(imp.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve path %s: %v", imp.Path, err)
			}
			imports[imp.Name] = "_mod_" + keyToIdent(ctx.moduleKey(absPath))
			if recurse != nil {
				if err := recurse(imp.Path); err != nil {
					return nil, err
				}
			}
		} else {
			// The checker has validated the name against the stdlib registry.
			imports[imp.Name] = "_" + moduleVarName(imp.Path)
		}
	}
	return imports, nil
}

// loadPathModule resolves, parses, and semantically checks the .goblin module
// at importPath, with dedup and circular-import detection, then calls emit to
// generate code for it. Imports inside the module resolve against its own
// directory for the duration of emit.
func (ctx *transpileContext) loadPathModule(importPath string, emit func(mod *ast.Module, absPath string) error) error {
	absPath, err := ctx.resolveImportPath(importPath)
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

	// Imports inside this module resolve against its own directory.
	savedBaseDir := ctx.baseDir
	ctx.baseDir = filepath.Dir(absPath)
	defer func() { ctx.baseDir = savedBaseDir }()

	mod, err := source.LoadFile(absPath)
	if err != nil {
		return err
	}

	if err := emit(mod, absPath); err != nil {
		return err
	}

	ctx.imported[absPath] = struct{}{}
	return nil
}

// singleFilePathLoader emits the loader argument for a path-import Load call
// in single-file mode: a reference to the module's generated executor func.
func singleFilePathLoader(key string) jen.Code {
	return jen.Id(keyToFuncName(key))
}

// registryPathLoader emits the loader argument for a path-import Load call in
// directory mode: a closure calling the generated package's Execute with the
// shared registry.
func (ctx *transpileContext) registryPathLoader(key string) jen.Code {
	importPath := ctx.goModuleName + "/" + key
	return jen.Func().Params().Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(
		jen.Return(jen.Qual(importPath, "Execute").Call(jen.Id("_registry"))),
	)
}

// Transpile renders a checked module (see source.LoadFile) as a single Go
// file. Relative imports resolve against the working directory.
func Transpile(mod *ast.Module, output io.Writer) error {
	ctx := newTranspileContext()
	if cwd, err := os.Getwd(); err == nil {
		ctx.baseDir = cwd
		ctx.rootDir = cwd
	}

	// Collect imports and transpile each path-imported .goblin module
	imports, err := ctx.collectModuleImports(mod, "", ctx.transpilePathModule)
	if err != nil {
		return err
	}
	ctx.moduleImports = imports

	f := jen.NewFile(mod.Name)
	f.Var().Id("builtin").Op("=").Qual(pathExtension, "BuiltinsModule")

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

	body, err := ctx.emitExecuteBody(mod, imports, "", singleFilePathLoader)
	if err != nil {
		return err
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
	}

	for _, fn := range ctx.moduleFuncs {
		f.Add(fn)
	}

	f.Func().Id("Execute").Params().Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(body...)
	f.Func().Id("main").Params().Block(mainBody()...)
	return f.Render(output)
}

// mainBody emits the generated main(): run Execute, print failures, and turn a
// Go-side runtime panic into a clean internal error instead of a Go stack dump.
func mainBody() []jen.Code {
	return []jen.Code{
		jen.Defer().Func().Params().Block(
			jen.If(jen.Id("r").Op(":=").Recover(), jen.Id("r").Op("!=").Nil()).Block(
				jen.Qual("fmt", "Fprintf").Call(jen.Qual("os", "Stderr"), jen.Lit("internal error: %v\n"), jen.Id("r")),
				jen.Qual("os", "Exit").Call(jen.Lit(1)),
			),
		).Call(),
		jen.List(jen.Id("_"), jen.Id("err")).Op(":=").Id("Execute").Call(),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Qual("fmt", "Fprintf").Call(jen.Qual("os", "Stderr"), jen.Lit("%+v\n"), jen.Id("err")),
			jen.Qual("os", "Exit").Call(jen.Lit(1)),
		),
	}
}

// transpilePathModule parses and transpiles a .goblin file at the given path,
// generating a top-level executor function.
func (ctx *transpileContext) transpilePathModule(importPath string) error {
	return ctx.loadPathModule(importPath, func(mod *ast.Module, absPath string) error {
		// Collect sub-module imports and transpile path imports first
		imports, err := ctx.collectModuleImports(mod, importPath, ctx.transpilePathModule)
		if err != nil {
			return err
		}

		// Save and restore module imports for this scope
		savedImports := ctx.moduleImports
		ctx.moduleImports = imports
		defer func() { ctx.moduleImports = savedImports }()

		funcBody, err := ctx.emitExecuteBody(mod, imports, importPath, singleFilePathLoader)
		if err != nil {
			return err
		}

		funcName := keyToFuncName(ctx.moduleKey(absPath))
		fn := jen.Func().Id(funcName).Params().Parens(jen.List(
			jen.Qual(pathObject, "Object"), jen.Error(),
		)).Block(funcBody...)

		ctx.moduleFuncs = append(ctx.moduleFuncs, fn)
		return nil
	})
}

// pathToPackageName returns the last path segment as the package name.
func pathToPackageName(importPath string) string {
	return filepath.Base(importPath)
}

// detectGoblinRoot locates a local goblin source checkout so generated code
// builds against it (via a replace directive) instead of the published module.
// GOBLIN_ROOT takes precedence; otherwise it walks up from the current working
// directory, then from the executable path, looking for a go.mod that declares
// github.com/aisk/goblin. Without a match the published runtime version is
// used, which is correct for released binaries but means a development binary
// run outside the repository silently builds against the released runtime.
func detectGoblinRoot() string {
	if root := os.Getenv("GOBLIN_ROOT"); root != "" {
		return root
	}

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
	content := generateGoModContent(moduleName, goblinRuntimeVersion(), goblinRoot)
	return os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(content), 0644)
}

// goblinRuntimeVersion returns the version of the Goblin module that built the
// running CLI. Binaries installed with `go install module@version` carry this
// information, so generated programs use the matching runtime instead of a
// potentially stale hard-coded version.
func goblinRuntimeVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return defaultGoblinRuntimeVersion
	}
	return goblinRuntimeVersionFromBuildInfo(info)
}

func goblinRuntimeVersionFromBuildInfo(info *debug.BuildInfo) string {
	if info != nil && info.Main.Path == pathBase && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return defaultGoblinRuntimeVersion
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
// mod must have been checked (see source.LoadFile).
// The entry-point module becomes output/main.go; each imported path module becomes
// its own package under outputDir.
func TranspileToDir(mod *ast.Module, sourceFile, outputDir string) error {
	// The Go module path gets a prefix so a program named after a standard
	// library package (io.goblin, time.goblin) does not shadow it.
	base := filepath.Base(sourceFile)
	moduleName := "goblin.program/" + strings.TrimSuffix(base, ".goblin")

	ctx := newTranspileContext()
	ctx.goModuleName = moduleName
	ctx.outputDir = outputDir

	absSource, err := filepath.Abs(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to resolve source file %s: %v", sourceFile, err)
	}
	ctx.baseDir = filepath.Dir(absSource)
	ctx.rootDir = ctx.baseDir

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Recursively transpile each path import into its own package file.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !imp.IsPathImport() {
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
	return ctx.loadPathModule(importPath, func(mod *ast.Module, absPath string) error {
		// Process sub-imports first (depth-first).
		imports, err := ctx.collectModuleImports(mod, importPath, ctx.transpilePathModuleToFile)
		if err != nil {
			return err
		}

		key := ctx.moduleKey(absPath)
		pkgName := pathToPackageName(key)
		pkgDir := filepath.Join(ctx.outputDir, filepath.FromSlash(key))
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return fmt.Errorf("failed to create package directory %s: %v", pkgDir, err)
		}

		f := jen.NewFile(pkgName)
		f.Var().Id("builtin").Op("=").Qual(pathExtension, "BuiltinsModule")

		// Register import aliases so jennifer uses _pkg_X for sub-path-imports.
		for _, stmt := range mod.Body {
			imp, ok := stmt.(*ast.Import)
			if !ok || !imp.IsPathImport() {
				continue
			}
			subAbsPath, err := ctx.resolveImportPath(imp.Path)
			if err != nil {
				return fmt.Errorf("failed to resolve path %s: %v", imp.Path, err)
			}
			subKey := ctx.moduleKey(subAbsPath)
			subImportPath := ctx.goModuleName + "/" + subKey
			f.ImportAlias(subImportPath, "_pkg_"+keyToIdent(subKey))
		}

		savedImports := ctx.moduleImports
		ctx.moduleImports = imports
		defer func() { ctx.moduleImports = savedImports }()

		savedTopDecls, savedLoaded := ctx.topDecls, ctx.loadedModuleVars
		ctx.topDecls, ctx.loadedModuleVars = nil, make(map[string]struct{})
		defer func() { ctx.topDecls, ctx.loadedModuleVars = savedTopDecls, savedLoaded }()

		funcBody, err := ctx.emitExecuteBody(mod, imports, importPath, ctx.registryPathLoader)
		if err != nil {
			return err
		}

		for _, decl := range ctx.topDecls {
			f.Add(decl)
		}

		f.Func().Id("Execute").Params(
			jen.Id("_registry").Op("*").Qual(pathObject, "Registry"),
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
		return nil
	})
}

// generateMainFile generates output/main.go for the top-level module.
func (ctx *transpileContext) generateMainFile(mod *ast.Module) error {
	f := jen.NewFile("main")
	f.Var().Id("builtin").Op("=").Qual(pathExtension, "BuiltinsModule")

	// Register import aliases for path imports.
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !imp.IsPathImport() {
			continue
		}
		absPath, err := ctx.resolveImportPath(imp.Path)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %v", imp.Path, err)
		}
		key := ctx.moduleKey(absPath)
		importPath := ctx.goModuleName + "/" + key
		f.ImportAlias(importPath, "_pkg_"+keyToIdent(key))
	}

	// Emit _registry global if there are any imports.
	for _, stmt := range mod.Body {
		if _, ok := stmt.(*ast.Import); ok {
			f.Var().Id("_registry").Op("=").Qual(pathObject, "NewRegistry").Call()
			break
		}
	}

	// Build module imports map for this scope.
	imports, err := ctx.collectModuleImports(mod, "", nil)
	if err != nil {
		return err
	}

	savedImports := ctx.moduleImports
	ctx.moduleImports = imports
	defer func() { ctx.moduleImports = savedImports }()

	savedTopDecls, savedLoaded := ctx.topDecls, ctx.loadedModuleVars
	ctx.topDecls, ctx.loadedModuleVars = nil, make(map[string]struct{})
	defer func() { ctx.topDecls, ctx.loadedModuleVars = savedTopDecls, savedLoaded }()

	body, err := ctx.emitExecuteBody(mod, imports, "", ctx.registryPathLoader)
	if err != nil {
		return err
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
	}

	f.Func().Id("Execute").Params().Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(body...)
	f.Func().Id("main").Params().Block(mainBody()...)

	outFile := filepath.Join(ctx.outputDir, "main.go")
	fh, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create main.go: %v", err)
	}
	defer fh.Close()

	return f.Render(fh)
}
