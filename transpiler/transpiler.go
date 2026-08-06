package transpiler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

const (
	pathBase                    = "github.com/aisk/goblin"
	pathObject                  = pathBase + "/object"
	pathExtension               = pathBase + "/extension"
	defaultGoblinRuntimeVersion = "v0.0.0-20260731160124-eddbfc600c08"
)

type moduleInfo struct {
	executorPath string
	varName      string
	executorFunc string
}

var knownModules = map[string]moduleInfo{
	"os":              {executorPath: pathExtension, varName: "os_module", executorFunc: "ExecuteOs"},
	"rand":            {executorPath: pathExtension, varName: "rand_module", executorFunc: "ExecuteRand"},
	"math":            {executorPath: pathExtension, varName: "math_module", executorFunc: "ExecuteMath"},
	"base64":          {executorPath: pathExtension, varName: "base64_module", executorFunc: "ExecuteBase64"},
	"http":            {executorPath: pathExtension + "/http", varName: "http_module", executorFunc: "Execute"},
	"fs":              {executorPath: pathExtension + "/fs", varName: "fs_module", executorFunc: "Execute"},
	"mime":            {executorPath: pathExtension, varName: "mime_module", executorFunc: "ExecuteMime"},
	"json":            {executorPath: pathExtension, varName: "json_module", executorFunc: "ExecuteJson"},
	"uuid":            {executorPath: pathExtension, varName: "uuid_module", executorFunc: "ExecuteUUID"},
	"path":            {executorPath: pathExtension + "/path", varName: "path_module", executorFunc: "Execute"},
	"time":            {executorPath: pathExtension + "/time", varName: "time_module", executorFunc: "Execute"},
	"exec":            {executorPath: pathExtension + "/exec", varName: "exec_module", executorFunc: "Execute"},
	"regexp":          {executorPath: pathExtension + "/regexp", varName: "regexp_module", executorFunc: "Execute"},
	"hex":             {executorPath: pathExtension, varName: "hex_module", executorFunc: "ExecuteHex"},
	"sha256":          {executorPath: pathExtension, varName: "sha256_module", executorFunc: "ExecuteSHA256"},
	"sha512":          {executorPath: pathExtension, varName: "sha512_module", executorFunc: "ExecuteSHA512"},
	"url":             {executorPath: pathExtension + "/url", varName: "url_module", executorFunc: "Execute"},
	"csv":             {executorPath: pathExtension, varName: "csv_module", executorFunc: "ExecuteCSV"},
	"gzip":            {executorPath: pathExtension, varName: "gzip_module", executorFunc: "ExecuteGzip"},
	"zlib":            {executorPath: pathExtension, varName: "zlib_module", executorFunc: "ExecuteZlib"},
	"tar":             {executorPath: pathExtension, varName: "tar_module", executorFunc: "ExecuteTar"},
	"zip":             {executorPath: pathExtension, varName: "zip_module", executorFunc: "ExecuteZip"},
	"base32":          {executorPath: pathExtension, varName: "base32_module", executorFunc: "ExecuteBase32"},
	"ascii85":         {executorPath: pathExtension, varName: "ascii85_module", executorFunc: "ExecuteASCII85"},
	"html":            {executorPath: pathExtension, varName: "html_module", executorFunc: "ExecuteHTML"},
	"quotedprintable": {executorPath: pathExtension, varName: "quotedprintable_module", executorFunc: "ExecuteQuotedPrintable"},
	"md5":             {executorPath: pathExtension, varName: "md5_module", executorFunc: "ExecuteMD5"},
	"sha1":            {executorPath: pathExtension, varName: "sha1_module", executorFunc: "ExecuteSHA1"},
	"crc32":           {executorPath: pathExtension, varName: "crc32_module", executorFunc: "ExecuteCRC32"},
	"adler32":         {executorPath: pathExtension, varName: "adler32_module", executorFunc: "ExecuteAdler32"},
	"flate":           {executorPath: pathExtension, varName: "flate_module", executorFunc: "ExecuteFlate"},
	"bzip2":           {executorPath: pathExtension, varName: "bzip2_module", executorFunc: "ExecuteBzip2"},
	"mail":            {executorPath: pathExtension + "/mail", varName: "mail_module", executorFunc: "Execute"},
	"hmac":            {executorPath: pathExtension, varName: "hmac_module", executorFunc: "ExecuteHMAC"},
	"crc64":           {executorPath: pathExtension, varName: "crc64_module", executorFunc: "ExecuteCRC64"},
	"fnv":             {executorPath: pathExtension, varName: "fnv_module", executorFunc: "ExecuteFNV"},
	"lzw":             {executorPath: pathExtension, varName: "lzw_module", executorFunc: "ExecuteLZW"},
	"pem":             {executorPath: pathExtension, varName: "pem_module", executorFunc: "ExecutePEM"},
	"netip":           {executorPath: pathExtension + "/netip", varName: "netip_module", executorFunc: "Execute"},
	"utf8":            {executorPath: pathExtension, varName: "utf8_module", executorFunc: "ExecuteUTF8"},
	"unicode":         {executorPath: pathExtension, varName: "unicode_module", executorFunc: "ExecuteUnicode"},
}

// transpileContext holds state for a single Transpile call.
type transpileContext struct {
	localNameCounter int
	moduleImports    map[string]string   // module name -> Go variable name
	importing        map[string]struct{} // paths currently being transpiled (cycle detection)
	imported         map[string]struct{} // paths already transpiled (dedup)
	moduleFuncs      []jen.Code          // top-level module executor functions (single-file mode)
	topDecls         []jen.Code          // top-level type declarations and methods
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
}

