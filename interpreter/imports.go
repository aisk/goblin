package interpreter

import (
	"fmt"
	"path/filepath"

	"github.com/aisk/goblin/ast"
	csvExt "github.com/aisk/goblin/extension/csv"
	execExt "github.com/aisk/goblin/extension/exec"
	"github.com/aisk/goblin/extension/fs"
	httpExt "github.com/aisk/goblin/extension/http"
	ioExt "github.com/aisk/goblin/extension/io"
	jsonExt "github.com/aisk/goblin/extension/json"
	mathExt "github.com/aisk/goblin/extension/math"
	osExt "github.com/aisk/goblin/extension/os"
	pathExt "github.com/aisk/goblin/extension/path"
	randExt "github.com/aisk/goblin/extension/rand"
	regexpExt "github.com/aisk/goblin/extension/regexp"
	timeExt "github.com/aisk/goblin/extension/time"
	urlExt "github.com/aisk/goblin/extension/url"
	uuidExt "github.com/aisk/goblin/extension/uuid"
	tarExt "github.com/aisk/goblin/extension/x/archive/tar"
	zipExt "github.com/aisk/goblin/extension/x/archive/zip"
	bzip2Ext "github.com/aisk/goblin/extension/x/compress/bzip2"
	flateExt "github.com/aisk/goblin/extension/x/compress/flate"
	gzipExt "github.com/aisk/goblin/extension/x/compress/gzip"
	lzwExt "github.com/aisk/goblin/extension/x/compress/lzw"
	zlibExt "github.com/aisk/goblin/extension/x/compress/zlib"
	hmacExt "github.com/aisk/goblin/extension/x/crypto/hmac"
	md5Ext "github.com/aisk/goblin/extension/x/crypto/md5"
	sha1Ext "github.com/aisk/goblin/extension/x/crypto/sha1"
	sha256Ext "github.com/aisk/goblin/extension/x/crypto/sha256"
	sha512Ext "github.com/aisk/goblin/extension/x/crypto/sha512"
	ascii85Ext "github.com/aisk/goblin/extension/x/encoding/ascii85"
	base32Ext "github.com/aisk/goblin/extension/x/encoding/base32"
	base64Ext "github.com/aisk/goblin/extension/x/encoding/base64"
	hexExt "github.com/aisk/goblin/extension/x/encoding/hex"
	pemExt "github.com/aisk/goblin/extension/x/encoding/pem"
	adler32Ext "github.com/aisk/goblin/extension/x/hash/adler32"
	crc32Ext "github.com/aisk/goblin/extension/x/hash/crc32"
	crc64Ext "github.com/aisk/goblin/extension/x/hash/crc64"
	fnvExt "github.com/aisk/goblin/extension/x/hash/fnv"
	htmlExt "github.com/aisk/goblin/extension/x/html"
	mimeExt "github.com/aisk/goblin/extension/x/mime"
	qpExt "github.com/aisk/goblin/extension/x/mime/quotedprintable"
	mailExt "github.com/aisk/goblin/extension/x/net/mail"
	netipExt "github.com/aisk/goblin/extension/x/net/netip"
	unicodeExt "github.com/aisk/goblin/extension/x/unicode"
	utf8Ext "github.com/aisk/goblin/extension/x/unicode/utf8"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/token"
)

// builtinModules maps a built-in module name to its executor, mirroring the
// transpiler's knownModules table. "os" is intentionally absent: the
// interpreter binds it per run via os.ExecuteWithFrozenArgs so argv is scoped
// to the script (or REPL) without process-global state.
var builtinModules = map[string]object.ModuleExecutor{
	"csv":                    csvExt.Execute,
	"exec":                   execExt.Execute,
	"fs":                     fs.Execute,
	"http":                   httpExt.Execute,
	"io":                     ioExt.Execute,
	"json":                   jsonExt.Execute,
	"math":                   mathExt.Execute,
	"path":                   pathExt.Execute,
	"rand":                   randExt.Execute,
	"regexp":                 regexpExt.Execute,
	"time":                   timeExt.Execute,
	"url":                    urlExt.Execute,
	"uuid":                   uuidExt.Execute,
	"x/archive/tar":          tarExt.Execute,
	"x/archive/zip":          zipExt.Execute,
	"x/compress/bzip2":       bzip2Ext.Execute,
	"x/compress/flate":       flateExt.Execute,
	"x/compress/gzip":        gzipExt.Execute,
	"x/compress/lzw":         lzwExt.Execute,
	"x/compress/zlib":        zlibExt.Execute,
	"x/crypto/hmac":          hmacExt.Execute,
	"x/crypto/md5":           md5Ext.Execute,
	"x/crypto/sha1":          sha1Ext.Execute,
	"x/crypto/sha256":        sha256Ext.Execute,
	"x/crypto/sha512":        sha512Ext.Execute,
	"x/encoding/ascii85":     ascii85Ext.Execute,
	"x/encoding/base32":      base32Ext.Execute,
	"x/encoding/base64":      base64Ext.Execute,
	"x/encoding/hex":         hexExt.Execute,
	"x/encoding/pem":         pemExt.Execute,
	"x/hash/adler32":         adler32Ext.Execute,
	"x/hash/crc32":           crc32Ext.Execute,
	"x/hash/crc64":           crc64Ext.Execute,
	"x/hash/fnv":             fnvExt.Execute,
	"x/html":                 htmlExt.Execute,
	"x/mime":                 mimeExt.Execute,
	"x/mime/quotedprintable": qpExt.Execute,
	"x/net/mail":             mailExt.Execute,
	"x/net/netip":            netipExt.Execute,
	"x/unicode":              unicodeExt.Execute,
	"x/unicode/utf8":         utf8Ext.Execute,
}

func isPathImport(path string) bool {
	return source.IsPathImport(path)
}

// loadInto resolves imports and hoists function, type and trait definitions
// for a module body into env, so references resolve regardless of source
// order. Types and traits are bound here once, in source order, so a type's
// impls resolve the traits declared above it. argv is the
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
			if err := defineType(s, env); err != nil {
				return positionError(err, s.Position())
			}
		case *ast.TraitDefine:
			if err := defineTrait(s, env); err != nil {
				return positionError(err, s.Position())
			}
		}
	}
	return nil
}

// loadError attaches the module frame to an error loadInto reported for a
// statement.
func loadError(err error, module string) error {
	if p, ok := err.(*positionedError); ok {
		return object.WithFrame(p.err, stackFrame(module, "<module>", p.pos))
	}
	return err
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
			return osExt.ExecuteWithFrozenArgs(argv)
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
	l, err := source.NewLexerFile(path)
	if err != nil {
		return nil, object.NewImportError("failed to read module %s: %v", path, err)
	}
	mod, err := source.Parse(l)
	if err != nil {
		return nil, err
	}
	if err := semantic.CheckModule(mod); err != nil {
		return nil, err
	}

	env := NewEnvironment(nil)
	if err := loadInto(mod, env, filepath.Dir(path), reg, argv); err != nil {
		return nil, loadError(err, moduleName(path))
	}
	if err := evalStatements(mod.Body, env); err != nil {
		var pos token.Pos
		if len(mod.Body) > 0 {
			pos = mod.Body[0].Position()
		}
		err, pos = takePosition(err, pos)
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
	return &object.Module{Name: source.ModuleName(path), Members: members}, nil
}
