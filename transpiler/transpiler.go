package transpiler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

const (
	pathBase                    = "github.com/aisk/goblin"
	pathObject                  = pathBase + "/object"
	pathExtension               = pathBase + "/extension"
	defaultGoblinRuntimeVersion = "v0.0.0-20260731160124-eddbfc600c08"
)

// knownModules lists the stdlib modules the transpiler can import. Each
// module lives in the Go package extension/<module name> and exposes an
// Execute constructor, so the name alone determines the generated import:
// see moduleExecutorPath and moduleVarName.
var knownModules = map[string]struct{}{
	"csv":                    {},
	"exec":                   {},
	"fs":                     {},
	"http":                   {},
	"json":                   {},
	"math":                   {},
	"os":                     {},
	"path":                   {},
	"rand":                   {},
	"regexp":                 {},
	"time":                   {},
	"url":                    {},
	"uuid":                   {},
	"x/archive/tar":          {},
	"x/archive/zip":          {},
	"x/compress/bzip2":       {},
	"x/compress/flate":       {},
	"x/compress/gzip":        {},
	"x/compress/lzw":         {},
	"x/compress/zlib":        {},
	"x/crypto/hmac":          {},
	"x/crypto/md5":           {},
	"x/crypto/sha1":          {},
	"x/crypto/sha256":        {},
	"x/crypto/sha512":        {},
	"x/encoding/ascii85":     {},
	"x/encoding/base32":      {},
	"x/encoding/base64":      {},
	"x/encoding/hex":         {},
	"x/encoding/pem":         {},
	"x/hash/adler32":         {},
	"x/hash/crc32":           {},
	"x/hash/crc64":           {},
	"x/hash/fnv":             {},
	"x/html":                 {},
	"x/mime":                 {},
	"x/mime/quotedprintable": {},
	"x/net/mail":             {},
	"x/net/netip":            {},
	"x/unicode":              {},
	"x/unicode/utf8":         {},
}

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

