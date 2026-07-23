package interpreter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	execExt "github.com/aisk/goblin/extension/exec"
	"github.com/aisk/goblin/extension/fs"
	httpExt "github.com/aisk/goblin/extension/http"
	pathExt "github.com/aisk/goblin/extension/path"
	regexpExt "github.com/aisk/goblin/extension/regexp"
	timeExt "github.com/aisk/goblin/extension/time"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/token"
)

// builtinModules maps a built-in module name to its executor, mirroring the
// transpiler's knownModules table. "os" is intentionally absent: the
// interpreter binds it per run via ExecuteOsWithFrozenArgs so argv is scoped to the
// script (or REPL) without process-global state.
var builtinModules = map[string]object.ModuleExecutor{
	"random": extension.ExecuteRandom,
	"math":   extension.ExecuteMath,
	"http":   httpExt.Execute,
	"fs":     fs.Execute,
	"mime":   extension.ExecuteMime,
	"json":   extension.ExecuteJson,
	"uuid":   extension.ExecuteUUID,
	"path":   pathExt.Execute,
	"time":   timeExt.Execute,
	"exec":   execExt.Execute,
	"regexp": regexpExt.Execute,
	"csv":    extension.ExecuteCSV,
}

func isPathImport(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.Contains(path, "/")
}

// loadInto resolves imports and hoists function/type definitions for a module
// body into env, so references resolve regardless of source order. argv is the
// script command line closed over by import "os".
func loadInto(mod *ast.Module, env *Environment, baseDir string, reg *object.Registry, argv []string) error {
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			m, err := resolveImport(imp, baseDir, reg, argv)
			if err != nil {
				return err
			}
			env.Define(imp.Name, m)
		}
	}
	for _, stmt := range mod.Body {
		switch s := stmt.(type) {
		case *ast.FunctionDefine:
			env.Define(s.Name, makeFunction(s, env))
		case *ast.TypeDefine:
			defineType(s, env)
		}
	}
	return nil
}

func resolveImport(imp *ast.Import, baseDir string, reg *object.Registry, argv []string) (object.Object, error) {
	if isPathImport(imp.Path) {
		full := filepath.Join(baseDir, imp.Path) + ".goblin"
		return reg.Load(full, func() (object.Object, error) {
			return loadModuleFile(full, reg, argv)
		})
	}
	if imp.Path == "os" {
		return reg.Load(imp.Path, func() (object.Object, error) {
			return extension.ExecuteOsWithFrozenArgs(argv)
		})
	}
	exec, ok := builtinModules[imp.Path]
	if !ok {
		return nil, object.NewImportError("unknown module: %s", imp.Path)
	}
	return reg.Load(imp.Path, exec)
}

// loadModuleFile interprets a Goblin source file as a module and returns its
// exported members.
func loadModuleFile(path string, reg *object.Registry, argv []string) (object.Object, error) {
	l, err := lexer.NewLexerFile(path)
	if err != nil {
		return nil, object.NewImportError("failed to read module %s: %v", path, err)
	}
	st, err := parser.NewParser().Parse(l)
	if err != nil {
		return nil, err
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		return nil, fmt.Errorf("internal error: unexpected AST type in module %s", path)
	}
	if err := semantic.CheckModule(mod); err != nil {
		return nil, err
	}

	env := NewEnvironment(nil)
	if err := loadInto(mod, env, filepath.Dir(path), reg, argv); err != nil {
		return nil, err
	}
	if err := evalStatements(mod.Body, env); err != nil {
		var pos token.Pos
		if len(mod.Body) > 0 {
			pos = mod.Body[0].Position()
		}
		return nil, object.WithFrame(err, stackFrame(moduleName(path), "<module>", pos))
	}

	members := make(map[string]object.Object)
	for _, stmt := range mod.Body {
		if exp, ok := stmt.(*ast.Export); ok {
			v, ok := env.Get(exp.Name)
			if !ok {
				return nil, fmt.Errorf("module %s exports undefined name '%s'", path, exp.Name)
			}
			members[exp.Name] = v
		}
	}
	return &object.Module{Name: moduleNameFromFile(path), Members: members}, nil
}

// moduleNameFromFile extracts a clean module name from a file path,
// e.g. "/path/to/mymod.goblin" → "mymod".
func moduleNameFromFile(path string) string {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".goblin")
}