func newTranspileContext() *transpileContext {
	return &transpileContext{
		localNameCounter: 0,
		moduleImports:    make(map[string]string),
		importing:        make(map[string]struct{}),
		imported:         make(map[string]struct{}),
		moduleFuncs:      nil,
		topDecls:         nil,
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
// returns a function restoring the previous one.
func (ctx *transpileContext) enterScope(body []ast.Statement) func() {
	saved := ctx.localTypes
	ctx.localTypes = inferLocals(body)
	return func() { ctx.localTypes = saved }
}

// typeOf reports the specialised native type of a name, or tyDynamic when the
// name is an ordinary boxed value.
func (ctx *transpileContext) typeOf(name string) staticType {
	if ctx.localTypes == nil {
		return tyDynamic
	}
	if t, ok := ctx.localTypes[name]; ok {
		return t
	}
	return tyDynamic
}

// nativeTypeOf reports whether an expression can be evaluated as a native Go
// expression in the current scope, and with what type.
func (ctx *transpileContext) nativeTypeOf(expr ast.Expression) staticType {
	if ctx.localTypes == nil {
		return tyDynamic
	}
	return staticTypeOf(expr, ctx.localTypes)
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
	"Not": true, "Iter": true, "Index": true,
	"GetAttr": true, "Attributes": true, "SetAttr": true, "SetIndex": true,
	"TypeName": true,
	"RAdd":     true, "RMinus": true, "RMultiply": true, "RDivide": true, "RModulo": true,
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

func tracedReturn(errVar, module, function string, pos token.Pos) jen.Code {
	return jen.Return(jen.Nil(), jen.Qual(pathObject, "WithFrame").Call(
		jen.Id(errVar), frameCode(module, function, pos),
	))
}

func modulePosition(mod *ast.Module) token.Pos {
	if len(mod.Body) > 0 {
		return mod.Body[0].Position()
	}
	return token.Pos{}
}

func isPathImport(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.Contains(path, "/")
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

func Transpile(mod *ast.Module, output io.Writer) error {
	if err := semantic.CheckModule(mod); err != nil {
		return err
	}

	ctx := newTranspileContext()
	if cwd, err := os.Getwd(); err == nil {
		ctx.baseDir = cwd
		ctx.rootDir = cwd
	}

	// Collect imports
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			if isPathImport(imp.Path) {
				ctx.moduleImports[imp.Name] = imp.Name
			} else {
				info, exists := knownModules[imp.Path]
				if !exists {
					return fmt.Errorf("unknown module: %s", imp.Path)
				}
				ctx.moduleImports[imp.Name] = info.varName
			}
		}
	}

	// Process path imports: parse and transpile each .goblin module
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok || !isPathImport(imp.Path) {
			continue
		}
		if err := ctx.transpilePathModule(imp.Path); err != nil {
			return err
		}
	}

	f := jen.NewFile(mod.Name)

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

	exportsVar := ctx.localName("exports")

	onError := func(errVar string) jen.Code {
		return tracedReturn(errVar, mod.Name, "<module>", modulePosition(mod))
	}

	stmts, err := ctx.transpileModuleStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return err
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
	}

	for _, fn := range ctx.moduleFuncs {
		f.Add(fn)
	}

	body := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports via registry
	for name, info := range knownModules {
		if _, ok := ctx.moduleImports[name]; ok {
			errVar := ctx.localName("err")
			body = append(body,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(info.executorPath, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports via registry
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
		errVar := ctx.localName("err")
		body = append(body,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
				jen.Lit(key),
				jen.Id(keyToFuncName(key)),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
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
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		return fmt.Errorf("parse error in module %s: %v", importPath, err)
	}

	mod, ok := st.(*ast.Module)
	if !ok {
		return fmt.Errorf("internal error: unexpected AST type for module %s", importPath)
	}
	if err := semantic.CheckModule(mod); err != nil {
		return fmt.Errorf("semantic error in module %s: %v", importPath, err)
	}

	// Collect sub-module imports
	subModuleImports := make(map[string]string)
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			if isPathImport(imp.Path) {
				subModuleImports[imp.Name] = imp.Name
				if err := ctx.transpilePathModule(imp.Path); err != nil {
					return err
				}
			} else {
				info, exists := knownModules[imp.Path]
				if !exists {
					return fmt.Errorf("unknown module in %s: %s", importPath, imp.Path)
				}
				subModuleImports[imp.Name] = info.varName
			}
		}
	}

	// Save and restore module imports for this scope
	savedImports := ctx.moduleImports
	ctx.moduleImports = subModuleImports
	defer func() { ctx.moduleImports = savedImports }()

	exportsVar := ctx.localName("exports")

	onError := func(errVar string) jen.Code {
		return tracedReturn(errVar, mod.Name, "<module>", modulePosition(mod))
	}

	stmts, err := ctx.transpileModuleStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return fmt.Errorf("transpile error in module %s: %v", importPath, err)
	}

	funcBody := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports for this sub-module via registry
	for name, info := range knownModules {
		if _, ok := subModuleImports[name]; ok {
			errVar := ctx.localName("err")
			funcBody = append(funcBody,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(info.executorPath, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports for this sub-module
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
		errVar := ctx.localName("err")
		funcBody = append(funcBody,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
				jen.Lit(subKey),
				jen.Id(keyToFuncName(subKey)),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
	}

	funcBody = append(funcBody, stmts...)
	funcBody = append(funcBody,
		jen.Return(
			jen.Op("&").Qual(pathObject, "Module").Values(
				jen.Id("Members").Op(":").Id(exportsVar),
			),
			jen.Nil(),
		),
	)

	funcName := keyToFuncName(ctx.moduleKey(absPath))
	fn := jen.Func().Id(funcName).Params().Parens(jen.List(
		jen.Qual(pathObject, "Object"), jen.Error(),
	)).Block(funcBody...)

	ctx.moduleFuncs = append(ctx.moduleFuncs, fn)
	ctx.imported[absPath] = struct{}{}
	return nil
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
	idxPre, idx, err := ctx.transpileExpression(expr.Index, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := ctx.localName("tmp")
	errVar := ctx.localName("err")
	preStmts := append(objPre, idxPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Add(obj).Dot("Index").Call(idx),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
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
		if moduleVar, ok := ctx.moduleImports[v.Name]; ok {
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
	// Fast path: when every argument is a plain positional (no *, **, or keyword
	// argument), the result is a simple slice literal. This avoids emitting the
	// positional/keyword/callArgs temporaries and per-argument append statements
	// that the general path below needs, which is the common case for most calls.
	allPositional := true
	for _, arg := range args {
		if arg.Kind != ast.CallArgumentPositional {
			allPositional = false
			break
		}
	}
	if allPositional {
		var preStmts []jen.Code
		argExprs := make([]jen.Code, 0, len(args))
		for _, arg := range args {
			argPreStmts, argExpr, err := ctx.transpileExpression(arg.Expr, onError)
			if err != nil {
				return nil, nil, err
			}
			preStmts = append(preStmts, argPreStmts...)
			argExprs = append(argExprs, argExpr)
		}
		callArgs := jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
			jen.Id("Positional"): jen.Qual(pathObject, "Args").Values(argExprs...),
		})
		return preStmts, callArgs, nil
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
	idxPre, idx, err := ctx.transpileExpression(s.Index, onError)
	if err != nil {
		return nil, err
	}
	valPre, val, err := ctx.transpileExpression(s.Value, onError)
	if err != nil {
		return nil, err
	}

	errVar := ctx.localName("err")
	stmts := append(objPre, idxPre...)
	stmts = append(stmts, valPre...)
	stmts = append(stmts,
		jen.Id(errVar).Op(":=").Qual(pathObject, "SetIndex").Call(obj, idx, val),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return stmts, nil
}

func (ctx *transpileContext) transpileSetAttr(s *ast.SetAttr, onError errHandler) ([]jen.Code, error) {
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
	// A natively-typed bool condition is already a Go bool: no boxing, no
	// ToBool() call, no error path.
	if ctx.nativeTypeOf(ifelse.Condition) == tyBool {
		pre, cond, err := ctx.emitNative(ifelse.Condition, onError)
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

	condPreStmts, cond, err := ctx.transpileExpression(ifelse.Condition, onError)
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
	condVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	stmts := append([]jen.Code{}, condPreStmts...)
	stmts = append(stmts,
		jen.List(jen.Id(condVar), jen.Id(errVar)).Op(":=").Add(cond).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.If(jen.Id(condVar)).Block(body...).Else().Block(elseBody...),
	)
	return stmts, nil
}

func (ctx *transpileContext) transpileWhile(while_ *ast.While, onError errHandler) ([]jen.Code, error) {
	// Same as in transpileIfElse: a native bool condition becomes the loop
	// condition directly, which is what lets a hot numeric loop compile down to
	// an ordinary Go `for`.
	if ctx.nativeTypeOf(while_.Condition) == tyBool {
		condPre, cond, err := ctx.emitNative(while_.Condition, onError)
		if err != nil {
			return nil, err
		}
		body, err := ctx.transpileStatements(while_.Body, onError, "")
		if err != nil {
			return nil, err
		}
		// With no guard to hoist this is a plain Go for-condition; a division
		// in the condition needs its check re-run each iteration, so the loop
		// becomes `for { guard; if !cond { break }; body }`.
		if len(condPre) == 0 {
			return []jen.Code{jen.For(cond).Block(body...)}, nil
		}
		loopBody := append([]jen.Code{}, condPre...)
		loopBody = append(loopBody, jen.If(jen.Op("!").Add(cond)).Block(jen.Break()))
		loopBody = append(loopBody, body...)
		return []jen.Code{jen.For().Block(loopBody...)}, nil
	}

	condPreStmts, cond, err := ctx.transpileExpression(while_.Condition, onError)
	if err != nil {
		return nil, err
	}
	body, err := ctx.transpileStatements(while_.Body, onError, "")
	if err != nil {
		return nil, err
	}

	condVar := ctx.localName("cond")
	errVar := ctx.localName("err")
	loopBody := append([]jen.Code{}, condPreStmts...)
	loopBody = append(loopBody,
		jen.List(jen.Id(condVar), jen.Id(errVar)).Op(":=").Add(cond).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.If(jen.Op("!").Id(condVar)).Block(jen.Break()),
	)
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

func (ctx *transpileContext) transpileFunctionCall(call *ast.FunctionCall, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	argPreStmts, args, err := ctx.transpileCallArguments(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}

	var callee *jen.Statement
	if mapped, ok := ctx.moduleImports[call.Name]; ok {
		callee = jen.Id(mapped)
	} else if !ctx.isUserName(call.Name) && isBuiltinFunction(call.Name) {
		callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(call.Name))
	} else {
		callee = jen.Id(call.Name)
	}

	return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
}

func (ctx *transpileContext) transpileCallExpression(call *ast.CallExpression, onError errHandler) ([]jen.Code, *jen.Statement, error) {
	argPreStmts, args, err := ctx.transpileCallArguments(call.Args, onError)
	if err != nil {
		return nil, nil, err
	}

	if ident, ok := call.Callee.(*ast.Identifier); ok {
		var callee *jen.Statement
		if mapped, ok := ctx.moduleImports[ident.Name]; ok {
			callee = jen.Id(mapped)
		} else if !ctx.isUserName(ident.Name) && isBuiltinFunction(ident.Name) {
			callee = jen.Id("builtin").Dot("Members").Index(jen.Lit(ident.Name))
		} else {
			callee = jen.Id(ident.Name)
		}
		return argPreStmts, jen.Qual(pathObject, "Call").Call(callee, args), nil
	}

	if member, ok := call.Callee.(*ast.MemberExpression); ok {
		objPre, obj, err := ctx.transpileExpression(member.Object, onError)
		if err != nil {
			return nil, nil, err
		}
		attrVar := ctx.localName("attr")
		errVar := ctx.localName("err")
		preStmts := append(objPre, argPreStmts...)
		preStmts = append(preStmts,
			jen.List(jen.Id(attrVar), jen.Id(errVar)).Op(":=").Parens(jen.Add(obj)).Dot("GetAttr").Call(jen.Lit(member.Property)),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		)
		return preStmts, jen.Qual(pathObject, "Call").Call(jen.Id(attrVar), args), nil
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
// resulting map. name is used only for BindArguments diagnostics, defaultsName
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
	// parameters, all positional, needs no binding map at all: the parameter
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
			// check, but then never reads the map.
			jen.Id("_").Op("=").Id(boundName),
		}
		for _, param := range fixedParams {
			slowBody = append(slowBody,
				jen.Id(param.Name).Op("=").Id(boundName).Index(jen.Lit(param.Name)),
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

	emit := func(param *ast.Parameter) {
		stmts = append(stmts,
			jen.Var().Id(param.Name).Qual(pathObject, "Object").Op("=").Id(boundName).Index(jen.Lit(param.Name)),
			jen.Id("_").Op("=").Id(param.Name),
		)
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
	// since a caller can pass anything.
	defer ctx.enterScope(body)()
	defer ctx.pushUserScope()()
	for _, param := range params {
		ctx.declareUserName(param.Name)
	}

	callArgsName := ctx.localName("callArgs")

	fnOnError := func(errVar string) jen.Code {
		return tracedReturn(errVar, "", name, pos)
	}

	argsDefine := ctx.emitParameterBinding(name, params, defaultsName, callArgsName, fnOnError)
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
	receiverName := strings.ToLower(typeDef.Name[:1])
	if receiverName == "_" {
		receiverName = "self"
	}

	structFields := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		structFields = append(structFields, jen.Id(field.Name).Qual(pathObject, "Object"))
	}

	ctx.topDecls = append(ctx.topDecls, jen.Type().Id(goTypeName).Struct(structFields...))

	reprFormat := fmt.Sprintf("<%s@%%p>", typeDef.Name)

	// A user type may customize protocol behavior (operators, iteration,
	// string/bool conversion, indexing) by defining a method with the matching
	// conventional name. When defined, the generated interface method delegates
	// to that method's wrapper; otherwise it falls back to a type error.
	defined := make(map[string]bool, len(typeDef.Methods))
	for _, m := range typeDef.Methods {
		defined[m.Name] = true
	}
	receiverParam := func() jen.Code { return jen.Id(receiverName).Op("*").Id(goTypeName) }
	// Protocol fallbacks raise the same TypeError the interpreter and the
	// built-in types raise, so `catch TypeError` and object.Compare's reflected
	// dispatch behave identically on user types across both backends.
	errorf := func(format string) jen.Code {
		return jen.Qual(pathObject, "NewTypeError").Call(jen.Lit(format), jen.Lit(typeDef.Name))
	}
	// protoCall builds `receiver.Wrapper(object.CallArgs{Positional: {args}})`.
	protoCall := func(goblinName string, args ...jen.Code) *jen.Statement {
		var callArgs jen.Code
		if len(args) == 0 {
			callArgs = jen.Qual(pathObject, "CallArgs").Values()
		} else {
			callArgs = jen.Qual(pathObject, "CallArgs").Values(jen.Dict{
				jen.Id("Positional"): jen.Qual(pathObject, "Args").Values(args...),
			})
		}
		return jen.Id(receiverName).Dot(methodWrapperName(goblinName)).Call(callArgs)
	}
	protoDecls := make([]jen.Code, 0, 18)

	// TypeName() string — the declared Goblin name, so diagnostics never leak
	// the generated Go type (the interpreter reports the same name here).
	protoDecls = append(protoDecls, jen.Func().Params(receiverParam()).Id("TypeName").Params().String().Block(
		jen.Return(jen.Lit(typeDef.Name)),
	))

	// String() string  <- "__str" (infallible fmt.Stringer, for diagnostics)
	reprReturn := jen.Return(jen.Qual("fmt", "Sprintf").Call(jen.Lit(reprFormat), jen.Id(receiverName)))
	strDecl := jen.Func().Params(receiverParam()).Id("String").Params().String()
	if defined["__str"] {
		strDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__str")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(reprReturn),
			jen.Return(jen.Qual("fmt", "Sprint").Call(jen.Id("_res"))),
		)
	} else {
		strDecl.Block(reprReturn)
	}
	protoDecls = append(protoDecls, strDecl)

	// ToString() (string, error)  <- "__str" (error-propagating)
	toStringReturn := jen.Return(jen.Qual("fmt", "Sprintf").Call(jen.Lit(reprFormat), jen.Id(receiverName)), jen.Nil())
	toStringDecl := jen.Func().Params(receiverParam()).Id("ToString").Params().Parens(jen.List(jen.String(), jen.Error()))
	if defined["__str"] {
		toStringDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__str")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.Lit(""), jen.Id("_err"))),
			jen.Return(jen.Id("_res").Dot("ToString").Call()),
		)
	} else {
		toStringDecl.Block(toStringReturn)
	}
	protoDecls = append(protoDecls, toStringDecl)

	// ToBool() (bool, error)  <- "__bool" (error-propagating)
	toBoolDecl := jen.Func().Params(receiverParam()).Id("ToBool").Params().Parens(jen.List(jen.Bool(), jen.Error()))
	if defined["__bool"] {
		toBoolDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__bool")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.False(), jen.Id("_err"))),
			jen.Return(jen.Id("_res").Dot("ToBool").Call()),
		)
	} else {
		toBoolDecl.Block(jen.Return(jen.True(), jen.Nil()))
	}
	protoDecls = append(protoDecls, toBoolDecl)

	// Equals(other) (bool, error) — __cmp when defined, identity otherwise. A
	// __cmp that fails fails the comparison rather than reporting "not equal".
	equalsDecl := jen.Func().Params(receiverParam()).Id("Equals").Params(
		jen.Id("other").Qual(pathObject, "Object"),
	).Parens(jen.List(jen.Bool(), jen.Error()))
	if defined["__cmp"] {
		equalsDecl.Block(
			jen.List(jen.Id("_cmp"), jen.Id("_err")).Op(":=").Id(receiverName).Dot("Compare").Call(jen.Id("other")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.False(), jen.Id("_err"))),
			jen.Return(jen.Id("_cmp").Op("==").Lit(0), jen.Nil()),
		)
	} else {
		equalsDecl.Block(
			jen.List(jen.Id("_o"), jen.Id("_ok")).Op(":=").Id("other").Assert(jen.Op("*").Id(goTypeName)),
			jen.Return(jen.Id("_ok").Op("&&").Id("_o").Op("==").Id(receiverName), jen.Nil()),
		)
	}
	protoDecls = append(protoDecls, equalsDecl)

	// Compare(other) (int, error)  <- "__cmp" (returns Int -1/0/1)
	cmpDecl := jen.Func().Params(receiverParam()).Id("Compare").Params(
		jen.Id("other").Qual(pathObject, "Object"),
	).Parens(jen.List(jen.Int(), jen.Error()))
	if defined["__cmp"] {
		cmpDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__cmp", jen.Id("other"))),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.Lit(0), jen.Id("_err"))),
			jen.List(jen.Id("_i"), jen.Id("_ok")).Op(":=").Id("_res").Assert(jen.Qual(pathObject, "Integer")),
			jen.If(jen.Op("!").Id("_ok")).Block(
				jen.Return(jen.Lit(0), jen.Qual(pathObject, "NewTypeError").Call(
					jen.Lit("%s.__cmp must return Int, got %s"),
					jen.Lit(typeDef.Name),
					jen.Qual("fmt", "Sprint").Call(jen.Id("_res")),
				)),
			),
			jen.Return(jen.Int().Parens(jen.Id("_i")), jen.Nil()),
		)
	} else {
		cmpDecl.Block(jen.Return(jen.Lit(0), errorf("cannot compare %s")))
	}
	protoDecls = append(protoDecls, cmpDecl)

	// Binary operators (other) (Object, error)
	binOps := []struct{ goMethod, goblin, errFmt string }{
		{"Add", "__add", "cannot add %s"},
		{"Minus", "__sub", "cannot subtract %s"},
		{"Multiply", "__mul", "cannot multiply %s"},
		{"Divide", "__div", "cannot divide %s"},
		{"Modulo", "__mod", "cannot modulo %s"},
	}
	for _, op := range binOps {
		d := jen.Func().Params(receiverParam()).Id(op.goMethod).Params(
			jen.Id("other").Qual(pathObject, "Object"),
		).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error()))
		if defined[op.goblin] {
			d.Block(jen.Return(protoCall(op.goblin, jen.Id("other"))))
		} else {
			d.Block(jen.Return(jen.Nil(), errorf(op.errFmt)))
		}
		protoDecls = append(protoDecls, d)
	}

	// Reflected operators (Object, bool, error) <- "__radd" and friends. The
	// methods are always generated so the object.Right* interfaces are
	// satisfied; the bool tells the dispatcher whether the type actually
	// defines a handler for this operand order.
	for _, op := range []struct{ goMethod, goblin string }{
		{"RAdd", "__radd"},
		{"RMinus", "__rsub"},
		{"RMultiply", "__rmul"},
		{"RDivide", "__rdiv"},
		{"RModulo", "__rmod"},
	} {
		d := jen.Func().Params(receiverParam()).Id(op.goMethod).Params(
			jen.Id("left").Qual(pathObject, "Object"),
		).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Bool(), jen.Error()))
		if defined[op.goblin] {
			d.Block(
				jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall(op.goblin, jen.Id("left"))),
				jen.Return(jen.Id("_res"), jen.True(), jen.Id("_err")),
			)
		} else {
			d.Block(jen.Return(jen.Nil(), jen.False(), jen.Nil()))
		}
		protoDecls = append(protoDecls, d)
	}

	// Not() (Object, error)  <- "not"
	notDecl := jen.Func().Params(receiverParam()).Id("Not").Params().Parens(
		jen.List(jen.Qual(pathObject, "Object"), jen.Error()),
	)
	if defined["__not"] {
		notDecl.Block(jen.Return(protoCall("__not")))
	} else {
		// Without __not, ! negates the instance's truthiness, matching the
		// default behavior of built-in types.
		notDecl.Block(
			jen.List(jen.Id("_b"), jen.Id("_err")).Op(":=").Id(receiverName).Dot("ToBool").Call(),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.Nil(), jen.Id("_err"))),
			jen.Return(jen.Qual(pathObject, "Bool").Call(jen.Op("!").Id("_b")), jen.Nil()),
		)
	}
	protoDecls = append(protoDecls, notDecl)

	// Iter() ([]Object, error)  <- "iter" (returns an iterable)
	iterDecl := jen.Func().Params(receiverParam()).Id("Iter").Params().Parens(
		jen.List(jen.Index().Qual(pathObject, "Object"), jen.Error()),
	)
	if defined["__iter"] {
		iterDecl.Block(
			jen.List(jen.Id("_res"), jen.Id("_err")).Op(":=").Add(protoCall("__iter")),
			jen.If(jen.Id("_err").Op("!=").Nil()).Block(jen.Return(jen.Nil(), jen.Id("_err"))),
			jen.Return(jen.Id("_res").Dot("Iter").Call()),
		)
	} else {
		iterDecl.Block(jen.Return(jen.Nil(), errorf("%s does not support iteration")))
	}
	protoDecls = append(protoDecls, iterDecl)

	// Index(index) (Object, error)  <- "get_item"
	indexDecl := jen.Func().Params(receiverParam()).Id("Index").Params(
		jen.Id("index").Qual(pathObject, "Object"),
	).Parens(jen.List(jen.Qual(pathObject, "Object"), jen.Error()))
	if defined["__getitem"] {
		indexDecl.Block(jen.Return(protoCall("__getitem", jen.Id("index"))))
	} else {
		indexDecl.Block(jen.Return(jen.Nil(), errorf("%s is not indexable")))
	}
	protoDecls = append(protoDecls, indexDecl)

	// SetIndex(index, value) (bool, error)  <- "__setitem". Without it the type
	// reports "not handled" and object.SetIndex raises the shared error.
	setIndexDecl := jen.Func().Params(receiverParam()).Id("SetIndex").Params(
		jen.Id("index").Qual(pathObject, "Object"), jen.Id("value").Qual(pathObject, "Object"),
	).Parens(jen.List(jen.Bool(), jen.Error()))
	if defined["__setitem"] {
		setIndexDecl.Block(
			jen.List(jen.Id("_"), jen.Id("_err")).Op(":=").Add(protoCall("__setitem", jen.Id("index"), jen.Id("value"))),
			jen.Return(jen.True(), jen.Id("_err")),
		)
	} else {
		setIndexDecl.Block(jen.Return(jen.False(), jen.Nil()))
	}
	protoDecls = append(protoDecls, setIndexDecl)

	ctx.topDecls = append(ctx.topDecls, protoDecls...)

	getAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+len(typeDef.Methods)+2)
	attributeNames := make([]jen.Code, 0, len(typeDef.Fields)+len(typeDef.Methods)+2)
	seenAttributes := make(map[string]bool, cap(attributeNames))
	for _, field := range typeDef.Fields {
		getAttrCases = append(getAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Return(jen.Id(receiverName).Dot(field.Name), jen.Nil()),
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

	setAttrCases := make([]jen.Code, 0, len(typeDef.Fields)+1)
	for _, field := range typeDef.Fields {
		setAttrCases = append(setAttrCases,
			jen.Case(jen.Lit(field.Name)).Block(
				jen.Id(receiverName).Dot(field.Name).Op("=").Id("value"),
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

	for _, method := range typeDef.Methods {
		wrapperName := methodWrapperName(method.Name)

		callArgsName := ctx.localName("callArgs")
		fnOnError := func(errVar string) jen.Code {
			return tracedReturn(errVar, typeDef.Name, typeDef.Name+"."+method.Name, method.Position())
		}

		bodyPrefix := []jen.Code{
			jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
			jen.Id("_").Op("=").Id("builtin"),
		}
		defaultsDecl, defaultsName, err := ctx.emitParamDefaults(method.Parameters[1:])
		if err != nil {
			return nil, err
		}
		if defaultsDecl != nil {
			bodyPrefix = append(bodyPrefix, defaultsDecl)
		}
		bodyPrefix = append(bodyPrefix,
			ctx.emitParameterBinding(typeDef.Name+"."+method.Name, method.Parameters[1:], defaultsName, callArgsName, fnOnError)...,
		)
		bodyPrefix = append(bodyPrefix,
			jen.Var().Id("self").Qual(pathObject, "Object").Op("=").Id(receiverName),
			jen.Id("_").Op("=").Id("self"),
		)

		methodBody, err := ctx.transpileStatements(method.Body, fnOnError, "")
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

	callArgsName := ctx.localName("callArgs")
	shadowPositionalName := ctx.localName("positional")
	shadowKeywordName := ctx.localName("keyword")
	enrichedCallArgsName := ctx.localName("enriched")
	boundName := ctx.localName("bound")
	errVar := ctx.localName("err")

	constructorSetup := []jen.Code{
		jen.Id(shadowPositionalName).Op(":=").Append(jen.Qual(pathObject, "Args").Values(), jen.Id(callArgsName).Dot("Positional").Op("...")),
		jen.Id(shadowKeywordName).Op(":=").Qual(pathObject, "Kwargs").Values(),
	}
	constructorSetup = append(constructorSetup,
		jen.For(jen.List(jen.Id("_key"), jen.Id("_value")).Op(":=").Op("range").Id(callArgsName).Dot("Keyword")).Block(
			jen.Id(shadowKeywordName).Index(jen.Id("_key")).Op("=").Id("_value"),
		),
	)

	for index, field := range typeDef.Fields {
		if !field.HasDefault() {
			continue
		}
		defaultPre, defaultValue, err := ctx.transpileExpression(field.DefaultValue, onError)
		if err != nil {
			return nil, err
		}
		constructorSetup = append(constructorSetup,
			jen.If(
				jen.Len(jen.Id(shadowPositionalName)).Op("<=").Lit(index),
			).BlockFunc(func(group *jen.Group) {
				group.List(jen.Id("_"), jen.Id("_has_"+field.Name)).Op(":=").Id(shadowKeywordName).Index(jen.Lit(field.Name))
				group.If(jen.Op("!").Id("_has_" + field.Name)).Block(append(defaultPre,
					jen.Id(shadowKeywordName).Index(jen.Lit(field.Name)).Op("=").Add(defaultValue),
				)...)
			}),
		)
	}

	fieldNames := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		fieldNames = append(fieldNames, jen.Lit(field.Name))
	}

	constructorSetup = append(constructorSetup,
		jen.Id(enrichedCallArgsName).Op(":=").Qual(pathObject, "CallArgs").Values(jen.Dict{
			jen.Id("Positional"): jen.Id(shadowPositionalName),
			jen.Id("Keyword"):    jen.Id(shadowKeywordName),
		}),
		jen.List(jen.Id(boundName), jen.Id(errVar)).Op(":=").Qual(pathObject, "BindArguments").Call(
			jen.Lit(typeDef.Name),
			jen.Index().String().Values(fieldNames...),
			jen.Nil(),
			jen.Lit(""),
			jen.Lit(""),
			jen.Id(enrichedCallArgsName),
		),
		jen.Id("_").Op("=").Id(boundName),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(
			jen.Return(jen.Nil(), jen.Id(errVar)),
		),
	)

	instanceValues := make([]jen.Code, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		instanceValues = append(instanceValues,
			jen.Id(field.Name).Op(":").Id(boundName).Index(jen.Lit(field.Name)),
		)
	}

	constructorBody := append(constructorSetup,
		jen.Id("_instance").Op(":=").Op("&").Id(goTypeName).Values(instanceValues...),
		jen.Return(jen.Id("_instance"), jen.Nil()),
	)

	constructorClosure := jen.Func().Params(
		jen.Id(callArgsName).Qual(pathObject, "CallArgs"),
	).Parens(
		jen.List(jen.Qual(pathObject, "Object"), jen.Error()),
	).Block(constructorBody...)

	ctx.topDecls = append(ctx.topDecls,
		jen.Var().Id(ctorVarName).Qual(pathObject, "Object"),
	)

	constructor := jen.Id(ctorVarName).Op("=").Op("&").Qual(pathObject, "Function").Values(
		jen.Id("Name").Op(":").Lit(typeDef.Name),
		jen.Id("Fn").Op(":").Add(constructorClosure),
	)

	return []jen.Code{constructor}, nil
}

func (ctx *transpileContext) transpileReturn(return_ *ast.Return, onError errHandler) ([]jen.Code, error) {
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
	lhsPre, lhs, err := ctx.transpileExpression(operation.LHS, onError)
	if err != nil {
		return nil, nil, err
	}
	rhsPre, rhs, err := ctx.transpileExpression(operation.RHS, onError)
	if err != nil {
		return nil, nil, err
	}

	tmpVar := ctx.localName("tmp")
	preStmts := append(lhsPre, rhsPre...)

	// Equality is total for the built-in types, but a user-defined __cmp may
	// fail; like ordering, that error propagates.
	if operation.Operator == ast.Equal || operation.Operator == ast.NotEqual {
		eqVar := ctx.localName("eq")
		eqErrVar := ctx.localName("err")
		result := jen.Id(eqVar)
		if operation.Operator == ast.NotEqual {
			result = jen.Op("!").Id(eqVar)
		}
		preStmts = append(preStmts,
			jen.List(jen.Id(eqVar), jen.Id(eqErrVar)).Op(":=").Qual(pathObject, "Equals").Call(lhs, rhs),
			jen.If(jen.Id(eqErrVar).Op("!=").Nil()).Block(onError(eqErrVar)),
			jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Bool").Call(result),
		)
		return preStmts, jen.Id(tmpVar), nil
	}

	cmpVar := ctx.localName("cmp")
	errVar := ctx.localName("err")
	preStmts = append(preStmts,
		jen.List(jen.Id(cmpVar), jen.Id(errVar)).Op(":=").Qual(pathObject, "Compare").Call(lhs, rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.Var().Id(tmpVar).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Bool").Call(
			jen.Id(cmpVar).Op(operation.Operator).Lit(0),
		),
	)
	return preStmts, jen.Id(tmpVar), nil
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
	rhsPre, rhs, err := ctx.transpileExpression(operation.RHS, onError)
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
		jen.List(jen.Id(tmpVar), jen.Id(errVar)).Op(":=").Qual(pathObject, methodName).Call(lhs, rhs),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
	)
	return preStmts, jen.Id(tmpVar), nil
}

// transpileLogicalOperation lowers && and || with short-circuit evaluation: the
// RHS is only evaluated (and may therefore only error or produce side effects)
// when the LHS does not already fix the result. The result is always a Bool,
// matching the previous truthiness-based behavior.
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
	rhsVar := ctx.localName("rhs")
	rhsTruthyVar := ctx.localName("cond")
	rhsErrVar := ctx.localName("err")

	rhsBlock := append([]jen.Code{}, rhsPre...)
	rhsBlock = append(rhsBlock,
		jen.Id(rhsVar).Op(":=").Add(rhs),
		jen.List(jen.Id(rhsTruthyVar), jen.Id(rhsErrVar)).Op(":=").Id(rhsVar).Dot("ToBool").Call(),
		jen.If(jen.Id(rhsErrVar).Op("!=").Nil()).Block(onError(rhsErrVar)),
		jen.Id(tmpVar).Op("=").Qual(pathObject, "Bool").Call(jen.Id(rhsTruthyVar)),
	)

	preStmts := append([]jen.Code{}, lhsPre...)
	preStmts = append(preStmts,
		jen.List(jen.Id(lhsTruthyVar), jen.Id(errVar)).Op(":=").Add(lhs).Dot("ToBool").Call(),
		jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
		jen.Var().Id(tmpVar).Qual(pathObject, "Object"),
	)

	if operation.Operator == ast.And {
		preStmts = append(preStmts,
			jen.Id(tmpVar).Op("=").Qual(pathObject, "False"),
			jen.If(jen.Id(lhsTruthyVar)).Block(rhsBlock...),
		)
	} else { // ast.Or
		preStmts = append(preStmts,
			jen.Id(tmpVar).Op("=").Qual(pathObject, "True"),
			jen.If(jen.Op("!").Id(lhsTruthyVar)).Block(rhsBlock...),
		)
	}

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
		call = operand.Dot("Not").Call()
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
	return []jen.Code{
		jen.Id(exportsVar).Index(jen.Lit(export.Name)).Op("=").Id(export.Name),
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
		pre, _, err = ctx.transpileMemberExpression(v, onError)
		codes = pre
	case *ast.IndexExpression:
		var pre []jen.Code
		var value *jen.Statement
		pre, value, err = ctx.transpileIndexExpression(v, onError)
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
// before executing the module body. Every module-level binding is declared
// upfront (initialized to nil), all function values are assigned next, and the
// remaining statements run in source order. This makes forward references —
// including mutually recursive functions — work in generated code. Type
// constructor names are pre-registered for the same reason (their variables
// are emitted as package-level declarations).
func (ctx *transpileContext) transpileModuleStatements(stmts []ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	// A module's top level is a function body too — it becomes Execute() — so
	// its locals are eligible for the same specialisation.
	defer ctx.enterScope(stmts)()
	defer ctx.pushUserScope()()

	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case *ast.TypeDefine:
			ctx.moduleImports[v.Name] = v.Name + "Constructor"
		case *ast.FunctionDefine:
			// Function names are hoisted (mirroring the interpreter), so they
			// shadow built-ins for the whole module body.
			ctx.declareUserName(v.Name)
		}
	}

	var decls, funcAssigns, body []jen.Code
	declare := func(name string) {
		decl := jen.Var().Id(name).Qual(pathObject, "Object").Op("=").Qual(pathObject, "Nil")
		decl.Op(";").Id("_").Op("=").Id(name)
		decls = append(decls, decl)
	}
	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case *ast.FunctionDefine:
			declare(v.Name)
			funcValue, err := ctx.buildFunctionValue(v.Name, v.Position(), v.Parameters, v.Body)
			if err != nil {
				return nil, err
			}
			funcAssigns = append(funcAssigns, jen.Id(v.Name).Op("=").Add(funcValue))
		case *ast.Declare:
			// Split `var x = expr` into a hoisted declaration and an in-place
			// assignment so function closures may reference the variable
			// regardless of where their definition appears in the source.
			// A specialised variable is hoisted with its native type; it can
			// never be captured by a closure, since being named inside a nested
			// scope is exactly what disqualifies it from specialisation.
			if t := ctx.typeOf(v.Name); t.native() {
				decl := jen.Var().Id(v.Name).Id(goTypeOf(t))
				decl.Op(";").Id("_").Op("=").Id(v.Name)
				decls = append(decls, decl)

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
	result := append(decls, funcAssigns...)
	return append(result, body...), nil
}

func (ctx *transpileContext) transpileStatements(stmts []ast.Statement, onError errHandler, exportsVar string) ([]jen.Code, error) {
	// Each statement list renders as a Go block, so names declared inside it
	// must not shadow built-ins beyond the block's end.
	defer ctx.pushUserScope()()
	var result []jen.Code
	for _, stmt := range stmts {
		codes, err := ctx.transpileStatement(stmt, onError, exportsVar)
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
	absPath, err := ctx.resolveImportPath(importPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %v", importPath, err)
	}

	if _, ok := ctx.imported[absPath]; ok {
		return nil
	}
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
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		return fmt.Errorf("parse error in module %s: %v", importPath, err)
	}

	mod, ok := st.(*ast.Module)
	if !ok {
		return fmt.Errorf("internal error: unexpected AST type for module %s", importPath)
	}
	if err := semantic.CheckModule(mod); err != nil {
		return fmt.Errorf("semantic error in module %s: %v", importPath, err)
	}

	// Process sub-imports first (depth-first).
	subModuleImports := make(map[string]string)
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok {
			continue
		}
		if isPathImport(imp.Path) {
			subModuleImports[imp.Name] = imp.Name
			if err := ctx.transpilePathModuleToFile(imp.Path); err != nil {
				return err
			}
		} else {
			info, exists := knownModules[imp.Path]
			if !exists {
				return fmt.Errorf("unknown module in %s: %s", importPath, imp.Path)
			}
			subModuleImports[imp.Name] = info.varName
		}
	}

	key := ctx.moduleKey(absPath)
	pkgName := pathToPackageName(key)
	pkgDir := filepath.Join(ctx.outputDir, filepath.FromSlash(key))
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return fmt.Errorf("failed to create package directory %s: %v", pkgDir, err)
	}

	f := jen.NewFile(pkgName)

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
	ctx.moduleImports = subModuleImports
	defer func() { ctx.moduleImports = savedImports }()

	savedTopDecls := ctx.topDecls
	ctx.topDecls = nil
	defer func() { ctx.topDecls = savedTopDecls }()

	exportsVar := ctx.localName("exports")
	onError := func(errVar string) jen.Code {
		return tracedReturn(errVar, mod.Name, "<module>", modulePosition(mod))
	}

	stmts, err := ctx.transpileModuleStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return fmt.Errorf("transpile error in module %s: %v", importPath, err)
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
	}

	funcBody := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports via registry parameter.
	for name, info := range knownModules {
		if _, ok := subModuleImports[name]; ok {
			errVar := ctx.localName("err")
			funcBody = append(funcBody,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(info.executorPath, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports via closure that passes registry down.
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
		errVar := ctx.localName("err")
		funcBody = append(funcBody,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("registry").Dot("Load").Call(
				jen.Lit(subKey),
				jen.Func().Params().Parens(jen.List(
					jen.Qual(pathObject, "Object"), jen.Error(),
				)).Block(
					jen.Return(jen.Qual(subImportPath, "Execute").Call(jen.Id("registry"))),
				),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
	}

	funcBody = append(funcBody, stmts...)
	funcBody = append(funcBody,
		jen.Return(
			jen.Op("&").Qual(pathObject, "Module").Values(
				jen.Id("Members").Op(":").Id(exportsVar),
			),
			jen.Nil(),
		),
	)

	f.Func().Id("Execute").Params(
		jen.Id("registry").Op("*").Qual(pathObject, "Registry"),
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

	ctx.imported[absPath] = struct{}{}
	return nil
}

// generateMainFile generates output/main.go for the top-level module.
func (ctx *transpileContext) generateMainFile(mod *ast.Module) error {
	f := jen.NewFile("main")

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
	mainModuleImports := make(map[string]string)
	for _, stmt := range mod.Body {
		imp, ok := stmt.(*ast.Import)
		if !ok {
			continue
		}
		if isPathImport(imp.Path) {
			mainModuleImports[imp.Name] = imp.Name
		} else {
			info, exists := knownModules[imp.Path]
			if !exists {
				return fmt.Errorf("unknown module: %s", imp.Path)
			}
			mainModuleImports[imp.Name] = info.varName
		}
	}

	savedImports := ctx.moduleImports
	ctx.moduleImports = mainModuleImports
	defer func() { ctx.moduleImports = savedImports }()

	savedTopDecls := ctx.topDecls
	ctx.topDecls = nil
	defer func() { ctx.topDecls = savedTopDecls }()

	exportsVar := ctx.localName("exports")
	onError := func(errVar string) jen.Code {
		return tracedReturn(errVar, mod.Name, "<module>", modulePosition(mod))
	}

	stmts, err := ctx.transpileModuleStatements(mod.Body, onError, exportsVar)
	if err != nil {
		return err
	}

	for _, decl := range ctx.topDecls {
		f.Add(decl)
	}

	body := []jen.Code{
		jen.Id("builtin").Op(":=").Qual(pathExtension, "BuiltinsModule"),
		jen.Id("_").Op("=").Id("builtin"),
		jen.Id(exportsVar).Op(":=").Map(jen.String()).Qual(pathObject, "Object").Values(),
	}

	// Builtin module imports via _registry global.
	for name, info := range knownModules {
		if _, ok := mainModuleImports[name]; ok {
			errVar := ctx.localName("err")
			body = append(body,
				jen.List(jen.Id(info.varName), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
					jen.Lit(name),
					jen.Qual(info.executorPath, info.executorFunc),
				),
				jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
				jen.Id("_").Op("=").Id(info.varName),
			)
		}
	}

	// Path module imports via closure passing _registry down.
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
		errVar := ctx.localName("err")
		body = append(body,
			jen.List(jen.Id(imp.Name), jen.Id(errVar)).Op(":=").Id("_registry").Dot("Load").Call(
				jen.Lit(key),
				jen.Func().Params().Parens(jen.List(
					jen.Qual(pathObject, "Object"), jen.Error(),
				)).Block(
					jen.Return(jen.Qual(importPath, "Execute").Call(jen.Id("_registry"))),
				),
			),
			jen.If(jen.Id(errVar).Op("!=").Nil()).Block(onError(errVar)),
			jen.Id("_").Op("=").Id(imp.Name),
		)
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
