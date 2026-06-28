package interpreter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/extension/fs"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

// builtinModules maps a built-in module name to its executor, mirroring the
// transpiler's knownModules table.
var builtinModules = map[string]object.ModuleExecutor{
	"os":     extension.ExecuteOs,
	"random": extension.ExecuteRandom,
	"math":   extension.ExecuteMath,
	"http":   extension.ExecuteHttp,
	"fs":     fs.Execute,
	"mime":   extension.ExecuteMime,
	"json":   extension.ExecuteJson,
}

func isPathImport(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.Contains(path, "/")
}

// loadInto resolves imports and hoists function/type definitions for a module
// body into env, so references resolve regardless of source order.
func loadInto(mod *ast.Module, env *Environment, baseDir string, reg *object.Registry) error {
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			m, err := resolveImport(imp, baseDir, reg)
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

func resolveImport(imp *ast.Import, baseDir string, reg *object.Registry) (object.Object, error) {
	if isPathImport(imp.Path) {
		full := filepath.Join(baseDir, imp.Path) + ".goblin"
		return reg.Load(full, func() (object.Object, error) {
			return loadModuleFile(full, reg)
		})
	}
	exec, ok := builtinModules[imp.Path]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", imp.Path)
	}
	return reg.Load(imp.Path, exec)
}

// loadModuleFile interprets a Goblin source file as a module and returns its
// exported members.
func loadModuleFile(path string, reg *object.Registry) (object.Object, error) {
	l, err := lexer.NewLexerFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read module %s: %w", path, err)
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
	if err := loadInto(mod, env, filepath.Dir(path), reg); err != nil {
		return nil, err
	}
	if err := evalStatements(mod.Body, env); err != nil {
		return nil, err
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
	return &object.Module{Members: members}, nil
}
