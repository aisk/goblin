package interpreter

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aisk/goblin/ast"
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
	// Modules that read the command line (os, cli) bind argv per run, so
	// the script (or REPL session) sees its own arguments without
	// process-global state.
	if withArgs, ok := stdlib.LookupWithArgs(imp.Path); ok {
		return reg.Load(imp.Path, func() (object.Object, error) {
			return withArgs(argv)
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

	return exportModule(mod, g, source.ModuleName(path), path)
}

// exportModule builds the module value of a loaded module. An exported name
// the module assigns somewhere is a live binding that reads the module's
// cell; any other exported name holds the value it was defined with, which
// cannot change, so it is stored directly and read without the indirection.
func exportModule(mod *ast.Module, g *globals, name, path string) (*object.Module, error) {
	m := &object.Module{Name: name, Members: map[string]object.Object{}}
	var assigned map[string]bool
	for _, stmt := range mod.Body {
		exp, ok := stmt.(*ast.Export)
		if !ok {
			continue
		}
		v, ok := g.Get(exp.Name)
		if !ok {
			return nil, fmt.Errorf("module %s exports undefined name '%s'", path, exp.Name)
		}
		if assigned == nil {
			assigned = ast.AssignedNames(mod.Body)
		}
		if !assigned[exp.Name] {
			m.Members[exp.Name] = v
			continue
		}
		c := g.cell(exp.Name)
		m.Bind(exp.Name, func() object.Object { return c.value })
	}
	return m, nil
}
