// Package stdlib is the registry of importable standard library modules.
//
// Every module lives in the Go package extension/<module name> and exposes an
// Execute constructor. The interpreter loads modules from this table; the
// transpiler only consults its names and emits an import of the module's own
// package, so a compiled program links only the modules it imports.
package stdlib

import (
	"sort"

	base64Ext "github.com/aisk/goblin/extension/base64"
	csvExt "github.com/aisk/goblin/extension/csv"
	execExt "github.com/aisk/goblin/extension/exec"
	"github.com/aisk/goblin/extension/fs"
	gzipExt "github.com/aisk/goblin/extension/gzip"
	hashExt "github.com/aisk/goblin/extension/hash"
	httpExt "github.com/aisk/goblin/extension/http"
	jsonExt "github.com/aisk/goblin/extension/json"
	mathExt "github.com/aisk/goblin/extension/math"
	osExt "github.com/aisk/goblin/extension/os"
	randExt "github.com/aisk/goblin/extension/rand"
	regexExt "github.com/aisk/goblin/extension/regex"
	timeExt "github.com/aisk/goblin/extension/time"
	urlExt "github.com/aisk/goblin/extension/url"
	uuidExt "github.com/aisk/goblin/extension/uuid"
	"github.com/aisk/goblin/object"
)

var modules = map[string]object.ModuleExecutor{
	"base64": base64Ext.Execute,
	"csv":    csvExt.Execute,
	"exec":   execExt.Execute,
	"fs":     fs.Execute,
	"gzip":   gzipExt.Execute,
	"hash":   hashExt.Execute,
	"http":   httpExt.Execute,
	"json":   jsonExt.Execute,
	"math":   mathExt.Execute,
	"os":     osExt.Execute,
	"rand":   randExt.Execute,
	"regex":  regexExt.Execute,
	"time":   timeExt.Execute,
	"url":    urlExt.Execute,
	"uuid":   uuidExt.Execute,
}

// Lookup returns the executor of a stdlib module.
func Lookup(name string) (object.ModuleExecutor, bool) {
	exec, ok := modules[name]
	return exec, ok
}

// Has reports whether name is a stdlib module.
func Has(name string) bool {
	_, ok := modules[name]
	return ok
}

// Names lists every stdlib module, sorted.
func Names() []string {
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
