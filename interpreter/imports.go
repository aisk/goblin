package interpreter

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aisk/goblin/ast"
	osExt "github.com/aisk/goblin/extension/os"
	"github.com/aisk/goblin/extension/stdlib"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/source"
)

// loadError attaches the module frame to an error loadInto reported for a
// statement.
func loadError(err error, module string) error {
	if p, ok := err.(*positionedError); ok {
		return object.WithFrame(p.err, stackFrame(module, "<module>", p.pos))
	}
	return err
}

func resolveImport(imp *ast.Import, baseDir string, reg *object.Registry, argv []string) (object.Object, error) {
	if imp.IsPathImport() {
		full := filepath.Join(baseDir, imp.Path) + ".goblin"
		return reg.Load(full, func() (object.Object, error) {
			return loadModuleFile(full, reg, argv)
		})
	}
	// os binds argv per run, so the script (or REPL session) sees its own
	// arguments without process-global state.
	if imp.Path == "os" {
		return reg.Load(imp.Path, func() (object.Object, error) {
			return osExt.ExecuteWithFrozenArgs(argv)
		})
	}
	exec, ok := stdlib.Lookup(imp.Path)
	if !ok {
		return nil, object.NewImportError("unknown module: %s", imp.Path)
	}
	return reg.Load(imp.Path, exec)
}

// loadModuleFile interprets a Goblin source file as a module and returns its
// exported members.
func loadModuleFile(path string, reg *object.Registry, argv []string) (object.Object, error) {
	mod, err := source.LoadFile(path)
	if err != nil {
		var readErr *source.ReadError
		if errors.As(err, &readErr) {
			return nil, object.NewImportError("%v", readErr)
		}
		return nil, err
	}

	g, err := runModule(mod, filepath.Dir(path), source.ModuleName(path), reg, argv)
	if err != nil {
		return nil, err
	}

	members := make(map[string]object.Object)
	for _, stmt := range mod.Body {
		if exp, ok := stmt.(*ast.Export); ok {
			v, ok := g.Get(exp.Name)
			if !ok {
				return nil, fmt.Errorf("module %s exports undefined name '%s'", path, exp.Name)
			}
			members[exp.Name] = v
		}
	}
	return &object.Module{Name: source.ModuleName(path), Members: members}, nil
}