// KnownModuleNames lists the stdlib modules the transpiler can import, sorted.
// It is exported so tests can assert the interpreter's builtinModules table
// stays in sync with this one.
func KnownModuleNames() []string {
	names := make([]string, 0, len(knownModules))
	for name := range knownModules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// transpileContext holds state for a single Transpile call.
type transpileContext struct {
	localNameCounter int
	moduleImports    map[string]string   // module name -> Go variable name
	importing        map[string]struct{} // paths currently being transpiled (cycle detection)
	imported         map[string]struct{} // paths already transpiled (dedup)
	moduleFuncs      []jen.Code          // top-level module executor functions (single-file mode)
	topDecls         []jen.Code          // package-level declarations: types, methods, module variables, direct functions
	// loadedModuleVars tracks the package-level variables holding imported
	// module values, keyed by variable name, so a module imported from several
	// places in one generated file is declared exactly once. Reset alongside
	// topDecls in directory mode, where every file is its own package.
	loadedModuleVars map[string]struct{}
	// localTypes holds the specialised native types for the function body
	// currently being transpiled (see typeinfer.go). It is swapped on entry to
	// every function body and restored on exit, so it always describes exactly
	// one scope. Names absent from it are ordinary object.Object values.
	localTypes map[string]staticType
	// For directory mode:
	goModuleName string
	outputDir    string
	// Import resolution: relative import paths are resolved against the
	// directory of the module currently being transpiled (baseDir), and
	// module identifiers are derived relative to the entry point's
	// directory (rootDir).
	baseDir string
	rootDir string
	// userScopes tracks user-declared names per lexical scope so they shadow
	// built-in functions the same way the interpreter's environment chain does.
	userScopes []map[string]struct{}

	// rangeNative reports whether `range` resolves to the builtin throughout
	// the function body currently being transpiled, making `for x in
	// range(a, b)` eligible for lowering to a native counting loop. Swapped
	// alongside localTypes by enterScope; the inference pass and transpileFor
	// must agree on it, so it is a whole-body property.
	rangeNative bool

	// directFns holds the module-level functions of the module currently
	// being transpiled that are eligible for direct-call lowering (see
	// directcall.go), and moduleScopeIdx is the index of that module's user
	// scope: a call site may only be lowered while no deeper scope shadows
	// the function's name.
	directFns      map[string]directFn
	directCtors    map[string]directFn
	moduleScopeIdx int

	// selfType describes the receiver while a type method body is being
	// transpiled, so `self.field` and `self.method(...)` can be generated as
	// direct Go field access and method calls. It is nil outside method
	// bodies, inside nested function bodies, and in a method that rebinds
	// self.
	selfType *selfInfo

	// sigs holds the inferred native signatures of the current module's
	// closed direct functions (see signatures.go); retType is the native
	// return type of the function body being transpiled, tyDynamic for an
	// ordinary object.Object result. Both are swapped like localTypes.
	sigs    map[string]*fnSig
	retType staticType

	// errorPos is the position of the statement currently being transpiled.
	// Frame-producing error handlers read it so tracebacks point at the
	// failing statement, mirroring the interpreter's positionError tagging.
	errorPos token.Pos
}

// selfInfo is what direct self access needs to know about the enclosing
// type: the Go receiver variable, the field names, and the Go wrapper name of
// each method.
type selfInfo struct {
	receiver string
	fields   map[string]bool
	methods  map[string]string
}

// selfMember reports whether expr is `self.name` for a field or method of the
// enclosing type, in a position where self is still the receiver.
func (ctx *transpileContext) selfMember(expr ast.Expression, name string) (info *selfInfo, ok bool) {
	if ctx.selfType == nil {
		return nil, false
	}
	ident, isIdent := expr.(*ast.Identifier)
	if !isIdent || ident.Name != "self" {
		return nil, false
	}
	return ctx.selfType, true
}

// framePos returns the position traceback frames should carry: the current
// statement when one is being transpiled, fallback (typically the enclosing
// definition) otherwise.
func (ctx *transpileContext) framePos(fallback token.Pos) token.Pos {
	if ctx.errorPos.Line != 0 {
		return ctx.errorPos
	}
	return fallback
}

func newTranspileContext() *transpileContext {
	return &transpileContext{
		localNameCounter: 0,
		moduleImports:    make(map[string]string),
		importing:        make(map[string]struct{}),
		imported:         make(map[string]struct{}),
		moduleFuncs:      nil,
		topDecls:         nil,
		loadedModuleVars: make(map[string]struct{}),
	}
}

// pushUserScope opens a lexical scope for user-declared names and returns a
// function closing it. User names shadow built-in functions, mirroring the
// interpreter's scope-chain-first name resolution.
func (ctx *transpileContext) pushUserScope() func() {
	ctx.userScopes = append(ctx.userScopes, map[string]struct{}{})
	return func() { ctx.userScopes = ctx.userScopes[:len(ctx.userScopes)-1] }
}

func (ctx *transpileContext) declareUserName(name string) {
	if len(ctx.userScopes) > 0 {
		ctx.userScopes[len(ctx.userScopes)-1][name] = struct{}{}
	}
}

func (ctx *transpileContext) isUserName(name string) bool {
	for _, scope := range ctx.userScopes {
		if _, ok := scope[name]; ok {
			return true
		}
	}
	return false
}

// enterScope installs the type environment inferred for a function body and
// returns a function restoring the previous one. seed and ret are the body's
// parameter types and native return type when it belongs to a closed direct
// function, nil and tyDynamic otherwise.
//
// The environment must come out identical to the one the signature inference
// computed for this body, which is why it is built by the same function from
// the same inputs: the module-wide rangeNative, the seed and the signatures.
func (ctx *transpileContext) enterScope(body []ast.Statement, seed map[string]staticType, ret staticType) func() {
	savedTypes, savedRet := ctx.localTypes, ctx.retType
	ctx.localTypes = inferLocals(body, ctx.rangeNative, seed, ctx.sigs)
	ctx.retType = ret
	return func() {
		ctx.localTypes, ctx.retType = savedTypes, savedRet
	}
}

// typeOf reports the specialised native type of a name, or tyDynamic when the
// name is an ordinary boxed value.
func (ctx *transpileContext) typeOf(name string) staticType {
	if t, ok := ctx.localTypes[name]; ok {
		return t
	}
	return tyDynamic
}

// nativeTypeOf reports whether an expression can be evaluated as a native Go
// expression in the current scope, and with what type.
func (ctx *transpileContext) nativeTypeOf(expr ast.Expression) staticType {
	return staticTypeOf(expr, ctx.localTypes, ctx.sigs)
}

// withSelfType installs the receiver description for a method body (or nil to
// leave one) and returns a function restoring the previous value.
func (ctx *transpileContext) withSelfType(info *selfInfo) func() {
	saved := ctx.selfType
	ctx.selfType = info
	return func() { ctx.selfType = saved }
}

// zeroOf is the Go zero value of a native type, or nil for a boxed result;
// error paths return it alongside the error.
func zeroOf(t staticType) *jen.Statement {
	switch t {
	case tyInt, tyFloat:
		return jen.Lit(0)
	case tyBool:
		return jen.False()
	}
	return jen.Nil()
}

// box wraps a native Go expression back into an object.Object.
func box(code *jen.Statement, t staticType) *jen.Statement {
	switch t {
	case tyInt:
		return jen.Qual(pathObject, "Integer").Call(code)
	case tyFloat:
		return jen.Qual(pathObject, "Float").Call(code)
	case tyBool:
		return jen.Qual(pathObject, "Bool").Call(code)
	}
	return code
}

// emitNative renders an expression as a plain Go expression. It mirrors
// staticTypeOf exactly and must only be called when that reported a native
// type; anything else is a bug in one of the two, and is reported as such
// rather than silently miscompiled.
//
// Native expressions have no side effects and, with the exception of division
// and modulo, cannot fail — so they collapse into a single Go expression with
// no error plumbing. Division and modulo need a zero check, and that is a
// statement, hence the leading []jen.Code most callers will find empty.
func (ctx *transpileContext) emitNative(expr ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	switch e := expr.(type) {
	case *ast.Literal:
		switch v := e.Value.(type) {
		case object.Integer:
			return nil, jen.Lit(int64(v)), nil
		case object.Float:
			return nil, jen.Lit(float64(v)), nil
		case object.Bool:
			return nil, jen.Lit(bool(v)), nil
		}

	case *ast.Identifier:
		return nil, jen.Id(e.Name), nil

	case *ast.FunctionCall, *ast.CallExpression:
		// A call to a closed direct function with a native return type. The
		// call and its error check are hoisted as preceding statements and
		// the expression is the result variable.
		name, args, _ := bareCall(expr)
		pre, call, ok, err := ctx.tryDirectCall(name, args, onError)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("%s: internal error: call to %s typed native but not lowered", expr.Position(), name)
		}
		tmpVar := ctx.localName("tmp")
		errVar := ctx.localName("err")
		pre = append(pre,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return pre, jen.Id(tmpVar), nil

	case *ast.UnaryOperation:
		pre, operand, err := ctx.emitNative(e.Operand, onError)
		if err != nil {
			return nil, nil, err
		}
		return pre, jen.Parens(jen.Op(e.Operator).Add(operand)), nil

	case *ast.BinaryOperation:
		lhsPre, lhs, err := ctx.emitNative(e.LHS, onError)
		if err != nil {
			return nil, nil, err
		}
		rhsPre, rhs, err := ctx.emitNative(e.RHS, onError)
		if err != nil {
			return nil, nil, err
		}
		pre := append(lhsPre, rhsPre...)

		// Go has no implicit numeric conversion, so a mixed int/float operand
		// pair — which Goblin widens to float — needs the int side converted
		// explicitly. This covers comparisons too: Integer.Equals(Float) and
		// Integer.Compare(Float) both compare as float64.
		lhsType := ctx.nativeTypeOf(e.LHS)
		rhsType := ctx.nativeTypeOf(e.RHS)
		if numeric(lhsType) && numeric(rhsType) && lhsType != rhsType {
			if lhsType == tyInt {
				lhs = jen.Float64().Call(lhs)
			} else {
				rhs = jen.Float64().Call(rhs)
			}
		}

		if e.Operator == "/" {
			// Bind the divisor so it is evaluated once, then reproduce
			// Goblin's ZeroDivisionError instead of Go's panic (integers) or
			// ±Inf (floats).
			divisor := ctx.localName("div")
			errVar := ctx.localName("err")
			pre = append(pre,
				jen.Id(divisor).Op(":=").Add(rhs),
				jen.If(jen.Id(divisor).Op("==").Lit(0)).Block(
					jen.Id(errVar).Op(":=").Qual(pathObject, "NewZeroDivisionError").Call(jen.Lit("division by zero")),
					onError(errVar),
				),
			)
			return pre, jen.Parens(lhs.Op("/").Id(divisor)), nil
		}

		if e.Operator == "%" {
			// Modulo needs a zero check like division, and for floats it must
			// go through math.Mod since Go's % only works on integers.
			divisor := ctx.localName("mod")
			errVar := ctx.localName("err")
			pre = append(pre,
				jen.Id(divisor).Op(":=").Add(rhs),
				jen.If(jen.Id(divisor).Op("==").Lit(0)).Block(
					jen.Id(errVar).Op(":=").Qual(pathObject, "NewZeroDivisionError").Call(jen.Lit("modulo by zero")),
					onError(errVar),
				),
			)
			if lhsType == tyInt && rhsType == tyInt {
				return pre, jen.Parens(lhs.Op("%").Id(divisor)), nil
			}
			// Mixed numeric operands were widened above, so both arguments are
			// float64 whenever this is not the integer-only case.
			return pre, jen.Parens(jen.Qual("math", "Mod").Call(lhs, jen.Id(divisor))), nil
		}

		// Goblin's operator spellings for arithmetic, comparison, && and ||
		// are the same as Go's, so the operator carries over verbatim.
		return pre, jen.Parens(lhs.Op(e.Operator).Add(rhs)), nil
	}

	return nil, nil, fmt.Errorf("internal error: %T is not a native expression", expr)
}

func (ctx *transpileContext) localName(prefix string) string {
	name := fmt.Sprintf("_%s_%d", prefix, ctx.localNameCounter)
	ctx.localNameCounter++
	return name
}

func (ctx *transpileContext) goTypeName(name string) string {
	return name
}

func exportedName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// reservedGoMethodNames are the object.Object interface methods (plus the
// optional setter interfaces) generated on every user type's struct. A
// user-defined goblin method whose exported name collides with one of these
// must be mangled so both can coexist on the same receiver.
var reservedGoMethodNames = map[string]bool{
	"String": true, "Bool": true, "Equals": true, "Compare": true, "Add": true,
	"Minus": true, "Multiply": true, "Divide": true, "Modulo": true,
	"Iter": true, "Index": true,
	"GetAttr": true, "Attributes": true, "SetAttr": true, "SetIndex": true,
	"TypeName": true, "ToString": true, "ToBool": true,
	"RAdd": true, "RMinus": true, "RMultiply": true, "RDivide": true, "RModulo": true,
	"Hash": true, "UserType": true, "FieldValues": true, "CallMethod": true,
}

// methodWrapperName returns the Go method name for a user-defined goblin
// method, mangled to avoid colliding with the interface methods generated on
// the same struct (e.g. goblin `add` -> Go `Add_`).
func methodWrapperName(name string) string {
	n := exportedName(name)
	if reservedGoMethodNames[n] {
		return n + "_"
	}
	return n
}

// errHandler generates the error-handling code for a given error variable name.
type errHandler func(errVar string) jen.Code

func frameCode(module, function string, pos token.Pos) *jen.Statement {
	file := ""
	if src, ok := pos.Context.(token.Sourcer); ok && src != nil {
		file = src.Source()
	}
	return jen.Qual(pathObject, "Frame").Values(jen.Dict{
		jen.Id("Module"):   jen.Lit(module),
		jen.Id("Function"): jen.Lit(function),
		jen.Id("File"):     jen.Lit(file),
		jen.Id("Line"):     jen.Lit(pos.Line),
		jen.Id("Column"):   jen.Lit(pos.Column),
	})
}

func tracedReturn(zero *jen.Statement, errVar, module, function string, pos token.Pos) jen.Code {
	return jen.Return(zero, jen.Qual(pathObject, "WithFrame").Call(
		jen.Id(errVar), frameCode(module, function, pos),
	))
}

func modulePosition(mod *ast.Module) token.Pos {
	if len(mod.Body) > 0 {
		return mod.Body[0].Position()
	}
	return token.Pos{}
}

// sourceModuleName derives the traceback module name from a position's source
// file via the shared source.ModuleName, so frames from both backends carry
// the same module tag.
func sourceModuleName(pos token.Pos) string {
	if src, ok := pos.Context.(token.Sourcer); ok && src != nil {
		return source.ModuleName(src.Source())
	}
	return ""
}

func isPathImport(path string) bool {
	return source.IsPathImport(path)
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
// first; stdlib imports are validated against knownModules. importPath is the
// import path of the module being collected, empty for the entry module (it
// only affects error wording).
func (ctx *transpileContext) collectModuleImports(mod *ast.Module, importPath string, recurse func(string) error) (map[string]string, error) {
	imports := make(map[string]string)
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok {
			continue
		}
		if isPathImport(imp.Path) {
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
			if _, exists := knownModules[imp.Path]; !exists {
				if importPath == "" {
					return nil, fmt.Errorf("unknown module: %s", imp.Path)
				}
				return nil, fmt.Errorf("unknown module in %s: %s", importPath, imp.Path)
			}
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

	l, err := source.NewLexerFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read module %s: %v", importPath, err)
	}
	mod, err := source.Parse(l)
	if err != nil {
		return fmt.Errorf("parse error in module %s: %v", importPath, err)
	}
	if err := semantic.CheckModule(mod); err != nil {
		return fmt.Errorf("semantic error in module %s: %v", importPath, err)
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
		if !ok || isPathImport(imp.Path) {
			continue
		}
		loadModule(imports[imp.Name], jen.Lit(imp.Path), jen.Qual(moduleExecutorPath(imp.Path), "Execute"))
	}

	// Path module imports via registry
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
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

func Transpile(mod *ast.Module, output io.Writer) error {
	if err := semantic.CheckModule(mod); err != nil {
		return err
	}

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

func transpileObject(obj object.Object) (*jen.Statement, error) {
	switch v := obj.(type) {
	case object.Bool:
		if bool(v) {
			return jen.Qual(pathObject, "True"), nil
		}
		return jen.Qual(pathObject, "False"), nil
	case object.Unit:
		return jen.Qual(pathObject, "Nil"), nil
	case object.Integer:
		i := jen.Qual(pathObject, "Integer").Call(jen.Lit(int64(v)))
		return i, nil
	case object.Float:
		f := jen.Qual(pathObject, "Float").Call(jen.Lit(float64(v)))
		return f, nil
	case object.String:
		s := jen.Qual(pathObject, "String").Call(jen.Lit(string(v)))
		return s, nil
	}
	return nil, fmt.Errorf("cannot transpile literal of type %s", obj.TypeName())
}

func (ctx *transpileContext) transpileListLiteral(list *ast.ListLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	preStmts, elements, err := ctx.transpileExpressions(list.Elements, onError)
	if err != nil {
		return nil, nil, err
	}

	return preStmts, jen.Op("&").Qual(pathObject, "List").Values(
		jen.Id("Elements").Op(":").Index().Qual(pathObject, "Object").Values(elements...),
	), nil
}

func (ctx *transpileContext) transpileIndexExpression(expr *ast.IndexExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	objPre, obj, err := ctx.transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}
	idxPre, idx, native, err := ctx.transpileIndexOperand(expr.Index, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(objPre, idxPre...)
	var call *jen.Statement
	if native {
		call = jen.Qual(pathObject, "IndexInt").Call(obj, idx)
	} else {
		call = jen.Add(obj).Dot("Index").Call(idx)
	}
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileIndexOperand renders the index of `obj[index]` or `obj[index] = v`.
// An index that is statically an integer stays a native int64 and native
// reports true, so the caller can reach for object.IndexInt / SetIndexInt and
// skip both the boxing (an allocation for anything above 255) and the
// interface dispatch; anything else is boxed as usual.
func (ctx *transpileContext) transpileIndexOperand(index ast.Expression, onError errHandler) (pre []jen.Code, code *jen.Statement, native bool, err error) {
	if ctx.nativeTypeOf(index) == tyInt {
		pre, code, err = ctx.emitNative(index, onError)
		return pre, code, true, err
	}
	pre, code, err = ctx.transpileExpression(index, onError)
	return pre, code, false, err
}

func (ctx *transpileContext) transpileDictLiteral(dict *ast.DictLiteral, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	var preStmts []jen.Code

	dictVar := ctx.localName("dict")
	preStmts = append(preStmts,
		jen.Id(dictVar).Op(":=").Qual(pathObject, "NewDict").Call(),
	)

	for _, elem := range dict.Elements {
		keyPre, key, err := ctx.transpileExpression(elem.Key, onError)
		if err != nil {
			return nil, nil, err
		}
		valuePre, value, err := ctx.transpileExpression(elem.Value, onError)
		if err != nil {
			return nil, nil, err
		}
		preStmts = append(preStmts, keyPre...)
		preStmts = append(preStmts, valuePre...)
		errVar := ctx.localName("err")
		preStmts = append(preStmts,
			jen.If(
				jen.Id(errVar).Op(":=").Id(dictVar).Dot("Set").Call(key, value),
				jen.Id(errVar).Op("!=").Nil(),
			).Block(onError(errVar)),
		)
	}

	return preStmts, jen.Id(dictVar), nil
}

func (ctx *transpileContext) transpileMemberExpression(expr *ast.MemberExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	// `self.field` inside a method is a plain Go field read on the receiver:
	// no GetAttr dispatch, no error path.
	if info, ok := ctx.selfMember(expr.Object, expr.Property); ok && info.fields[expr.Property] {
		return nil, jen.Id(info.receiver).Dot(fieldGoName(expr.Property)), nil
	}

	objPre, obj, err := ctx.transpileExpression(expr.Object, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := ctx.localName("attr")
	errVar := ctx.localName("err")
	preStmts := append(objPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(obj)).Dot("GetAttr").Call(jen.Lit(expr.Property)),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

func (ctx *transpileContext) transpileExpression(expr ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	// If the whole subtree evaluates natively, emit it as one Go expression and
	// box the result once, instead of one interface call and one allocation per
	// operator. Callers that can consume the unboxed value (declarations,
	// assignments, conditions) check for this themselves before calling here.
	if t := ctx.nativeTypeOf(expr); t.native() {
		pre, code, err := ctx.emitNative(expr, onError)
		if err != nil {
			return nil, nil, err
		}
		return pre, box(code, t), nil
	}

	switch v := expr.(type) {
	case *ast.Literal:
		obj, err := transpileObject(v.Value)
		if err != nil {
			return nil, nil, err
		}
		return nil, obj, nil
	case *ast.Identifier:
		if moduleVar, ok := ctx.moduleBinding(v.Name); ok {
			return nil, jen.Id(moduleVar), nil
		}
		if !ctx.isUserName(v.Name) && isBuiltinFunction(v.Name) {
			return nil, jen.Id("builtin").Dot("Members").Index(jen.Lit(v.Name)), nil
		}
		return nil, jen.Id(v.Name), nil
	case *ast.FunctionCall:
		argPreStmts, call, err := ctx.transpileFunctionCall(v, onError)
		if err != nil {
			return nil, nil, err
		}
		tmpVar := ctx.localName("tmp")
		errVar := ctx.localName("err")
		preStmts := append(argPreStmts,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Id(tmpVar), nil
	case *ast.CallExpression:
		argPreStmts, call, err := ctx.transpileCallExpression(v, onError)
		if err != nil {
			return nil, nil, err
		}
		tmpVar := ctx.localName("tmp")
		errVar := ctx.localName("err")
		preStmts := append(argPreStmts,
			jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Id(tmpVar), nil
	case *ast.BinaryOperation:
		return ctx.transpileBinaryOperation(v, onError)
	case *ast.UnaryOperation:
		return ctx.transpileUnaryOperation(v, onError)
	case *ast.ListLiteral:
		return ctx.transpileListLiteral(v, onError)
	case *ast.DictLiteral:
		return ctx.transpileDictLiteral(v, onError)
	case *ast.IndexExpression:
		return ctx.transpileIndexExpression(v, onError)
	case *ast.MemberExpression:
		return ctx.transpileMemberExpression(v, onError)
	case *ast.FunctionLiteral:
		return ctx.transpileFunctionLiteral(v)
	}
	return nil, nil, fmt.Errorf("%s: cannot transpile expression of type %T", expr.Position(), expr)
}

func (ctx *transpileContext) transpileExpressions(exprs []ast.Expression, onError errHandler) ([]jen.Code, []jen.Code, error) {
	var allPreStmts []jen.Code
	var results []jen.Code
	for _, expr := range exprs {
		pre, r, err := ctx.transpileExpression(expr, onError)
		if err != nil {
			return nil, nil, err
		}
		allPreStmts = append(allPreStmts, pre...)
		results = append(results, r)
	}
	return allPreStmts, results, nil
}

func (ctx *transpileContext) transpileCallArguments(args []ast.CallArgument, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	// Fast path: without * and ** the whole argument list is known here, so it
	// can be one composite literal instead of a callArgs temporary plus an
	// append or AddKeyword statement per argument. Beyond the statements it
	// saves, this is what keeps the argument slice on the stack: a value that
	// is only ever built and passed does not escape, while one that is
	// assigned to a local and mutated does.
	//
	// Duplicate keyword names are the one case that still needs AddKeyword,
	// which raises the same error both backends raise.
	staticShape := true
	seenKeyword := make(map[string]bool, len(args))
	for _, arg := range args {
		switch arg.Kind {
		case ast.CallArgumentPositional:
		case ast.CallArgumentKeyword:
			if seenKeyword[arg.Name] {
				staticShape = false
			}
			seenKeyword[arg.Name] = true
		default:
			staticShape = false
		}
		if !staticShape {
			break
		}
	}
	if staticShape {
		var preStmts []jen.Code
		positional := make([]jen.Code, 0, len(args))
		keyword := make([]jen.Code, 0, len(args))
		for _, arg := range args {
			argPreStmts, argExpr, err := ctx.transpileExpression(arg.Expr, onError)
			if err != nil {
				return nil, nil, err
			}
			preStmts = append(preStmts, argPreStmts...)
			if arg.Kind == ast.CallArgumentKeyword {
				keyword = append(keyword, jen.Values(
					jen.Id("Name").Op(":").Lit(arg.Name),
					jen.Id("Value").Op(":").Add(argExpr),
				))
				continue
			}
			positional = append(positional, argExpr)
		}
		fields := jen.Dict{}
		if len(positional) > 0 {
			fields[jen.Id("Positional")] = jen.Qual(pathObject, "Args").Values(positional...)
		}
		if len(keyword) > 0 {
			fields[jen.Id("Keyword")] = jen.Qual(pathObject, "Kwargs").Values(keyword...)
		}
		return preStmts, jen.Qual(pathObject, "CallArgs").Values(fields), nil
	}

	callArgsVar := ctx.localName("callArgs")
	allPreStmts := []jen.Code{
		jen.Id(callArgsVar).Op(":=").Qual(pathObject, "CallArgs").Values(),
	}

	for _, arg := range args {
		argPreStmts, argExpr, err := ctx.transpileExpression(arg.Expr, onError)
		if err != nil {
			return nil, nil, err
		}
		allPreStmts = append(allPreStmts, argPreStmts...)

		switch arg.Kind {
		case ast.CallArgumentPositional:
			allPreStmts = append(allPreStmts,
				jen.Id(callArgsVar).Dot("Positional").Op("=").Append(jen.Id(callArgsVar).Dot("Positional"), argExpr),
			)
		case ast.CallArgumentStarred:
			iterVar := ctx.localName("iter")
			errVar := ctx.localName("err")
			allPreStmts = append(allPreStmts,
				jen.List(jen.Id(iterVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(argExpr)).Dot("Iter").Call(),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id(callArgsVar).Dot("Positional").Op("=").Append(jen.Id(callArgsVar).Dot("Positional"), jen.Id(iterVar).Op("...")),
			)
		case ast.CallArgumentKeyword:
			// Duplicate detection lives in AddKeyword, shared with the
			// interpreter.
			errVar := ctx.localName("err")
			allPreStmts = append(allPreStmts,
				jen.If(
					jen.Id(errVar).Op(":=").Id(callArgsVar).Dot("AddKeyword").Call(jen.Lit(arg.Name), argExpr),
					jen.Id(errVar).Op("!=").Nil(),
				).Block(onError(errVar)),
			)
		case ast.CallArgumentKeywordUnpack:
			// Dict/key validation and duplicate detection live in
			// UnpackKeywords, shared with the interpreter.
			errVar := ctx.localName("err")
			allPreStmts = append(allPreStmts,
				jen.If(
					jen.Id(errVar).Op(":=").Id(callArgsVar).Dot("UnpackKeywords").Call(argExpr),
					jen.Id(errVar).Op("!=").Nil(),
				).Block(onError(errVar)),
			)
		}
	}

	return allPreStmts, jen.Id(callArgsVar), nil
}

func isBuiltinFunction(name string) bool {
	_, ok := extension.BuiltinsModule.Members[name]
	return ok
}

// directBuiltin names the Go function behind a built-in, so a call to it can
// be emitted as a plain Go call instead of a map lookup followed by
// object.Call through a function value. Skipping the function value is also
// what lets the argument list stay on the stack: escape analysis cannot see
// through an indirect call.
//
// fn is the same function the builtins module holds, and
// TestDirectBuiltinsMatchModule checks that pair by pointer, so a rename or a
// re-pointed member cannot make the emitted call go somewhere else.
type directBuiltin struct {
	path string
	name string
	fn   func(object.CallArgs) (object.Object, error)
}

var directBuiltins = map[string]directBuiltin{
	"print":    {pathExtension, "Print", extension.Print},
	"eprint":   {pathExtension, "Eprint", extension.Eprint},
	"spawn":    {pathExtension, "Spawn", extension.Spawn},
	"range":    {pathExtension, "Range", extension.Range},
	"max":      {pathExtension, "Max", extension.Max},
	"min":      {pathExtension, "Min", extension.Min},
	"Error":    {pathObject, "ErrorConstructor", object.ErrorConstructor},
	"Int":      {pathObject, "IntConstructor", object.IntConstructor},
	"Float":    {pathObject, "FloatConstructor", object.FloatConstructor},
	"Str":      {pathObject, "StrConstructor", object.StrConstructor},
	"Bytes":    {pathObject, "BytesConstructor", object.BytesConstructor},
	"Bool":     {pathObject, "BoolConstructor", object.BoolConstructor},
	"List":     {pathObject, "ListConstructor", object.ListConstructor},
	"Dict":     {pathObject, "DictConstructor", object.DictConstructor},
	"Chan":     {pathObject, "ChanConstructor", object.ChanConstructor},
	"Goblin":   {pathObject, "GoblinConstructor", object.GoblinConstructor},
	"Function": {pathObject, "FunctionConstructor", object.FunctionConstructor},
}

func (ctx *transpileContext) transpileDeclare(decl *ast.Declare, onError errHandler) ([]jen.Code, error) {
	if t := ctx.typeOf(decl.Name); t.native() {
		pre, value, err := ctx.emitNative(decl.Value, onError)
		if err != nil {
			return nil, err
		}
		ctx.declareUserName(decl.Name)
		declStmt := jen.Var().Id(decl.Name).Id(goTypeOf(t)).Op("=").Add(value)
		declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
		return append(pre, declStmt), nil
	}

	preStmts, value, err := ctx.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	ctx.declareUserName(decl.Name)
	declStmt := jen.Var().Id(decl.Name).Qual(pathObject, "Object").Op("=").Add(value)
	declStmt.Op(";").Id("_").Op("=").Id(decl.Name)
	return append(preStmts, declStmt), nil
}

func (ctx *transpileContext) transpileAssign(decl *ast.Assign, onError errHandler) ([]jen.Code, error) {
	if ctx.typeOf(decl.Target).native() {
		pre, value, err := ctx.emitNative(decl.Value, onError)
		if err != nil {
			return nil, err
		}
		assignStmt := jen.Id(decl.Target).Op("=").Add(value)
		assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
		return append(pre, assignStmt), nil
	}

	preStmts, value, err := ctx.transpileExpression(decl.Value, onError)
	if err != nil {
		return nil, err
	}
	assignStmt := jen.Id(decl.Target).Op("=").Add(value)
	assignStmt.Op(";").Id("_").Op("=").Id(decl.Target)
	return append(preStmts, assignStmt), nil
}

func (ctx *transpileContext) transpileSetIndex(s *ast.SetIndex, onError errHandler) ([]jen.Code, error) {
	objPre, obj, err := ctx.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	idxPre, idx, native, err := ctx.transpileIndexOperand(s.Index, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := ctx.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	setter := "SetIndex"
	if native {
		setter = "SetIndexInt"
	}
	errVar := ctx.localName("err")
	stmts := append(objPre, idxPre...)
	stmts = append(stmts, valPre...)
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, setter).Call(obj, idx, val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (ctx *transpileContext) transpileSetAttr(s *ast.SetAttr, onError errHandler) ([]jen.Code, error) {
	// `self.field = v` inside a method is a plain Go field store.
	if info, ok := ctx.selfMember(s.Object, s.Property); ok && info.fields[s.Property] {
		valPre, val, err := ctx.transpileExpression(s.Value, onError)
		if err != nil {
			return nil, err
		}
		return append(valPre, jen.Id(info.receiver).Dot(fieldGoName(s.Property)).Op("=").Add(val)), nil
	}

	objPre, obj, err := ctx.transpileExpression(s.Object, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := ctx.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	errVar := ctx.localName("err")
	stmts := append(objPre, valPre...)
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "SetAttr").Call(obj, jen.Lit(s.Property), val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (ctx *transpileContext) transpileIfElse(ifelse *ast.IfElse, onError errHandler) ([]jen.Code, error) {
	pre, cond, err := ctx.transpileCondition(ifelse.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(ifelse.IfBody, onError, "")
	if err != nil {
		return nil, err
	}
	elseBody, err := ctx.transpileStatements(ifelse.ElseBody, onError, "")
	if err != nil {
		return nil, err
	}
	return append(pre, jen.If(cond).Block(body...).Else().Block(elseBody...)), nil
}

func (ctx *transpileContext) transpileWhile(while_ *ast.While, onError errHandler) ([]jen.Code, error) {
	condPre, cond, err := ctx.transpileCondition(while_.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(while_.Body, onError, "")
	if err != nil {
		return nil, err
	}
	// A condition without preceding statements is a plain Go for-condition,
	// which is what lets a hot numeric loop compile down to an ordinary Go
	// `for`. Statements the condition needs (a zero check, a call, a boxed
	// comparison) have to run every iteration, so the loop then becomes
	// `for { statements; if !cond { break }; body }`.
	if len(condPre) == 0 {
		return []jen.Code{jen.For(cond).Block(body...)}, nil
	}
	loopBody := append([]jen.Code{}, condPre...)
	loopBody = append(loopBody, jen.If(jen.Op("!").Parens(cond)).Block(jen.Break()))
	loopBody = append(loopBody, body...)
	return []jen.Code{jen.For().Block(loopBody...)}, nil
}

func (ctx *transpileContext) transpileBreak(break_ *ast.Break) ([]jen.Code, error) {
	return []jen.Code{jen.Break()}, nil
}

func (ctx *transpileContext) transpileContinue(continue_ *ast.Continue) ([]jen.Code, error) {
	return []jen.Code{jen.Continue()}, nil
}

func (ctx *transpileContext) transpileFor(for_ *ast.For, onError errHandler) ([]jen.Code, error) {
	// `for x in range(a, b)` runs as a native counting loop when `range` is
	// known to be the builtin, instead of materialising a list of boxed
	// integers only to iterate it once. The condition must stay in lockstep
	// with collectDeclarations, which types the loop variable accordingly.
	if ctx.rangeNative {
		if startExpr, endExpr, ok := rangeCallParts(for_.Iterator); ok {
			return ctx.transpileRangeFor(for_, startExpr, endExpr, onError)
		}
	}

	iterPreStmts, iterator, err := ctx.transpileExpression(for_.Iterator, onError)
	if err != nil {
		return nil, err
	}
	popScope := ctx.pushUserScope()
	ctx.declareUserName(for_.Variable)
	body, err := ctx.transpileStatements(for_.Body, onError, "")
	popScope()
	if err != nil {
		return nil, err
	}

	iterVar := ctx.localName("iter")
	elementsVar := ctx.localName("elements")
	errVar := ctx.localName("err")

	forLoopBody := []jen.Code{
		jen.Id(for_.Variable).Op(":=").Id(iterVar),
		jen.Id("_").Op("=").Id(for_.Variable),
	}
	forLoopBody = append(forLoopBody, body...)

	result := append(iterPreStmts,
		jen.List(jen.Id(elementsVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(iterator)).Dot("Iter").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.For(jen.List(jen.Id("_"), jen.Id(iterVar)).Op(":=").Op("range").Id(elementsVar)).Block(forLoopBody...),
	)

	return []jen.Code{jen.Block(result...)}, nil
}

// transpileRangeFor lowers `for x in range(a, b)` onto a native Go counting
// loop. The bounds are evaluated once, before the loop, exactly as range()'s
// arguments would be; when they are not provably Integer the shared
// extension.RangeBounds helper validates them, so a bad bound reports the very
// error range() itself raises. The loop variable is a fresh binding each
// iteration — assigning it in the body never affects the iteration, matching
// list iteration semantics — and is declared as a native int64 when the
// inference pass proved every assignment to it in this body stays Integer.
func (ctx *transpileContext) transpileRangeFor(for_ *ast.For, startExpr, endExpr ast.Expression, onError errHandler) ([]jen.Code, error) {
	counterVar := ctx.localName("range_i")
	endVar := ctx.localName("range_end")

	var pre []jen.Code
	var startCode *jen.Statement
	if ctx.nativeTypeOf(startExpr) == tyInt && ctx.nativeTypeOf(endExpr) == tyInt {
		startPre, start, err := ctx.emitNative(startExpr, onError)
		if err != nil {
			return nil, err
		}
		endPre, end, err := ctx.emitNative(endExpr, onError)
		if err != nil {
			return nil, err
		}
		pre = append(append(pre, startPre...), endPre...)
		pre = append(pre, jen.Id(endVar).Op(":=").Add(end))
		startCode = start
	} else {
		startPre, start, err := ctx.transpileExpression(startExpr, onError)
		if err != nil {
			return nil, err
		}
		endPre, end, err := ctx.transpileExpression(endExpr, onError)
		if err != nil {
			return nil, err
		}
		startVar := ctx.localName("range_start")
		errVar := ctx.localName("err")
		pre = append(append(append(pre, startPre...), endPre...),
			jen.List(jen.Id(startVar), jen.Id(endVar), jen.Id(errVar)).Op(":=").Qual(pathExtension, "RangeBounds").Call(start, end),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		startCode = jen.Id(startVar)
	}

	popScope := ctx.pushUserScope()
	ctx.declareUserName(for_.Variable)
	body, err := ctx.transpileStatements(for_.Body, onError, "")
	popScope()
	if err != nil {
		return nil, err
	}

	loopBody := make([]jen.Code, 0, len(body)+2)
	if ctx.typeOf(for_.Variable) == tyInt {
		loopBody = append(loopBody, jen.Id(for_.Variable).Op(":=").Id(counterVar))
	} else {
		loopBody = append(loopBody,
			jen.Var().Id(for_.Variable).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Integer").Call(jen.Id(counterVar)),
		)
	}
	loopBody = append(loopBody, jen.Id("_").Op("=").Id(for_.Variable))
	loopBody = append(loopBody, body...)

	result := append(pre, jen.For(
		jen.Id(counterVar).Op(":=").Add(startCode),
		jen.Id(counterVar).Op("<").Id(endVar),
		jen.Id(counterVar).Op("++"),
	).Block(loopBody...))

	return []jen.Code{jen.Block(result...)}, nil
}

func (ctx *transpileContext) transpileFunctionCall(call *ast.FunctionCall, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if pre, direct, ok, err := ctx.tryDirectCall(call.Name, call.Args, onError); ok || err != nil {
		return pre, direct, err
	}

	argPreStmts, args, err := ctx.transpileCallArguments(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}

	var callee *jen.Statement
	if mapped, ok := ctx.moduleBinding(call.Name); ok {
		callee = jen.Id(mapped)
	} else if !ctx.isUserName(call.Name) && isBuiltinFunction(call.Name) {
		callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(call.Name))
	} else {
		callee = jen.Id(call.Name)
	}

	return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
}

func (ctx *transpileContext) transpileCallExpression(call *ast.CallExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if ident, ok := call.Callee.(*ast.Identifier); ok {
		if pre, direct, lowered, err := ctx.tryDirectCall(ident.Name, call.Args, onError); lowered || err != nil {
			return pre, direct, err
		}
	}

	argPreStmts, args, err := ctx.transpileCallArguments(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}

	if ident, ok := call.Callee.(*ast.Identifier); ok {
		var callee *jen.Statement
		if mapped, ok := ctx.moduleBinding(ident.Name); ok {
			callee = jen.Id(mapped)
		} else if !ctx.isUserName(ident.Name) && isBuiltinFunction(ident.Name) {
			if direct, ok := directBuiltins[ident.Name]; ok {
				return argPreStmts, jen.Qual(direct.path, direct.name).Call(args), nil
			}
			callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(ident.Name))
		} else {
			callee = jen.Id(ident.Name)
		}
		return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
	}

	if member, ok := call.Callee.(*ast.MemberExpression); ok {
		// `self.method(...)` inside a method calls the sibling's Go wrapper
		// directly. A field of the same name would take precedence at
		// runtime (GetAttr checks fields first), so it stays generic then.
		if info, isSelf := ctx.selfMember(member.Object, member.Property); isSelf && !info.fields[member.Property] {
			if wrapper, isMethod := info.methods[member.Property]; isMethod {
				return argPreStmts, jen.Id(info.receiver).Dot(wrapper).Call(args), nil
			}
		}
		objPre, obj, err := ctx.transpileExpression(member.Object, onError)
		if err != nil {
			return nil, nil, err
		}
		// object.CallMethod runs a method without materializing the bound
		// function that GetAttr would have to allocate. It falls back to
		// GetAttr itself for anything that is not a method, so a field
		// holding a function still works and error messages are unchanged.
		preStmts := append(objPre, argPreStmts...)
		return preStmts, jen.Qual(pathObject, "CallMethod").Call(obj, jen.Lit(member.Property), args), nil
	}

	calleePre, callee, err := ctx.transpileExpression(call.Callee, onError)
	if err != nil {
		return nil, nil, err
	}
	preStmts := append(calleePre, argPreStmts...)
	return preStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
}

// emitParamDefaults emits the declaration of an []object.ParamDefault literal
// holding one lazy closure per defaulted parameter (nil per required one). It
// returns nil code and an empty name when no parameter declares a default.
// Callers must place the declaration before the parameter variable
// declarations: the closures must capture the enclosing scope, not the
// parameters a default expression may share a name with.
func (ctx *transpileContext) emitParamDefaults(params []*ast.Parameter) (jen.Code, string, error) {
	hasDefault := false
	for _, param := range params {
		if param.HasDefault() {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		return nil, "", nil
	}

	onError := func(errVar string) jen.Code {
		return jen.Return(jen.Nil(), jen.Id(errVar))
	}

	var entries []jen.Code
	for _, param := range params {
		if param.VarArgs || param.KwArgs {
			continue
		}
		if !param.HasDefault() {
			entries = append(entries, jen.Nil())
			continue
		}
		pre, value, err := ctx.transpileExpression(param.Default, onError)
		if err != nil {
			return nil, "", err
		}
		body := append(pre, jen.Return(value, jen.Nil()))
		entries = append(entries, jen.Func().Params().Parens(jen.List(
			jen.Qual(pathObject, "Object"), jen.Id("error")),
		).Block(body...))
	}

	name := ctx.localName("defaults")
	decl := jen.Id(name).Op(":=").Index().Qual(pathObject, "ParamDefault").Values(entries...)
	return decl, name, nil
}

// emitParameterBinding emits the statements that bind a CallArgs value (the
// local named callArgsName) to one Go variable per parameter. It splits params
// into fixed / *varargs / **kwargs, calls object.BindArguments, and unpacks the
// slice it returns. name is used only for BindArguments diagnostics, defaultsName
// names the enclosing []object.ParamDefault emitted by emitParamDefaults (""
// when no parameter has a default), and fnOnError builds the error-return
// emitted when binding fails. It is shared by named functions, anonymous
// literals, and type methods.
func (ctx *transpileContext) emitParameterBinding(name string, params []*ast.Parameter, defaultsName string, callArgsName string, fnOnError errHandler) []jen.Code {
	var varArgsParam *ast.Parameter
	var kwArgsParam *ast.Parameter
	fixedParams := make([]*ast.Parameter, 0, len(params))
	for _, param := range params {
		switch {
		case param.VarArgs:
			varArgsParam = param
		case param.KwArgs:
			kwArgsParam = param
		default:
			fixedParams = append(fixedParams, param)
		}
	}

	fixedParamNames := make([]jen.Code, 0, len(fixedParams))
	for _, param := range fixedParams {
		fixedParamNames = append(fixedParamNames, jen.Lit(param.Name))
	}

	boundName := ctx.localName("bound")
	errVar := ctx.localName("err")
	varArgsName := ""
	if varArgsParam != nil {
		varArgsName = varArgsParam.Name
	}
	kwArgsName := ""
	if kwArgsParam != nil {
		kwArgsName = kwArgsParam.Name
	}

	bindCall := func() jen.Code {
		defaultsArg := jen.Nil()
		if defaultsName != "" {
			defaultsArg = jen.Id(defaultsName)
		}
		return jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(name),
			jen.Index().String().Values(fixedParamNames...),
			defaultsArg,
			jen.Lit(varArgsName),
			jen.Lit(kwArgsName),
			jen.Id(callArgsName),
		)
	}

	// A function without varargs/kwargs that is called with exactly its fixed
	// parameters, all positional, needs no binding at all: the parameter
	// names and their positions are known here, at transpile time. The call
	// shape is not — functions are first-class values — so the check is emitted
	// into the generated code, with BindArguments kept as the fallback so
	// diagnostics for every other shape stay identical.
	if varArgsParam == nil && kwArgsParam == nil {
		stmts := make([]jen.Code, 0, len(fixedParams)*2+1)
		for _, param := range fixedParams {
			stmts = append(stmts,
				jen.Var().Id(param.Name).Qual(pathObject, "Object"),
				jen.Id("_").Op("=").Id(param.Name),
			)
		}

		fastBody := make([]jen.Code, 0, len(fixedParams))
		for i, param := range fixedParams {
			fastBody = append(fastBody,
				jen.Id(param.Name).Op("=").Id(callArgsName).Dot("Positional").Index(jen.Lit(i)),
			)
		}

		slowBody := []jen.Code{
			bindCall(),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(fnOnError(errVar)),
			// A zero-parameter function still needs the call for its arity
			// check, but then never reads the result.
			jen.Id("_").Op("=").Id(boundName),
		}
		for i, param := range fixedParams {
			slowBody = append(slowBody,
				jen.Id(param.Name).Op("=").Id(boundName).Index(jen.Lit(i)),
			)
		}

		fastCond := jen.Len(jen.Id(callArgsName).Dot("Keyword")).Op("==").Lit(0).
			Op("&&").
			Len(jen.Id(callArgsName).Dot("Positional")).Op("==").Lit(len(fixedParams))

		return append(stmts, jen.If(fastCond).Block(fastBody...).Else().Block(slowBody...))
	}

	stmts := []jen.Code{
		bindCall(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(fnOnError(errVar)),
		jen.Id("_").Op("=").Id(boundName),
	}

	// BindArguments returns one slot per fixed parameter in declaration
	// order, followed by the *varargs list and the **kwargs dict.
	slot := 0
	emit := func(param *ast.Parameter) {
		stmts = append(stmts,
			jen.Var().Id(param.Name).Qual(pathObject, "Object").Op("=").Id(boundName).Index(jen.Lit(slot)),
			jen.Id("_").Op("=").Id(param.Name),
		)
		slot++
	}
	for _, param := range fixedParams {
		emit(param)
	}
	if varArgsParam != nil {
		emit(varArgsParam)
	}
	if kwArgsParam != nil {
		emit(kwArgsParam)
	}

	return stmts
}

// buildFunctionValue emits an `&object.Function{...}` expression that wraps a
// closure binding the given parameters and running the given body. It is shared
// by named function definitions and anonymous function literals. name is used
// only for the runtime function's repr and for BindArguments diagnostics.
func (ctx *transpileContext) buildFunctionValue(name string, pos token.Pos, params []*ast.Parameter, body []ast.Statement) (*jen.Statement, error) {
	// Default expressions belong to the enclosing scope, so they are transpiled
	// before entering the body's inference scope.
	defaultsDecl, defaultsName, err := ctx.emitParamDefaults(params)
	if err != nil {
		return nil, err
	}

	// Each function body gets its own inferred types; parameters stay boxed,
	// since a caller can pass anything. Parameters are declared before
	// enterScope so the inference pass sees them as potential shadows.
	defer ctx.pushUserScope()()
	for _, param := range params {
		ctx.declareUserName(param.Name)
	}
	defer ctx.enterScope(body, nil, tyDynamic)()
	defer ctx.withSelfType(nil)()

	callArgsName := ctx.localName("callArgs")

	module := sourceModuleName(pos)
	fnOnError := func(errVar string) jen.Code {
		return tracedReturn(jen.Nil(), errVar, module, name, ctx.framePos(pos))
	}

	// Binding errors point at the definition, like the interpreter's frame
	// for BindArgumentsInto failures.
	savedPos := ctx.errorPos
	ctx.errorPos = pos
	argsDefine := ctx.emitParameterBinding(name, params, defaultsName, callArgsName, fnOnError)
	ctx.errorPos = savedPos
	if defaultsDecl != nil {
		argsDefine = append([]jen.Code{defaultsDecl}, argsDefine...)
	}

	bodyCode, err := ctx.transpileStatements(body, fnOnError, "")
	if err != nil {
		return nil, err
	}

	bodyCode = append(argsDefine, bodyCode...)

	closure := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Id("error")),
	).Block(bodyCode...)

	return jen.Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(name),
		jen.Id("Fn").Op(":").Add(closure),
	), nil
}

// buildDirectFunction generates a direct-lowered module-level function: a
// package-level Go function holding the transpiled body, with one Go
// parameter per Goblin parameter, and the returned assignment of the
// &object.Function wrapper adapting CallArgs-shaped calls onto it. The
// wrapper reproduces the fast/slow binding split buildFunctionValue
// generates, so every call shape that is not lowered — keyword arguments,
// wrong arity, the function used as a value — behaves exactly as before,
// diagnostics included.
func (ctx *transpileContext) buildDirectFunction(info directFn, fn *ast.FunctionDefine) ([]jen.Code, error) {
	name, pos, params, body := fn.Name, fn.Position(), fn.Parameters, fn.Body

	// A closed function (see signatures.go) carries an inferred signature:
	// its parameters and result take the native Go types the signature
	// says, and it needs no generic wrapper since nothing can reach it
	// other than the lowered call sites. Closedness excludes parameter
	// defaults, so there are none to evaluate for it.
	sig := ctx.sigs[name]
	var seed map[string]staticType
	retType := tyDynamic
	var defaultsDecl jen.Code
	defaultsName := ""
	if sig != nil {
		seed = make(map[string]staticType, len(params))
		for i, param := range params {
			seed[param.Name] = sig.params[i]
		}
		retType = sig.ret
	} else {
		// Default expressions belong to the enclosing scope, so they are
		// transpiled before the parameters are declared.
		var err error
		defaultsDecl, defaultsName, err = ctx.emitParamDefaults(params)
		if err != nil {
			return nil, err
		}
	}

	popScope := ctx.pushUserScope()
	for _, param := range params {
		ctx.declareUserName(param.Name)
	}
	restoreTypes := ctx.enterScope(body, seed, retType)
	defer ctx.withSelfType(nil)()

	module := sourceModuleName(pos)
	zero := zeroOf(retType)
	fnOnError := func(errVar string) jen.Code {
		return tracedReturn(zero, errVar, module, name, ctx.framePos(pos))
	}

	bodyCode, err := ctx.transpileStatements(body, fnOnError, "")
	restoreTypes()
	popScope()
	if err != nil {
		return nil, err
	}

	goType := func(t staticType) *jen.Statement {
		if t.native() {
			return jen.Id(goTypeOf(t))
		}
		return jen.Qual(pathObject, "Object")
	}
	paramDecls := make([]jen.Code, 0, len(params))
	for i, param := range params {
		paramType := tyDynamic
		if sig != nil {
			paramType = sig.params[i]
		}
		paramDecls = append(paramDecls, jen.Id(param.Name).Add(goType(paramType)))
	}
	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Id(info.goName).Params(paramDecls...).Parens(jen.List(
			goType(retType), jen.Id("error"),
		)).Block(bodyCode...))
	if sig != nil {
		return nil, nil
	}

	callArgsName := ctx.localName("callArgs")
	boundName := ctx.localName("bound")
	errVar := ctx.localName("err")

	fastArgs := make([]jen.Code, 0, len(params))
	slowArgs := make([]jen.Code, 0, len(params))
	paramNames := make([]jen.Code, 0, len(params))
	for i, param := range params {
		fastArgs = append(fastArgs, jen.Id(callArgsName).Dot("Positional").Index(jen.Lit(i)))
		slowArgs = append(slowArgs, jen.Id(boundName).Index(jen.Lit(i)))
		paramNames = append(paramNames, jen.Lit(param.Name))
	}

	// Binding errors point at the definition, like buildFunctionValue's.
	savedPos := ctx.errorPos
	ctx.errorPos = pos
	bindErrReturn := fnOnError(errVar)
	ctx.errorPos = savedPos

	defaultsArg := jen.Nil()
	if defaultsName != "" {
		defaultsArg = jen.Id(defaultsName)
	}

	wrapperBody := []jen.Code{
		jen.If(
			jen.Len(jen.Id(callArgsName).Dot("Keyword")).Op("==").Lit(0).
				Op("&&").
				Len(jen.Id(callArgsName).Dot("Positional")).Op("==").Lit(len(params)),
		).Block(
			jen.Return(jen.Id(info.goName).Call(fastArgs...)),
		),
	}
	if defaultsDecl != nil {
		wrapperBody = append(wrapperBody, defaultsDecl)
	}
	wrapperBody = append(wrapperBody,
		jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(name),
			jen.Index().String().Values(paramNames...),
			defaultsArg,
			jen.Lit(""),
			jen.Lit(""),
			jen.Id(callArgsName),
		),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(bindErrReturn),
		// A zero-parameter function still needs the call for its arity check,
		// but then never reads the result.
		jen.Id("_").Op("=").Id(boundName),
		jen.Return(jen.Id(info.goName).Call(slowArgs...)),
	)

	wrapper := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Id("error"),
	)).Block(wrapperBody...)

	return []jen.Code{
		jen.Id(name).Op("=").Op("&").Qual(pathObject, "Function").Values(
			jen.Id("Name").Op(":").Lit(name),
			jen.Id("Fn").Op(":").Add(wrapper),
		),
	}, nil
}

func (ctx *transpileContext) transpileFunctionDefine(fn *ast.FunctionDefine, onError errHandler) ([]jen.Code, error) {
	// Declared before the body transpiles so the function can shadow a
	// built-in even in recursive references to itself.
	ctx.declareUserName(fn.Name)
	funcValue, err := ctx.buildFunctionValue(fn.Name, fn.Position(), fn.Parameters, fn.Body)
	if err != nil {
		return nil, err
	}

	// Declare before assigning so the closure body can reference the
	// function's own name for recursion (a combined `var f = ...` would keep
	// f out of scope inside its own initializer under Go scoping rules).
	result := jen.Var().Id(fn.Name).Qual(pathObject, "Object")
	result.Op(";").Id(fn.Name).Op("=").Add(funcValue)
	result.Op(";").Id("_").Op("=").Id(fn.Name)

	return []jen.Code{result}, nil
}

func (ctx *transpileContext) transpileFunctionLiteral(fn *ast.FunctionLiteral) ([]jen.Code, *jen.Statement, error) {
	funcValue, err := ctx.buildFunctionValue("<lambda>", fn.Position(), fn.Parameters, fn.Body)
	if err != nil {
		return nil, nil, err
	}
	return nil, funcValue, nil
}

func (ctx *transpileContext) transpileTypeDefine(typeDef *ast.TypeDefine, onError errHandler) ([]jen.Code, error) {
	ctorVarName := typeDef.Name + "Constructor"
	ctx.moduleImports[typeDef.Name] = ctorVarName
	goTypeName := ctx.goTypeName(typeDef.Name)
	receiverName := receiverGoName

	structFields := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		structFields = append(structFields, jen.Id(fieldGoName(field.Name)).Qual(pathObject, "Object"))
	}

	ctx.topDecls = append(ctx.topDecls, jen.Type().Id(goTypeName).Struct(structFields...))

	// typeVar holds the type's object.UserType: its impls, built when the type
	// statement runs, since the traits they name are runtime values.
	typeVar := ctx.localName("type_" + typeDef.Name)
	ctx.topDecls = append(ctx.topDecls, jen.Var().Id(typeVar).Op("*").Qual(pathObject, "UserType"))

	receiverParam := func() jen.Code { return jen.Id(receiverName).Op("*").Id(goTypeName) }
	obj := func() *jen.Statement { return jen.Qual(pathObject, "Object") }
	// userCall builds `return object.<helper>(receiver, args...)`: every
	// operator and conversion dispatches through the type's impls in the
	// object package, exactly as the interpreter's instances do.
	userCall := func(helper string, args ...jen.Code) jen.Code {
		return jen.Return(jen.Qual(pathObject, helper).Call(append([]jen.Code{jen.Id(receiverName)}, args...)...))
	}
	protoDecls := make([]jen.Code, 0, 24)
	method := func(name string, params []jen.Code, results jen.Code, body ...jen.Code) {
		protoDecls = append(protoDecls, jen.Func().Params(receiverParam()).Id(name).Params(params...).Add(results).Block(body...))
	}
	other := func(name string) []jen.Code { return []jen.Code{jen.Id(name).Add(obj())} }

	// TypeName() string — the declared Goblin name, so diagnostics never leak
	// the generated Go type (the interpreter reports the same name here).
	method("TypeName", nil, jen.String(), jen.Return(jen.Lit(typeDef.Name)))
	method("UserType", nil, jen.Op("*").Qual(pathObject, "UserType"), jen.Return(jen.Id(typeVar)))
	fieldValues := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldValues = append(fieldValues, jen.Id(receiverName).Dot(fieldGoName(field.Name)))
	}
	method("FieldValues", nil, jen.Index().Add(obj()), jen.Return(jen.Index().Add(obj()).Values(fieldValues...)))
	method("String", nil, jen.String(), userCall("UserString"))
	method("ToString", nil, jen.Parens(jen.List(jen.String(), jen.Error())), userCall("UserToString"))
	method("ToBool", nil, jen.Parens(jen.List(jen.Bool(), jen.Error())), userCall("UserToBool"))
	method("Hash", nil, jen.Parens(jen.List(jen.Uint64(), jen.Error())), userCall("UserHash"))
	method("Equals", other("other"), jen.Parens(jen.List(jen.Bool(), jen.Error())), userCall("UserEquals", jen.Id("other")))
	method("Compare", other("other"), jen.Parens(jen.List(jen.Int(), jen.Error())), userCall("UserCompare", jen.Id("other")))

	// A binary operator whose arithmetic trait this type implements calls the
	// forward method's wrapper directly, skipping the impl lookup and the
	// argument slice the generic dispatch allocates. A missing operator goes
	// through the shared helper, which raises its error.
	implWrappers := make(map[*ast.FunctionDefine]string)
	for _, block := range typeDef.Impls {
		for _, m := range block.Methods {
			implWrappers[m] = ctx.localName("impl_" + m.Name)
		}
	}
	// forward maps each implemented built-in trait to the wrapper of its first
	// method; for the arithmetic traits that is the forward operator.
	forward := map[*object.Trait]string{}
	for _, block := range typeDef.Impls {
		trait := ctx.builtinTrait(block.Trait)
		for _, m := range block.Methods {
			if trait != nil && m.Name == trait.Methods[0].Name {
				forward[trait] = implWrappers[m]
			}
		}
	}
	for _, op := range []struct {
		goMethod, index string
		trait           *object.Trait
	}{
		{"Add", "ArithAdd", object.AddTrait},
		{"Minus", "ArithSub", object.SubTrait},
		{"Multiply", "ArithMul", object.MulTrait},
		{"Divide", "ArithDiv", object.DivTrait},
		{"Modulo", "ArithMod", object.ModTrait},
	} {
		if wrapper, ok := forward[op.trait]; ok {
			method(op.goMethod, other("other"), jen.Parens(jen.List(obj(), jen.Error())),
				jen.Return(jen.Id(receiverName).Dot(wrapper).Call(jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
					jen.Id("Positional"): jen.Qual(pathObject, "Args").Values(jen.Id("other")),
				}))))
			continue
		}
		method(op.goMethod, other("other"), jen.Parens(jen.List(obj(), jen.Error())),
			userCall("UserArith", jen.Qual(pathObject, op.index), jen.Id("other")))
	}
	for _, op := range []struct{ goMethod, index string }{
		{"RAdd", "ArithAdd"},
		{"RMinus", "ArithSub"},
		{"RMultiply", "ArithMul"},
		{"RDivide", "ArithDiv"},
		{"RModulo", "ArithMod"},
	} {
		method(op.goMethod, other("left"), jen.Parens(jen.List(obj(), jen.Bool(), jen.Error())),
			userCall("UserReflected", jen.Qual(pathObject, op.index), jen.Id("left")))
	}
	method("Iter", nil, jen.Parens(jen.List(jen.Index().Add(obj()), jen.Error())), userCall("UserIter"))
	method("Index", other("index"), jen.Parens(jen.List(obj(), jen.Error())), userCall("UserIndex", jen.Id("index")))
	method("SetIndex", []jen.Code{jen.Id("index").Add(obj()), jen.Id("value").Add(obj())}, jen.Parens(jen.List(jen.Bool(), jen.Error())),
		userCall("UserSetIndex", jen.Id("index"), jen.Id("value")))

	ctx.topDecls = append(ctx.topDecls, protoDecls...)

	getAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+len(typeDef.Methods)+2)
	attributeNames := make([]jen.Code, 0, len(typeDef.Fields)+len(typeDef.Methods)+2)
	seenAttributes := make(map[string]bool, cap(attributeNames))
	for _, field := range typeDef.Fields {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Return(jen.Id(receiverName).Dot(fieldGoName(field.Name)), jen.Nil()),
			),
		)
		if !seenAttributes[field.Name] {
			attributeNames = append(attributeNames, jen.Lit(field.Name))
			seenAttributes[field.Name] = true
		}
	}
	for _, method := range typeDef.Methods {
		wrapperName := methodWrapperName(method.Name)
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit(method.Name)).Block(
				jen.Return(
					jen.Op("&").Qual(pathObject, "Function").Values(
						jen.Id("Name").Op(":").Lit(method.Name),
						jen.Id("Fn").Op(":").Id(receiverName).Dot(wrapperName),
					),
					jen.Nil(),
				),
			),
		)
		if !seenAttributes[method.Name] {
			attributeNames = append(attributeNames, jen.Lit(method.Name))
			seenAttributes[method.Name] = true
		}
	}
	userDefinedConstructor := false
	for _, field := range typeDef.Fields {
		if field.Name == "constructor" {
			userDefinedConstructor = true
			break
		}
	}
	if !userDefinedConstructor {
		for _, method := range typeDef.Methods {
			if method.Name == "constructor" {
				userDefinedConstructor = true
				break
			}
		}
	}
	if !userDefinedConstructor {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit("constructor")).Block(
				jen.Return(jen.Id(ctorVarName), jen.Nil()),
			),
		)
		attributeNames = append(attributeNames, jen.Lit("constructor"))
		seenAttributes["constructor"] = true
	}
	if !seenAttributes["attributes"] {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit("attributes")).Block(
				jen.Return(jen.Qual(pathObject, "AttributesFunction").Call(jen.Id(receiverName)), jen.Nil()),
			),
		)
		attributeNames = append(attributeNames, jen.Lit("attributes"))
		seenAttributes["attributes"] = true
	}
	if !seenAttributes["traits"] {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit("traits")).Block(
				jen.Return(jen.Qual(pathObject, "TraitsFunction").Call(jen.Id(receiverName)), jen.Nil()),
			),
		)
		attributeNames = append(attributeNames, jen.Lit("traits"))
	}
	getAttrCases = append(getAttrCases,
		jen.Default().Block(
			jen.Return(
				jen.Nil(),
				jen.Qual(pathObject, "NewAttributeError").Call(jen.Lit("%s has no attribute '%s'"), jen.Lit(typeDef.Name), jen.Id("name")),
			),
		),
	)

	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("GetAttr").Params(
			jen.Id("name").String(),
		).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(
			jen.Switch(jen.Id("name")).Block(getAttrCases...),
		),
	)
	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("Attributes").Params().Index().String().Block(
			jen.Return(jen.Index().String().Values(attributeNames...)),
		),
	)

	// CallMethod satisfies object.MethodCaller, so `p.step()` calls the
	// method body directly instead of allocating the bound function GetAttr
	// hands out. Only real methods are answered here: a field shadowing a
	// method name, "constructor" and "attributes" all fall through to
	// GetAttr, which keeps their behavior exactly as it was.
	fieldNamed := make(map[string]bool, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNamed[field.Name] = true
	}
	callMethodCases := make([]jen.Code, 0, len(typeDef.Methods))
	for _, method := range typeDef.Methods {
		if fieldNamed[method.Name] {
			continue
		}
		callMethodCases = append(callMethodCases,
			jen.Case(jen.Lit(method.Name)).Block(
				jen.List(jen.Id("_value"), jen.Id("_err")).Op(":=").Id(receiverName).Dot(methodWrapperName(method.Name)).Call(jen.Id("_args")),
				jen.Return(jen.Id("_value"), jen.True(), jen.Id("_err")),
			),
		)
	}
	if len(callMethodCases) > 0 {
		ctx.topDecls = append(ctx.topDecls,
			jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("CallMethod").Params(
				jen.Id("name").String(), jen.Id("_args").Qual(pathObject, "CallArgs"),
			).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Bool(), jen.Error())).Block(
				jen.Switch(jen.Id("name")).Block(callMethodCases...),
				jen.Return(jen.Nil(), jen.False(), jen.Nil()),
			),
		)
	}

	setAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+1)
	for _, field := range typeDef.Fields {
		setAttrCases = append(setAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Id(receiverName).Dot(fieldGoName(field.Name)).Op("=").Id("value"),
				jen.Return(jen.True(), jen.Nil()),
			),
		)
	}
	setAttrCases = append(setAttrCases,
		jen.Default().Block(
			jen.Return(
				jen.True(),
				jen.Qual(pathObject, "NewAttributeError").Call(jen.Lit("%s has no attribute '%s'"), jen.Lit(typeDef.Name), jen.Id("name")),
			),
		),
	)
	ctx.topDecls = append(ctx.topDecls,
		jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id("SetAttr").Params(
			jen.Id("name").String(), jen.Id("value").Qual(pathObject, "Object"),
		).Parens(jen.List(jen.Bool(), jen.Error())).Block(
			jen.Switch(jen.Id("name")).Block(setAttrCases...),
		),
	)

	fieldSet := make(map[string]bool, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldSet[field.Name] = true
	}
	methodWrappers := make(map[string]string, len(typeDef.Methods))
	for _, method := range typeDef.Methods {
		methodWrappers[method.Name] = methodWrapperName(method.Name)
	}

	// Ordinary methods and impl methods compile alike, to a Go method taking
	// the arguments after self. Impl methods get their own wrapper names, so
	// they never collide with an ordinary method or with each other.
	type methodDef struct {
		def     *ast.FunctionDefine
		wrapper string
	}
	methodDefs := make([]methodDef, 0, len(typeDef.AllMethods()))
	for _, method := range typeDef.Methods {
		methodDefs = append(methodDefs, methodDef{method, methodWrapperName(method.Name)})
	}
	for _, block := range typeDef.Impls {
		for _, method := range block.Methods {
			methodDefs = append(methodDefs, methodDef{method, implWrappers[method]})
		}
	}

	for _, md := range methodDefs {
		method, wrapperName := md.def, md.wrapper

		// A method body is a scope of its own, with its own inferred
		// environment, like any other function body.
		popMethodScope := ctx.pushUserScope()
		for _, param := range method.Parameters {
			ctx.declareUserName(param.Name)
		}
		restoreMethodTypes := ctx.enterScope(method.Body, nil, tyDynamic)
		var self *selfInfo
		if !bodyRebinds(method.Body, "self") {
			self = &selfInfo{receiver: receiverName, fields: fieldSet, methods: methodWrappers}
		}
		restoreSelf := ctx.withSelfType(self)

		callArgsName := ctx.localName("callArgs")
		methodModule := sourceModuleName(method.Position())
		qualifiedName := typeDef.Name + "." + method.Name
		fnOnError := func(errVar string) jen.Code {
			return tracedReturn(jen.Nil(), errVar, methodModule, qualifiedName, ctx.framePos(method.Position()))
		}
		savedPos := ctx.errorPos
		ctx.errorPos = method.Position()

		var bodyPrefix []jen.Code
		defaultsDecl, defaultsName, err := ctx.emitParamDefaults(method.Parameters[1:])
		if err != nil {
			return nil, err
		}
		if defaultsDecl != nil {
			bodyPrefix = append(bodyPrefix, defaultsDecl)
		}
		bodyPrefix = append(bodyPrefix,
			ctx.emitParameterBinding(qualifiedName, method.Parameters[1:], defaultsName, callArgsName, fnOnError)...,
		)
		bodyPrefix = append(bodyPrefix,
			jen.Var().Id("self").Qual(pathObject, "Object").Op("=").Id(receiverName),
			jen.Id("_").Op("=").Id("self"),
		)

		methodBody, err := ctx.transpileStatements(method.Body, fnOnError, "")
		ctx.errorPos = savedPos
		restoreSelf()
		restoreMethodTypes()
		popMethodScope()
		if err != nil {
			return nil, err
		}

		ctx.topDecls = append(ctx.topDecls,
			jen.Func().Params(jen.Id(receiverName).Op("*").Id(goTypeName)).Id(wrapperName).Params(
				jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
			).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(
				append(bodyPrefix, methodBody...)...,
			),
		)
	}

	// The impls are registered when the type statement runs: a trait is a
	// runtime value, and an impl may name one from an imported module.
	fieldNameLits := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNameLits = append(fieldNameLits, jen.Lit(field.Name))
	}
	registration := []jen.Code{
		jen.Id(typeVar).Op("=").Qual(pathObject, "NewUserType").Call(jen.Lit(typeDef.Name), jen.Index().String().Values(fieldNameLits...)),
	}
	for _, block := range typeDef.Impls {
		pre, trait, err := ctx.transpileTraitRef(block.Trait, "impl "+block.Trait.String(), onError)
		if err != nil {
			return nil, err
		}
		registration = append(registration, pre...)
		methods := make([]jen.Code, 0, len(block.Methods))
		for _, method := range block.Methods {
			argsName := ctx.localName("implArgs")
			call := jen.Id(argsName).Dot("Positional").Index(jen.Lit(0)).Assert(jen.Op("*").Id(goTypeName)).
				Dot(implWrappers[method]).Call(jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
				jen.Id("Positional"): jen.Id(argsName).Dot("Positional").Index(jen.Lit(1), jen.Empty()),
			}))
			methods = append(methods, jen.Values(jen.Dict{
				jen.Id("Name"):  jen.Lit(method.Name),
				jen.Id("Arity"): jen.Lit(len(method.Parameters)),
				jen.Id("Fn"): jen.Op("&").Qual(pathObject, "Function").Values(jen.Dict{
					jen.Id("Name"): jen.Lit(method.Name),
					jen.Id("Fn"): jen.Func().Params(jen.Id(argsName).Qual(pathObject, "CallArgs")).
						Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error())).Block(jen.Return(call)),
				}),
			}))
		}
		registration = append(registration,
			jen.Id(typeVar).Dot("Implement").Call(trait, jen.Index().Qual(pathObject, "ImplMethod").Values(methods...)),
		)
	}
	sealErr := ctx.localName("err")
	registration = append(registration,
		jen.If(jen.Id(sealErr).Op(":=").Id(typeVar).Dot("Seal").Call(), jen.Id(sealErr).Op("!=").Nil()).Block(onError(sealErr)),
	)

	callArgsName := ctx.localName("callArgs")
	boundName := ctx.localName("bound")
	errVar := ctx.localName("err")

	fieldNames := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNames = append(fieldNames, jen.Lit(field.Name))
	}

	// Field defaults reach BindArguments as lazy closures, so a default is
	// evaluated only for the call that omits its field, exactly like a
	// function parameter default.
	defaultsArg := jen.Nil()
	hasDefault := false
	for _, field := range typeDef.Fields {
		if field.HasDefault() {
			hasDefault = true
			break
		}
	}
	if hasDefault {
		entries := make([]jen.Code, 0, len(typeDef.Fields))
		for _, field := range typeDef.Fields {
			if !field.HasDefault() {
				entries = append(entries, jen.Nil())
				continue
			}
			defaultPre, defaultValue, err := ctx.transpileExpression(field.DefaultValue, func(errVar string) jen.Code {
				return jen.Return(jen.Nil(), jen.Id(errVar))
			})
			if err != nil {
				return nil, err
			}
			entries = append(entries, jen.Func().Params().Parens(jen.List(
				jen.Qual(pathObject, "Object"), jen.Id("error"),
			)).Block(append(defaultPre, jen.Return(defaultValue, jen.Nil()))...))
		}
		defaultsArg = jen.Index().Qual(pathObject, "ParamDefault").Values(entries...)
	}

	fastValues := make([]jen.Code, 0, len(typeDef.Fields))
	slowValues := make([]jen.Code, 0, len(typeDef.Fields))
	for index, field := range typeDef.Fields {
		fastValues = append(fastValues,
			jen.Id(fieldGoName(field.Name)).Op(":").Id(callArgsName).Dot("Positional").Index(jen.Lit(index)))
		slowValues = append(slowValues,
			jen.Id(fieldGoName(field.Name)).Op(":").Id(boundName).Index(jen.Lit(index)))
	}

	// A construction that supplies every field positionally needs no binding
	// at all: the field order is known here. Every other shape goes through
	// BindArguments, so its diagnostics stay identical.
	constructorBody := []jen.Code{
		jen.If(
			jen.Len(jen.Id(callArgsName).Dot("Keyword")).Op("==").Lit(0).
				Op("&&").
				Len(jen.Id(callArgsName).Dot("Positional")).Op("==").Lit(len(typeDef.Fields)),
		).Block(
			jen.Return(jen.Op("&").Id(goTypeName).Values(fastValues...), jen.Nil()),
		),
		jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(typeDef.Name),
			jen.Index().String().Values(fieldNames...),
			defaultsArg,
			jen.Lit(""),
			jen.Lit(""),
			jen.Id(callArgsName),
		),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(
			jen.Return(jen.Nil(), jen.Id(errVar)),
		),
		jen.Id("_").Op("=").Id(boundName),
		jen.Return(jen.Op("&").Id(goTypeName).Values(slowValues...), jen.Nil()),
	}

	constructorClosure := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(
		jen.List(jen.Qual(pathObject, "Object"), jen.Error()),
	).Block(constructorBody...)

	ctx.topDecls = append(ctx.topDecls,
		jen.Var().Id(ctorVarName).Qual(pathObject, "Object"),
	)

	// The direct constructor lowered call sites use (see collectDirectFns):
	// one Go parameter per field, returning the struct literal.
	if info, ok := ctx.directCtors[typeDef.Name]; ok {
		params := make([]jen.Code, 0, len(typeDef.Fields))
		values := make([]jen.Code, 0, len(typeDef.Fields))
		for _, field := range typeDef.Fields {
			params = append(params, jen.Id(field.Name).Qual(pathObject, "Object"))
			values = append(values, jen.Id(fieldGoName(field.Name)).Op(":").Id(field.Name))
		}
		ctx.topDecls = append(ctx.topDecls,
			jen.Func().Id(info.goName).Params(params...).Parens(jen.List(
				jen.Qual(pathObject, "Object"), jen.Error(),
			)).Block(jen.Return(jen.Op("&").Id(goTypeName).Values(values...), jen.Nil())),
		)
	}

	constructor := jen.Id(ctorVarName).Op("=").Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(typeDef.Name),
		jen.Id("Fn").Op(":").Add(constructorClosure),
	)

	return append(registration, constructor), nil
}

