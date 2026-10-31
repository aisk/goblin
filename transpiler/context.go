package transpiler

import (
	"fmt"
	"strings"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

// generator transpiles one Goblin module into one Go file. Module-level
// state lives directly on it; the state of the function body being generated
// is swapped as a unit by enterFunction.
type generator struct {
	prog *program
	mod  *ast.Module
	// key is the module's key within the program, empty for the entry module.
	key string
	// label names the module in transpile errors: its import path, empty for
	// the entry module.
	label string
	// baseDir is the module's directory, which its relative imports resolve
	// against.
	baseDir string

	// topDecls collects the file's package-level declarations: types and
	// their methods, module variables, direct functions.
	topDecls []jen.Code

	// moduleVars maps the module-level names whose Go variable is not the
	// name itself: an imported module (the package-level variable holding it)
	// and a type (its constructor variable, the bare name being its struct).
	moduleVars map[string]string
	// exportsVar is the Execute local collecting the module's exports.
	exportsVar string

	// rangeNative reports whether `range` resolves to the builtin throughout
	// the module, making `for x in range(a, b)` eligible for lowering to a
	// native counting loop. The inference pass and transpileFor must agree on
	// it, so it is a whole-module property.
	rangeNative bool
	// directFns and directCtors hold the module-level functions and types
	// eligible for direct-call lowering (see directcall.go), and
	// moduleScopeIdx is the index of the module's user scope: a call site may
	// only be lowered while no deeper scope shadows the name.
	directFns      map[string]directFn
	directCtors    map[string]directFn
	moduleScopeIdx int
	// sigs holds the inferred native signatures of the module's closed direct
	// functions (see signatures.go).
	sigs map[string]*fnSig

	body bodyState

	// userScopes tracks user-declared names per lexical scope so they shadow
	// built-in functions the same way the interpreter's scope chain does.
	userScopes []map[string]struct{}

	// errorPos is the position of the statement currently being transpiled.
	// Frame-producing error handlers read it so tracebacks point at the
	// failing statement, mirroring the interpreter's positionError tagging.
	errorPos token.Pos
}

// bodyState describes the function body being generated.
type bodyState struct {
	// types holds the specialised native types inferred for the body's
	// locals (see typeinfer.go). Names absent from it are ordinary
	// object.Object values.
	types map[string]staticType
	// ret is the native return type of a closed direct function, tyDynamic
	// for an ordinary object.Object result.
	ret staticType
	// self describes the receiver while a type method body is generated, so
	// `self.field` and `self.method(...)` become direct Go field access and
	// method calls. It is nil outside method bodies, inside nested function
	// bodies, and in a method that rebinds self.
	self *selfInfo
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
func (g *generator) selfMember(expr ast.Expression, name string) (info *selfInfo, ok bool) {
	if g.body.self == nil {
		return nil, false
	}
	ident, isIdent := expr.(*ast.Identifier)
	if !isIdent || ident.Name != "self" {
		return nil, false
	}
	return g.body.self, true
}

// framePos returns the position traceback frames should carry: the current
// statement when one is being transpiled, fallback (typically the enclosing
// definition) otherwise.
func (g *generator) framePos(fallback token.Pos) token.Pos {
	if g.errorPos.Line != 0 {
		return g.errorPos
	}
	return fallback
}

// pushUserScope opens a lexical scope for user-declared names and returns a
// function closing it. User names shadow built-in functions, mirroring the
// interpreter's scope-chain-first name resolution.
func (g *generator) pushUserScope() func() {
	g.userScopes = append(g.userScopes, map[string]struct{}{})
	return func() { g.userScopes = g.userScopes[:len(g.userScopes)-1] }
}

func (g *generator) declareUserName(name string) {
	if len(g.userScopes) > 0 {
		g.userScopes[len(g.userScopes)-1][name] = struct{}{}
	}
}

func (g *generator) isUserName(name string) bool {
	for _, scope := range g.userScopes {
		if _, ok := scope[name]; ok {
			return true
		}
	}
	return false
}

// enterFunction opens the scope of a function body: a user scope declaring
// the parameters, the body's inferred type environment, and its receiver
// description (nil outside methods). seed and ret are the parameter types and
// native return type of a closed direct function, nil and tyDynamic
// otherwise. The returned function restores the enclosing body.
//
// The type environment must come out identical to the one the signature
// inference computed for this body, which is why it is built by the same
// function from the same inputs: the module-wide rangeNative, the seed and
// the signatures. Parameters are declared first so the inference sees them
// as potential shadows.
func (g *generator) enterFunction(params []*ast.Parameter, body []ast.Statement, seed map[string]staticType, ret staticType, self *selfInfo) func() {
	popScope := g.pushUserScope()
	for _, param := range params {
		g.declareUserName(param.Name)
	}
	saved := g.body
	g.body = bodyState{types: inferLocals(body, g.rangeNative, seed, g.sigs), ret: ret, self: self}
	return func() {
		g.body = saved
		popScope()
	}
}

// typeOf reports the specialised native type of a name, or tyDynamic when the
// name is an ordinary boxed value.
func (g *generator) typeOf(name string) staticType {
	if t, ok := g.body.types[name]; ok {
		return t
	}
	return tyDynamic
}

// nativeTypeOf reports whether an expression can be evaluated as a native Go
// expression in the current scope, and with what type.
func (g *generator) nativeTypeOf(expr ast.Expression) staticType {
	return staticTypeOf(expr, g.body.types, g.sigs)
}

func (g *generator) localName(prefix string) string {
	name := fmt.Sprintf("_%s_%d", prefix, g.prog.names)
	g.prog.names++
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
