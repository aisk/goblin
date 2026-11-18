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
	cliExt "github.com/aisk/goblin/extension/cli"
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
	tarExt "github.com/aisk/goblin/extension/tar"
	timeExt "github.com/aisk/goblin/extension/time"
	urlExt "github.com/aisk/goblin/extension/url"
	zipExt "github.com/aisk/goblin/extension/zip"
	"github.com/aisk/goblin/object"
)

var modules = map[string]object.ModuleExecutor{
	"base64": base64Ext.Execute,
	"cli":    cliExt.Execute,
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
	"tar":    tarExt.Execute,
	"time":   timeExt.Execute,
	"url":    urlExt.Execute,
	"zip":    zipExt.Execute,
}

// withArgs lists the modules that read the program's command line (the
// program name and its arguments). Their Execute reads the process's own,
// which is right for a compiled program; the interpreter builds them with
// the script's command line instead, through LookupWithArgs.
var withArgs = map[string]func(argv []string) (object.Object, error){
	"cli": cliExt.ExecuteWithFrozenArgs,
	"os":  osExt.ExecuteWithFrozenArgs,
}

// LookupWithArgs returns the constructor of a stdlib module that reads the
// command line, building it from a given argv (the script path followed by
// its arguments).
func LookupWithArgs(name string) (func(argv []string) (object.Object, error), bool) {
	fn, ok := withArgs[name]
	return fn, ok
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