// receiverGoName is the receiver of every generated method. It has the shape
// of a transpiler scratch name, which the checker reserves, so no parameter
// can shadow it.
const receiverGoName = "_recv_0"

// fieldGoName is the Go struct field holding a Goblin field. The prefix keeps
// fields apart from the generated methods: those start uppercase, and impl
// and scratch names start with an underscore.
func fieldGoName(name string) string {
	return "f_" + name
}

// builtinTrait reports the built-in trait a reference statically names: a bare
// built-in trait name that no module-level declaration or import shadows at
// this point. It returns nil for anything else.
func (ctx *transpileContext) builtinTrait(ref *ast.TraitRef) *object.Trait {
	if ref.Module != "" || ctx.isUserName(ref.Name) {
		return nil
	}
	if _, bound := ctx.moduleBinding(ref.Name); bound {
		return nil
	}
	return object.BuiltinTraits[ref.Name]
}

// transpileTraitRef evaluates the trait an impl block or a dependency list
// names to a *object.Trait expression.
// site says where the reference appears, for the runtime messages.
func (ctx *transpileContext) transpileTraitRef(ref *ast.TraitRef, site string, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if trait := ctx.builtinTrait(ref); trait != nil {
		return nil, jen.Qual(pathObject, trait.Name+"Trait"), nil
	}
	name := ref.Name
	if ref.Module != "" {
		name = ref.Module
	}
	ident, err := ast.NewIdentifier(&token.Token{Lit: []byte(name), Pos: ref.Pos})
	if err != nil {
		return nil, nil, err
	}
	pre, value, err := ctx.transpileExpression(ident.(*ast.Identifier), onError)
	if err != nil {
		return nil, nil, err
	}
	traitVar := ctx.localName("trait")
	errVar := ctx.localName("err")
	var call *jen.Statement
	if ref.Module != "" {
		call = jen.Qual(pathObject, "ModuleTrait").Call(value, jen.Lit(ref.Name), jen.Lit(site), jen.Lit(ref.String()))
	} else {
		call = jen.Qual(pathObject, "AsTrait").Call(value, jen.Lit(site), jen.Lit(ref.String()))
	}
	pre = append(pre,
		jen.List(jen.Id(traitVar), jen.Id(errVar)).Op(":=").Add(call),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return pre, jen.Id(traitVar), nil
}

// transpileTraitDefine emits the assignment of a trait declaration's trait
// object. Default methods compile to function values taking self first, like
// the interpreter's closures.
func (ctx *transpileContext) transpileTraitDefine(def *ast.TraitDefine, onError errHandler) ([]jen.Code, error) {
	var pre []jen.Code
	deps := make([]jen.Code, 0, len(def.Deps))
	for _, ref := range def.Deps {
		depPre, dep, err := ctx.transpileTraitRef(ref, "trait "+def.Name, onError)
		if err != nil {
			return nil, err
		}
		pre = append(pre, depPre...)
		deps = append(deps, dep)
	}
	methods := make([]jen.Code, 0, len(def.Methods))
	for _, method := range def.Methods {
		fields := jen.Dict{
			jen.Id("Name"):  jen.Lit(method.Name),
			jen.Id("Arity"): jen.Lit(len(method.Parameters)),
		}
		if method.Body == nil {
			fields[jen.Id("Required")] = jen.True()
		} else {
			fn, err := ctx.buildFunctionValue(def.Name+"."+method.Name, method.Position(), method.Parameters, method.Body)
			if err != nil {
				return nil, err
			}
			fields[jen.Id("Default")] = fn
		}
		methods = append(methods, jen.Values(fields))
	}
	return append(pre, jen.Id(def.Name).Op("=").Qual(pathObject, "NewTrait").Call(
		jen.Lit(def.Name),
		jen.Index().Op("*").Qual(pathObject, "Trait").Values(deps...),
		jen.Index().Qual(pathObject, "TraitMethod").Values(methods...),
	)), nil
}

func (ctx *transpileContext) transpileReturn(return_ *ast.Return, onError errHandler) ([]jen.Code, error) {
	if ctx.retType.native() {
		// The signature inference only makes a return type native when every
		// reachable return agrees with it; the parser's implicit `return nil`
		// is then unreachable and only has to keep Go's terminating-statement
		// rule satisfied.
		if ctx.nativeTypeOf(return_.Value) == ctx.retType {
			pre, value, err := ctx.emitNative(return_.Value, onError)
			if err != nil {
				return nil, err
			}
			return append(pre, jen.Return(jen.List(value, jen.Nil()))), nil
		}
		if isImplicitReturn(return_) {
			return []jen.Code{jen.Return(jen.List(zeroOf(ctx.retType), jen.Nil()))}, nil
		}
		return nil, fmt.Errorf("%s: internal error: return does not match the inferred native return type", return_.Position())
	}

	preStmts, r, err := ctx.transpileExpression(return_.Value, onError)
	if err != nil {
		return nil, err
	}
	return append(preStmts, jen.Return(jen.List(r, jen.Nil()))), nil
}

// transpileRaise turns `raise expr` into an error (the raised Error value, or a
// type error if expr is not an Error) that is then routed through the active
// error handler (a top-level `return`, or a jump to the enclosing `try`'s catch
// label).
func (ctx *transpileContext) transpileRaise(raise *ast.Raise, onError errHandler) ([]jen.Code, error) {
	preStmts, val, err := ctx.transpileExpression(raise.Value, onError)
	if err != nil {
		return nil, err
	}
	errVar := ctx.localName("err")
	code := append(preStmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "Raise").Call(val),
		onError(errVar),
	)
	return code, nil
}

