package transpiler

import (
	"fmt"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

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
