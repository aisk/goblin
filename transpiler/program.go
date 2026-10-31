package transpiler

import (
	"fmt"
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

// program is one transpilation: an entry module and every .goblin module it
// imports, directly or not, rendered as a Go module directory. The entry
// module becomes outputDir/main.go; each imported module becomes its own
// package under outputDir, at its module key.
type program struct {
	// goModule is the Go module path of the generated program. It gets a
	// prefix so a program named after a standard library package (io.goblin,
	// time.goblin) does not shadow it.
	goModule  string
	outputDir string
	// rootDir is the entry module's directory; module keys are relative to it.
	rootDir string

	loading map[string]bool // modules being transpiled, for cycle detection
	done    map[string]bool // modules already written

	// names numbers generated temporaries. Only uniqueness within one file
	// matters; a program-wide counter is simply the easiest way to get it.
	names int
}

// TranspileToDir transpiles a checked module (see source.LoadFile) read from
// sourceFile into a buildable Go module directory.
func TranspileToDir(mod *ast.Module, sourceFile, outputDir string) error {
	absSource, err := filepath.Abs(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to resolve source file %s: %v", sourceFile, err)
	}
	p := &program{
		goModule:  "goblin.program/" + strings.TrimSuffix(filepath.Base(sourceFile), ".goblin"),
		outputDir: outputDir,
		rootDir:   filepath.Dir(absSource),
		loading:   map[string]bool{},
		done:      map[string]bool{},
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	if err := p.emitModule(mod, absSource, ""); err != nil {
		return err
	}
	return writeGoMod(outputDir, p.goModule)
}

// emitModule transpiles the modules mod imports from disk, then mod itself,
// and writes its Go file. label is the import path mod was loaded through,
// empty for the program's entry module.
func (p *program) emitModule(mod *ast.Module, absPath, label string) error {
	for _, imp := range moduleImports(mod) {
		if imp.IsPathImport() {
			if err := p.emitImport(resolveImport(filepath.Dir(absPath), imp.Path), imp.Path); err != nil {
				return err
			}
		}
	}

	var key string
	if label != "" {
		key = p.moduleKey(absPath)
	}
	file, err := newGenerator(p, mod, absPath, key, label).file()
	if err != nil {
		return err
	}

	path := filepath.Join(p.outputDir, "main.go")
	if key != "" {
		dir := filepath.Join(p.outputDir, filepath.FromSlash(key))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create package directory %s: %v", dir, err)
		}
		path = filepath.Join(dir, packageName(key)+".go")
	}
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", path, err)
	}
	defer out.Close()
	if err := file.Render(out); err != nil {
		return fmt.Errorf("failed to render file %s: %v", path, err)
	}
	return nil
}

// emitImport loads and emits the module at absPath once, however many
// modules import it, and rejects import cycles.
func (p *program) emitImport(absPath, importPath string) error {
	if p.done[absPath] {
		return nil
	}
	if p.loading[absPath] {
		return fmt.Errorf("circular import detected: %s", importPath)
	}
	p.loading[absPath] = true
	defer delete(p.loading, absPath)

	mod, err := source.LoadFile(absPath)
	if err != nil {
		return err
	}
	if err := p.emitModule(mod, absPath, importPath); err != nil {
		return err
	}
	p.done[absPath] = true
	return nil
}

// moduleImports lists a module's import statements in source order.
func moduleImports(mod *ast.Module) []*ast.Import {
	var imports []*ast.Import
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			imports = append(imports, imp)
		}
	}
	return imports
}

// resolveImport resolves a relative import path against the importing
// module's directory, as the interpreter does.
func resolveImport(baseDir, importPath string) string {
	return filepath.Join(baseDir, importPath+".goblin")
}

// moduleKey returns a stable slash-separated identifier for a resolved module
// path, relative to the entry point's directory. Path components above the
// root are folded into "__up" so distinct modules never collide.
func (p *program) moduleKey(absPath string) string {
	path := strings.TrimSuffix(absPath, ".goblin")
	if rel, err := filepath.Rel(p.rootDir, path); err == nil {
		path = rel
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == ".." {
			parts[i] = "__up"
		}
	}
	return strings.Join(parts, "/")
}

// importPath is the Go import path of an imported module's package.
func (p *program) importPath(key string) string {
	return p.goModule + "/" + key
}

// keyToIdent turns a module key into a Go identifier fragment.
func keyToIdent(key string) string {
	return strings.NewReplacer("/", "_", ".", "_", "-", "_").Replace(key)
}

// packageName is the Go package name of an imported module: its last path
// segment.
func packageName(key string) string {
	return filepath.Base(key)
}

// moduleExecutorPath returns the Go import path of a stdlib module's package.
func moduleExecutorPath(name string) string {
	return pathExtension + "/" + name
}

// moduleVarName returns the generated-code variable holding a loaded module,
// derived from the last path segment ("x/compress/gzip" -> "gzip_module").
func moduleVarName(name string) string {
	return name[strings.LastIndex(name, "/")+1:] + "_module"
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

// --- go.mod

// writeGoMod writes the go.mod of the generated program.
func writeGoMod(outputDir, moduleName string) error {
	content := goModContent(moduleName, goblinRuntimeVersion(), detectGoblinRoot())
	return os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(content), 0644)
}

func goModContent(moduleName, runtimeVersion, goblinRoot string) string {
	content := fmt.Sprintf("module %s\n\ngo 1.19\n\nrequire github.com/aisk/goblin %s\n", moduleName, runtimeVersion)
	if goblinRoot != "" {
		content += fmt.Sprintf("\nreplace github.com/aisk/goblin => %s\n", goblinRoot)
	}
	return content
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
	if cwd, err := os.Getwd(); err == nil {
		if root := findGoblinRoot(cwd, 10); root != "" {
			return root
		}
	}
	if exe, err := os.Executable(); err == nil {
		return findGoblinRoot(filepath.Dir(exe), 5)
	}
	return ""
}

// findGoblinRoot walks up at most depth directories from dir looking for the
// goblin module's go.mod.
func findGoblinRoot(dir string, depth int) string {
	for i := 0; i < depth; i++ {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(data), "module "+pathBase) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// goblinRuntimeVersion returns the version of the Goblin module that built the
// running CLI. Binaries installed with `go install module@version` carry this
// information, so generated programs use the matching runtime instead of a
// potentially stale hard-coded version.
func goblinRuntimeVersion() string {
	info, _ := debug.ReadBuildInfo()
	return goblinRuntimeVersionFromBuildInfo(info)
}

func goblinRuntimeVersionFromBuildInfo(info *debug.BuildInfo) string {
	if info != nil && info.Main.Path == pathBase && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return defaultGoblinRuntimeVersion
}