// transpileTryCatch lowers `try { ... } catch e { ... }` using goto/labels so
// that `return`/`break`/`continue` inside the try body still target the real
// enclosing function/loop (a closure or defer/recover boundary would capture
// them). Inside the try body the error handler stores the error and jumps to
// the catch label; the unconditional post-block check guarantees the label is
// referenced even when the body has no fallible operation.
func (ctx *transpileContext) transpileTryCatch(tc *ast.TryCatch, onError errHandler) ([]jen.Code, error) {
	excVar := ctx.localName("exc")
	catchLabel := ctx.localName("catch")
	doneLabel := ctx.localName("done")

	tryOnError := func(errVar string) jen.Code {
		return jen.Block(
			jen.Id(excVar).Op("=").Id(errVar),
			jen.Goto().Id(catchLabel),
		)
	}

	tryBody, err := ctx.transpileStatements(tc.TryBody, tryOnError, "")
	if err != nil {
		return nil, err
	}
	// The catch variable lives in its own scope covering the catch body, and
	// shadows builtins there like any user name (the interpreter defines it in
	// a child environment).
	popCatchScope := ctx.pushUserScope()
	ctx.declareUserName(tc.CatchVar)
	catchBody, err := ctx.transpileStatements(tc.CatchBody, onError, "")
	popCatchScope()
	if err != nil {
		return nil, err
	}

	catchBlock := []jen.Code{
		jen.Id(tc.CatchVar).Op(":=").Qual(pathObject, "ErrorValue").Call(jen.Id(excVar)),
		jen.Id("_").Op("=").Id(tc.CatchVar),
	}
	catchBlock = append(catchBlock, catchBody...)

	return []jen.Code{
		jen.Var().Id(excVar).Error(),
		jen.Block(tryBody...),
		jen.If(jen.Id(excVar).Op("!=").Nil()).Block(jen.Goto().Id(catchLabel)),
		jen.Goto().Id(doneLabel),
		jen.Id(catchLabel).Op(":"),
		jen.Block(catchBlock...),
		jen.Id(doneLabel).Op(":"),
		jen.Id("_").Op("=").Id(excVar),
	}, nil
}

func isComparisonOperator(op string) bool {
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		return true
	}
	return false
}

func (ctx *transpileContext) transpileComparisonOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	preStmts, result, err := ctx.transpileComparisonBool(operation, onError)
	if err != nil {
		return nil, nil, err
	}
	tmpVar := ctx.localName("tmp")
	preStmts = append(preStmts,
		jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Bool").Call(result),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileComparisonBool evaluates a comparison of boxed operands down to a
// Go bool expression, which the operator boxes and a condition uses as is.
func (ctx *transpileContext) transpileComparisonBool(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, nativeRHS, err := ctx.transpileNativeIntOperand(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}
	preStmts := append(lhsPre, rhsPre...)

	// Equality is total for the built-in types, but a user type's eq may fail;
	// like ordering, that error propagates.
	entry := map[string]string{
		ast.Equal: "Equals", ast.NotEqual: "NotEquals",
		ast.LessThan: "Less", ast.LessOrEqual: "LessEqual",
		ast.GreaterThan: "Greater", ast.GreaterOrEqual: "GreaterEqual",
	}[operation.Operator]
	resultVar := ctx.localName("cmp")
	errVar := ctx.localName("err")
	preStmts = append(preStmts,
		jen.List(jen.Id(resultVar), jen.Id(errVar)).Op(":=").Qual(pathObject, nativeRHS(entry)).Call(lhs, rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(resultVar), nil
}

// transpileNativeIntOperand renders the right operand of a binary operator
// whose left operand is boxed. A statically integer right operand stays a
// native int64 and the returned function maps an operator entry point to its
// native-right variant (Add to AddInt, and so on); otherwise the operand is
// boxed and the mapping is the identity.
func (ctx *transpileContext) transpileNativeIntOperand(operand ast.Expression, onError errHandler) (pre []jen.Code, code *jen.Statement, variant func(string) string, err error) {
	if ctx.nativeTypeOf(operand) == tyInt {
		pre, code, err = ctx.emitNative(operand, onError)
		return pre, code, func(op string) string { return op + "Int" }, err
	}
	pre, code, err = ctx.transpileExpression(operand, onError)
	return pre, code, func(op string) string { return op }, err
}

// transpileCondition evaluates an if or while condition to a Go bool: a
// native bool expression as it is, a comparison of boxed operands through the
// comparison's own bool result, anything else through ToBool.
func (ctx *transpileContext) transpileCondition(cond ast.Expression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if ctx.nativeTypeOf(cond) == tyBool {
		return ctx.emitNative(cond, onError)
	}
	if op, ok := cond.(*ast.BinaryOperation); ok && isComparisonOperator(op.Operator) {
		return ctx.transpileComparisonBool(op, onError)
	}
	pre, value, err := ctx.transpileExpression(cond, onError)
	if err != nil {
		return nil, nil, err
	}
	condVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	pre = append(pre,
		jen.List(jen.Id(condVar), jen.Id(errVar)).Op(":=").Add(value).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return pre, jen.Id(condVar), nil
}

func (ctx *transpileContext) transpileBinaryOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	if isComparisonOperator(operation.Operator) {
		return ctx.transpileComparisonOperation(operation, onError)
	}
	if operation.Operator == ast.And || operation.Operator == ast.Or {
		return ctx.transpileLogicalOperation(operation, onError)
	}

	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, nativeRHS, err := ctx.transpileNativeIntOperand(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	var methodName string
	switch operation.Operator {
	case "+":
		methodName = "Add"
	case "-":
		methodName = "Minus"
	case "*":
		methodName = "Multiply"
	case "/":
		methodName = "Divide"
	case "%":
		methodName = "Modulo"
	default:
		return nil, nil, fmt.Errorf("unsupported binary operator: %s", operation.Operator)
	}

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(lhsPre, rhsPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Qual(pathObject, nativeRHS(methodName)).Call(lhs, rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileLogicalOperation lowers && and || with short-circuit evaluation: the
// RHS is only evaluated (and may therefore only error or produce side effects)
// when the LHS does not already fix the result. The value of the expression is
// the operand that decided it rather than a coerced Bool, so `x || fallback`
// yields x when x is truthy.
func (ctx *transpileContext) transpileLogicalOperation(operation *ast.BinaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	// Always transpile the RHS so diagnostics surface at compile time, but
	// emit its statements inside a guarded block so it only runs at runtime
	// when the LHS leaves the result open.
	rhsPre, rhs, err := ctx.transpileExpression(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	lhsTruthyVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	tmpVar := ctx.localName("tmp")

	rhsBlock := append([]jen.Code{}, rhsPre...)
	rhsBlock = append(rhsBlock, jen.Id(tmpVar).Op("=").Add(rhs))

	guard := jen.Id(lhsTruthyVar)
	if operation.Operator == ast.Or {
		guard = jen.Op("!").Id(lhsTruthyVar)
	}

	preStmts := append([]jen.Code{}, lhsPre...)
	preStmts = append(preStmts,
		jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Add(lhs),
		jen.List(jen.Id(lhsTruthyVar), jen.Id(errVar)).Op(":=").Id(tmpVar).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.If(guard).Block(rhsBlock...),
	)

	return preStmts, jen.Id(tmpVar), nil
}

func (ctx *transpileContext) transpileUnaryOperation(operation *ast.UnaryOperation, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	operandPre, operand, err := ctx.transpileExpression(operation.Operand, onError)
	if err != nil {
		return nil, nil, err
	}

	var call *jen.Statement
	switch operation.Operator {
	case "!":
		call = jen.Qual(pathObject, "Not").Call(operand)
	case "+":
		call = jen.Qual(pathObject, "Positive").Call(operand)
	case "-":
		call = jen.Qual(pathObject, "Negate").Call(operand)
	default:
		return nil, nil, fmt.Errorf("unsupported unary operator: %s", operation.Operator)
	}

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(operandPre,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(call),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
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

// lineDirective returns a jen.Code that renders as a Go `//line` directive
// pointing back to the original .goblin source location. Returns nil if the
// position lacks file context (zero-value Pos, synthetic nodes), in which
// case jen will skip it during rendering.
func lineDirective(pos token.Pos) jen.Code {
	src, ok := pos.Context.(token.Sourcer)
	if !ok || src == nil {
		return nil
	}
	path := src.Source()
	if path == "" || pos.Line <= 0 {
		return nil
	}
	// `go build` runs in a temp dir; absolutize so diagnostics resolve.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	col := pos.Column
	if col <= 0 {
		col = 1
	}
	return jen.Comment(fmt.Sprintf("//line %s:%d:%d", path, pos.Line, col))
}

func (ctx *transpileContext) transpileStatement(stmt ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	// Import produces no output; attaching a //line directive would orphan
	// it onto the following statement and skew its mapped line number.
	if _, isImport := stmt.(*ast.Import); isImport {
		return nil, nil
	}

	var prelude []jen.Code
	if d := lineDirective(stmt.Position()); d != nil {
		prelude = append(prelude, d)
	}

	var codes []jen.Code
	var err error
	switch v := stmt.(type) {
	case *ast.Declare:
		codes, err = ctx.transpileDeclare(v, onError)
	case *ast.Assign:
		codes, err = ctx.transpileAssign(v, onError)
	case *ast.SetIndex:
		codes, err = ctx.transpileSetIndex(v, onError)
	case *ast.SetAttr:
		codes, err = ctx.transpileSetAttr(v, onError)
	case *ast.FunctionCall:
		var argPreStmts []jen.Code
		var call *jen.Statement
		argPreStmts, call, err = ctx.transpileFunctionCall(v, onError)
		if err == nil {
			errVar := ctx.localName("err")
			codes = append(argPreStmts,
				jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			)
		}
	case *ast.CallExpression:
		var argPreStmts []jen.Code
		var call *jen.Statement
		argPreStmts, call, err = ctx.transpileCallExpression(v, onError)
		if err == nil {
			errVar := ctx.localName("err")
			codes = append(argPreStmts,
				jen.List(jen.Id("_"), jen.Id(errVar)).Op(":=").Add(call),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			)
		}
	case *ast.FunctionDefine:
		codes, err = ctx.transpileFunctionDefine(v, onError)
	case *ast.TypeDefine:
		codes, err = ctx.transpileTypeDefine(v, onError)
	case *ast.IfElse:
		codes, err = ctx.transpileIfElse(v, onError)
	case *ast.While:
		codes, err = ctx.transpileWhile(v, onError)
	case *ast.For:
		codes, err = ctx.transpileFor(v, onError)
	case *ast.Break:
		codes, err = ctx.transpileBreak(v)
	case *ast.Continue:
		codes, err = ctx.transpileContinue(v)
	case *ast.Return:
		codes, err = ctx.transpileReturn(v, onError)
	case *ast.Raise:
		codes, err = ctx.transpileRaise(v, onError)
	case *ast.TryCatch:
		codes, err = ctx.transpileTryCatch(v, onError)
	case *ast.Export:
		codes, err = ctx.transpileExport(v, exportsVar)
	case *ast.BinaryOperation:
		var pre []jen.Code
		pre, _, err = ctx.transpileBinaryOperation(v, onError)
		codes = pre
	case *ast.UnaryOperation:
		var pre []jen.Code
		pre, _, err = ctx.transpileUnaryOperation(v, onError)
		codes = pre
	case *ast.MemberExpression:
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = ctx.transpileMemberExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	case *ast.IndexExpression:
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = ctx.transpileIndexExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	case ast.Expression:
		// Any other bare expression statement: evaluate it for its side
		// effects and discard the value.
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = ctx.transpileExpression(v, onError)
		if err == nil {
			codes = append(pre, jen.Id("_").Op("=").Add(value))
		}
	default:
		return nil, fmt.Errorf("%s: cannot transpile statement of type %T", stmt.Position(), stmt)
	}
	if err != nil {
		return nil, err
	}
	return append(prelude, codes...), nil
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

func (ctx *transpileContext) transpileStatements(stmts []ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	// Each statement list renders as a Go block, so names declared inside it
	// must not shadow built-ins beyond the block's end.
	defer ctx.pushUserScope()()
	var result []jen.Code
	for _, stmt := range stmts {
		saved := ctx.errorPos
		ctx.errorPos = stmt.Position()
		codes, err := ctx.transpileStatement(stmt, onError, exportsVar)
		ctx.errorPos = saved
		if err != nil {
			return nil, err
		}
		result = append(result, codes...)
	}
	return result, nil
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
// The entry-point module becomes output/main.go; each imported path module becomes
// its own package under outputDir.
func TranspileToDir(mod *ast.Module, sourceFile, outputDir string) error {
	if err := semantic.CheckModule(mod); err != nil {
		return err
	}

	base := filepath.Base(sourceFile)
	moduleName := strings.TrimSuffix(base, ".goblin")

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
		if !ok || !isPathImport(imp.Path) {
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
			if !ok || !isPathImport(imp.Path) {
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
		if !ok || !isPathImport(imp.Path) {
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
