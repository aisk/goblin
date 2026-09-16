package semantic

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

type Diagnostic struct {
	Pos     token.Pos
	Kind    string
	Message string
}

type Error struct {
	Diagnostic Diagnostic
}

func (e *Error) Error() string {
	diag := e.Diagnostic
	return fmt.Sprintf("%s: semantic error: %s", formatPos(diag.Pos), diag.Message)
}

type symbol struct {
	name string
	// kind is "type" or "trait" for a declaration whose binding the
	// program may not reassign, and empty otherwise.
	kind string
}

type scope struct {
	parent  *scope
	symbols map[string]symbol
}

func newScope(parent *scope) *scope {
	return &scope{
		parent:  parent,
		symbols: make(map[string]symbol),
	}
}

func (s *scope) declare(name string) bool {
	if _, exists := s.symbols[name]; exists {
		return false
	}
	s.symbols[name] = symbol{name: name}
	return true
}

// resolve finds the symbol a name refers to from this scope.
func (s *scope) resolve(name string) (symbol, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if sym, exists := cur.symbols[name]; exists {
			return sym, true
		}
	}
	return symbol{}, false
}

func (s *scope) lookup(name string) bool {
	for cur := s; cur != nil; cur = cur.parent {
		if _, exists := cur.symbols[name]; exists {
			return true
		}
	}
	return false
}

type checker struct {
	currentScope *scope
	loopDepth    int
	funcDepth    int

	// traits holds the module's trait declarations checked so far, as trait
	// values carrying only the method shapes, so impl blocks can be validated
	// by the same object.CheckImpls the runtime uses. A trait is only visible
	// to the declarations after it.
	traits map[string]*object.Trait
	// traitNames lists every trait the module declares, to tell a trait used
	// before its declaration from a name that is no trait at all.
	traitNames map[string]bool
	// opaque marks traits whose shape is not fully known statically, because
	// they depend on a trait from an imported module. Types implementing one
	// are validated at runtime instead.
	opaque  map[*object.Trait]bool
	imports map[string]bool
}

// goReservedNames lists names the transpiler cannot emit as user identifiers.
// User names are emitted as Go identifiers verbatim, so a Go keyword would not
// compile, and the remaining entries would shadow identifiers every transpiled
// program references (the object/extension/fmt package aliases, the generated
// `builtin` and `_registry` helpers, and the top-level Execute/main wrappers)
// — letting them through leaks Go compiler errors instead of a Goblin
// diagnostic. The names that double as Goblin keywords never parse as
// identifiers anyway, but are listed for completeness.
var goReservedNames = map[string]struct{}{
	"break": {}, "case": {}, "chan": {}, "const": {}, "continue": {},
	"default": {}, "defer": {}, "else": {}, "fallthrough": {}, "for": {},
	"func": {}, "go": {}, "goto": {}, "if": {}, "import": {},
	"interface": {}, "map": {}, "package": {}, "range": {}, "return": {},
	"select": {}, "struct": {}, "switch": {}, "type": {}, "var": {},
	"object": {}, "extension": {}, "builtin": {}, "fmt": {},
	"_registry": {}, "Execute": {}, "main": {},
}

func (c *checker) checkReservedName(pos token.Pos, name string) error {
	if _, ok := goReservedNames[name]; ok {
		return c.newError(pos, "'%s' is a reserved name", name)
	}
	if isTranspilerScratchName(name) {
		return c.newError(pos, "'%s' is a reserved name (identifiers of the form _name_N are reserved for the transpiler)", name)
	}
	return nil
}

// isTranspilerScratchName reports whether name matches the shape of the
// transpiler's generated temporaries: a leading underscore and a trailing
// underscore-digits suffix (e.g. _err_0, _exports_1, _math_module_2). The
// whole family is reserved so generated locals can never collide with user
// identifiers in the same scope.
func isTranspilerScratchName(name string) bool {
	if len(name) < 4 || name[0] != '_' {
		return false
	}
	i := len(name) - 1
	for i > 0 && name[i] >= '0' && name[i] <= '9' {
		i--
	}
	return i < len(name)-1 && i > 1 && name[i] == '_'
}

func CheckModule(mod *ast.Module) error {
	c := &checker{
		currentScope: newScope(nil),
		traits:       make(map[string]*object.Trait),
		traitNames:   make(map[string]bool),
		opaque:       make(map[*object.Trait]bool),
		imports:      make(map[string]bool),
	}

	// Module-level import, function, type, and trait names are hoisted: they are
	// visible across the whole module regardless of definition order, so
	// mutually recursive functions work. Calling a function before the
	// statement defining it has executed is still a runtime error.
	for _, stmt := range mod.Body {
		var name string
		var pos token.Pos
		switch v := stmt.(type) {
		case *ast.Import:
			name, pos = v.Name, v.Position()
			c.imports[v.Name] = true
		case *ast.TraitDefine:
			name, pos = v.Name, v.Position()
			c.traitNames[v.Name] = true
		case *ast.FunctionDefine:
			name, pos = v.Name, v.Position()
		case *ast.TypeDefine:
			name, pos = v.Name, v.Position()
		default:
			continue
		}
		if err := c.checkReservedName(pos, name); err != nil {
			return err
		}
		if !c.currentScope.declare(name) {
			return c.newError(pos, "duplicate declaration in same scope: %s", name)
		}
		// Types and traits are bound when the module loads, so reassigning
		// one could not change what the impls and constructions see.
		switch stmt.(type) {
		case *ast.TypeDefine:
			c.currentScope.symbols[name] = symbol{name: name, kind: "type"}
		case *ast.TraitDefine:
			c.currentScope.symbols[name] = symbol{name: name, kind: "trait"}
		}
	}

	return c.checkStatements(mod.Body, true)
}

func (c *checker) checkStatements(stmts []ast.Statement, isModuleScope bool) error {
	for _, stmt := range stmts {
		if err := c.checkStatement(stmt, isModuleScope); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) withScope(fn func() error) error {
	prev := c.currentScope
	c.currentScope = newScope(prev)
	defer func() {
		c.currentScope = prev
	}()
	return fn()
}

func (c *checker) checkStatement(stmt ast.Statement, isModuleScope bool) error {
	switch v := stmt.(type) {
	case *ast.Import:
		if !isModuleScope {
			return c.newError(v.Position(), "import is only allowed at module scope")
		}
		return nil
	case *ast.TypeDefine:
		if !isModuleScope {
			return c.newError(v.Position(), "type is only allowed at module scope")
		}
		// The type name itself was already hoisted by CheckModule.

		seenFields := make(map[string]struct{}, len(v.Fields))
		seenDefault := false
		for _, field := range v.Fields {
			if err := c.checkReservedName(field.Pos, field.Name); err != nil {
				return err
			}
			if _, ok := seenFields[field.Name]; ok {
				return c.newError(field.Pos, "duplicate type field name: %s", field.Name)
			}
			seenFields[field.Name] = struct{}{}

			if field.HasDefault() {
				seenDefault = true
				if err := c.checkExpression(field.DefaultValue); err != nil {
					return err
				}
				continue
			}
			if seenDefault {
				return c.newError(field.Pos, "required type field cannot appear after default field: %s", field.Name)
			}
		}

		seenMethods := make(map[string]struct{}, len(v.Methods))
		for _, method := range v.Methods {
			if err := c.checkReservedName(method.Position(), method.Name); err != nil {
				return err
			}
			if _, ok := seenMethods[method.Name]; ok {
				return c.newError(method.Position(), "duplicate type method name: %s", method.Name)
			}
			if _, ok := seenFields[method.Name]; ok {
				return c.newError(method.Position(), "type method name conflicts with field name: %s", method.Name)
			}
			seenMethods[method.Name] = struct{}{}

			if !declaresSelf(method) {
				return c.newError(method.Position(), "type method must declare 'self' as the first parameter")
			}
			if err := c.checkMethod(method); err != nil {
				return err
			}
		}
		return c.checkImpls(v)
	case *ast.TraitDefine:
		if !isModuleScope {
			return c.newError(v.Position(), "trait is only allowed at module scope")
		}
		return c.checkTrait(v)
	case *ast.Declare:
		if err := c.checkExpression(v.Value); err != nil {
			return err
		}
		if err := c.checkReservedName(v.Position(), v.Name); err != nil {
			return err
		}
		if !c.currentScope.declare(v.Name) {
			return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Name)
		}
		return nil
	case *ast.Assign:
		sym, ok := c.currentScope.resolve(v.Target)
		if !ok {
			return c.newError(v.Position(), "assignment to undefined identifier: %s", v.Target)
		}
		if sym.kind != "" {
			return c.newError(v.Position(), "cannot assign to %s %s", sym.kind, v.Target)
		}
		return c.checkExpression(v.Value)
	case *ast.SetIndex:
		if err := c.checkExpression(v.Object); err != nil {
			return err
		}
		if err := c.checkExpression(v.Index); err != nil {
			return err
		}
		return c.checkExpression(v.Value)
	case *ast.SetAttr:
		if err := c.checkExpression(v.Object); err != nil {
			return err
		}
		return c.checkExpression(v.Value)
	case *ast.FunctionDefine:
		// Module-level function names were already hoisted by CheckModule;
		// nested functions are declared where they appear.
		if !isModuleScope {
			if err := c.checkReservedName(v.Position(), v.Name); err != nil {
				return err
			}
			if !c.currentScope.declare(v.Name) {
				return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Name)
			}
		}
		return c.checkFunction(v.Parameters, v.Body)
	case *ast.IfElse:
		if err := c.checkExpression(v.Condition); err != nil {
			return err
		}
		if err := c.withScope(func() error {
			return c.checkStatements(v.IfBody, false)
		}); err != nil {
			return err
		}
		return c.withScope(func() error {
			return c.checkStatements(v.ElseBody, false)
		})
	case *ast.While:
		if err := c.checkExpression(v.Condition); err != nil {
			return err
		}
		return c.withScope(func() error {
			c.loopDepth++
			defer func() { c.loopDepth-- }()
			return c.checkStatements(v.Body, false)
		})
	case *ast.For:
		if err := c.checkExpression(v.Iterator); err != nil {
			return err
		}
		return c.withScope(func() error {
			c.loopDepth++
			defer func() { c.loopDepth-- }()

			if err := c.checkReservedName(v.Position(), v.Variable); err != nil {
				return err
			}
			if !c.currentScope.declare(v.Variable) {
				return c.newError(v.Position(), "duplicate declaration in same scope: %s", v.Variable)
			}
			// The range binding is in the loop scope; the body is a nested block,
			// so `var x` may shadow `for x` just as it can in Go.
			return c.withScope(func() error {
				return c.checkStatements(v.Body, false)
			})
		})
	case *ast.Break:
		if c.loopDepth == 0 {
			return c.newError(v.Position(), "break used outside loop")
		}
		return nil
	case *ast.Continue:
		if c.loopDepth == 0 {
			return c.newError(v.Position(), "continue used outside loop")
		}
		return nil
	case *ast.Return:
		if c.funcDepth == 0 {
			return c.newError(v.Position(), "return used outside function")
		}
		return c.checkExpression(v.Value)
	case *ast.Raise:
		return c.checkExpression(v.Value)
	case *ast.TryCatch:
		if err := c.withScope(func() error {
			return c.checkStatements(v.TryBody, false)
		}); err != nil {
			return err
		}
		return c.withScope(func() error {
			if err := c.checkReservedName(v.Position(), v.CatchVar); err != nil {
				return err
			}
			c.currentScope.declare(v.CatchVar)
			return c.checkStatements(v.CatchBody, false)
		})
	case *ast.Export:
		if !isModuleScope {
			return c.newError(v.Position(), "export is only allowed at module scope")
		}
		if !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "export of undefined identifier: %s", v.Name)
		}
		return nil
	case ast.Expression:
		if !hasEffect(v) {
			return c.newError(v.Position(), "expression value is not used")
		}
		return c.checkExpression(v)
	default:
		return nil
	}
}

// declaresSelf reports whether a method's first parameter is a plain `self`.
func declaresSelf(method *ast.FunctionDefine) bool {
	if len(method.Parameters) == 0 {
		return false
	}
	self := method.Parameters[0]
	return self.Name == "self" && !self.VarArgs && !self.KwArgs && !self.HasDefault()
}

// checkMethod checks a method body, of a type or of a trait. Default
// expressions are evaluated in the defining scope, where fields and self are
// not visible; inside the body only the parameters are in scope, fields are
// reached through self.
func (c *checker) checkMethod(method *ast.FunctionDefine) error {
	if err := c.checkParameterDefaults(method.Parameters); err != nil {
		return err
	}
	return c.withScope(func() error {
		c.funcDepth++
		defer func() { c.funcDepth-- }()

		if err := c.checkParameterOrder(method.Parameters); err != nil {
			return err
		}
		for _, param := range method.Parameters {
			if err := c.checkReservedName(param.Pos, param.Name); err != nil {
				return err
			}
			if !c.currentScope.declare(param.Name) {
				return c.newError(param.Pos, "duplicate parameter name: %s", param.Name)
			}
		}
		return c.checkStatements(method.Body, false)
	})
}

// checkTraitMethod checks what every trait method shares, in a trait
// declaration or an impl block: self first, and a fixed parameter list.
func (c *checker) checkTraitMethod(method *ast.FunctionDefine) error {
	if err := c.checkReservedName(method.Position(), method.Name); err != nil {
		return err
	}
	if !declaresSelf(method) {
		return c.newError(method.Position(), "trait method must declare 'self' as the first parameter")
	}
	for _, param := range method.Parameters {
		if param.VarArgs || param.KwArgs {
			return c.newError(param.Pos, "trait method '%s' cannot use variadic or keyword parameters", method.Name)
		}
		if param.HasDefault() {
			return c.newError(param.Pos, "trait method '%s' cannot declare default parameter values", method.Name)
		}
	}
	return nil
}

// checkTrait validates a trait declaration and makes it visible to the
// declarations that follow.
func (c *checker) checkTrait(def *ast.TraitDefine) error {
	opaque := false
	deps := make([]*object.Trait, 0, len(def.Deps))
	for _, ref := range def.Deps {
		dep, known, err := c.resolveTrait(ref, "trait "+def.Name)
		if err != nil {
			return err
		}
		if !known || c.opaque[dep] {
			opaque = true
			continue
		}
		deps = append(deps, dep)
	}

	methods := make([]object.TraitMethod, 0, len(def.Methods))
	seen := make(map[string]bool, len(def.Methods))
	for _, method := range def.Methods {
		if err := c.checkTraitMethod(method); err != nil {
			return err
		}
		if seen[method.Name] {
			return c.newError(method.Position(), "duplicate trait method name: %s", method.Name)
		}
		seen[method.Name] = true
		if method.Body != nil {
			if err := c.checkMethod(method); err != nil {
				return err
			}
		}
		methods = append(methods, object.TraitMethod{
			Name:     method.Name,
			Arity:    len(method.Parameters),
			Required: method.Body == nil,
		})
	}

	trait := object.NewTrait(def.Name, deps, methods)
	if opaque {
		c.opaque[trait] = true
	}
	c.traits[def.Name] = trait
	return nil
}

// resolveTrait finds the trait an impl block or a dependency list names. A
// bare name is a trait declared earlier in the module or a built-in trait. A
// module-qualified name must go through an import, but what it refers to is
// only known at runtime: known is false then.
// site says where the reference appears ("impl Shape", "trait Titled").
func (c *checker) resolveTrait(ref *ast.TraitRef, site string) (trait *object.Trait, known bool, err error) {
	if ref.Module != "" {
		if !c.imports[ref.Module] || !c.currentScope.lookup(ref.Module) {
			return nil, false, c.newError(ref.Pos, object.ErrFmtTraitNotDefined, site, ref)
		}
		return nil, false, nil
	}
	if trait, ok := c.traits[ref.Name]; ok && c.currentScope.lookup(ref.Name) {
		return trait, true, nil
	}
	if c.traitNames[ref.Name] {
		return nil, false, c.newError(ref.Pos, object.ErrFmtTraitNotDefined, site, ref)
	}
	if c.currentScope.lookup(ref.Name) {
		return nil, false, c.newError(ref.Pos, object.ErrFmtNotATrait, site, ref)
	}
	if trait, ok := object.BuiltinTraits[ref.Name]; ok {
		return trait, true, nil
	}
	return nil, false, c.newError(ref.Pos, object.ErrFmtTraitNotDefined, site, ref)
}

// checkImpls validates a type's impl blocks: their methods like any trait
// method, and the impls as a whole with object.CheckImpls when every trait
// involved is known statically.
func (c *checker) checkImpls(def *ast.TypeDefine) error {
	specs := make([]object.ImplSpec, 0, len(def.Impls))
	static := true
	for _, block := range def.Impls {
		trait, known, err := c.resolveTrait(block.Trait, "impl "+block.Trait.String())
		if err != nil {
			return err
		}
		methods := make([]object.ImplMethod, len(block.Methods))
		for i, method := range block.Methods {
			if err := c.checkTraitMethod(method); err != nil {
				return err
			}
			if err := c.checkMethod(method); err != nil {
				return err
			}
			methods[i] = object.ImplMethod{Name: method.Name, Arity: len(method.Parameters)}
		}
		if !known || c.opaque[trait] {
			static = false
			continue
		}
		specs = append(specs, object.ImplSpec{Trait: trait, Methods: methods})
	}
	if !static {
		return nil
	}
	issue := object.CheckImpls(def.Name, specs)
	if issue == nil {
		return nil
	}
	block := def.Impls[issue.Impl]
	pos := block.Trait.Pos
	if issue.Method >= 0 {
		pos = block.Methods[issue.Method].Position()
	}
	return c.newError(pos, "%s", issue.Message)
}

// hasEffect reports whether an expression may do something when evaluated on
// its own, and so is allowed to stand as a statement. Calls run code, and
// index and member access can run trait methods. Anything else, such
// as `a + b` or a lone literal, only computes a value, so as a statement it
// is almost certainly a mistake: with newlines ending statements, a stray
// `- 2` line is exactly this shape.
func hasEffect(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.CallExpression, *ast.FunctionCall, *ast.IndexExpression, *ast.MemberExpression:
		return true
	}
	return false
}

// checkFunction validates a function's parameter list and body in a fresh
// scope. It backs both named function definitions and anonymous function
// literals.
func (c *checker) checkFunction(params []*ast.Parameter, body []ast.Statement) error {
	// Default expressions are evaluated in the defining scope, so they are
	// checked before the parameters come into scope.
	if err := c.checkParameterDefaults(params); err != nil {
		return err
	}
	return c.withScope(func() error {
		c.funcDepth++
		defer func() { c.funcDepth-- }()

		if err := c.checkParameterOrder(params); err != nil {
			return err
		}

		for _, param := range params {
			if err := c.checkReservedName(param.Pos, param.Name); err != nil {
				return err
			}
			if !c.currentScope.declare(param.Name) {
				return c.newError(param.Pos, "duplicate parameter name: %s", param.Name)
			}
		}

		return c.checkStatements(body, false)
	})
}

// checkParameterDefaults validates default expressions in the current
// (enclosing) scope, before the parameters themselves are declared.
func (c *checker) checkParameterDefaults(params []*ast.Parameter) error {
	for _, param := range params {
		if param.HasDefault() {
			if err := c.checkExpression(param.Default); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkParameterOrder validates the parameter list shape: unique names, *args
// last or followed only by **kwargs, **kwargs last, and no required parameter
// after a defaulted one.
func (c *checker) checkParameterOrder(params []*ast.Parameter) error {
	seen := make(map[string]struct{}, len(params))
	seenDefault := false
	for i, param := range params {
		if _, ok := seen[param.Name]; ok {
			return c.newError(param.Pos, "duplicate parameter name: %s", param.Name)
		}
		seen[param.Name] = struct{}{}
		if param.KwArgs {
			if i != len(params)-1 {
				return c.newError(param.Pos, "kwargs parameter must be the last parameter")
			}
			continue
		}
		if param.VarArgs {
			if i < len(params)-1 && !(i == len(params)-2 && params[len(params)-1].KwArgs) {
				return c.newError(param.Pos, "args parameter must be the last parameter or followed by kwargs")
			}
			continue
		}
		if param.HasDefault() {
			seenDefault = true
		} else if seenDefault {
			return c.newError(param.Pos, "required parameter cannot appear after default parameter: %s", param.Name)
		}
	}
	return nil
}

func (c *checker) checkExpression(expr ast.Expression) error {
	switch v := expr.(type) {
	case *ast.FunctionLiteral:
		return c.checkFunction(v.Parameters, v.Body)
	case *ast.Identifier:
		if !isBuiltin(v.Name) && !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "undefined identifier: %s", v.Name)
		}
		return nil
	case *ast.Literal:
		return nil
	case *ast.FunctionCall:
		if !isBuiltin(v.Name) && !c.currentScope.lookup(v.Name) {
			return c.newError(v.Position(), "undefined identifier: %s", v.Name)
		}
		return c.checkCallArguments(v.Args)
	case *ast.CallExpression:
		if id, ok := v.Callee.(*ast.Identifier); ok && isBuiltin(id.Name) {
			return c.checkCallArguments(v.Args)
		}
		if err := c.checkExpression(v.Callee); err != nil {
			return err
		}
		return c.checkCallArguments(v.Args)
	case *ast.BinaryOperation:
		if err := c.checkExpression(v.LHS); err != nil {
			return err
		}
		return c.checkExpression(v.RHS)
	case *ast.UnaryOperation:
		return c.checkExpression(v.Operand)
	case *ast.ListLiteral:
		for _, elem := range v.Elements {
			if err := c.checkExpression(elem); err != nil {
				return err
			}
		}
		return nil
	case *ast.DictLiteral:
		for _, elem := range v.Elements {
			if err := c.checkExpression(elem.Key); err != nil {
				return err
			}
			if err := c.checkExpression(elem.Value); err != nil {
				return err
			}
		}
		return nil
	case *ast.IndexExpression:
		if err := c.checkExpression(v.Object); err != nil {
			return err
		}
		return c.checkExpression(v.Index)
	case *ast.MemberExpression:
		return c.checkExpression(v.Object)
	default:
		return nil
	}
}

func (c *checker) newError(pos token.Pos, format string, args ...any) error {
	return &Error{
		Diagnostic: Diagnostic{
			Pos:     pos,
			Kind:    "semantic",
			Message: fmt.Sprintf(format, args...),
		},
	}
}

func formatPos(pos token.Pos) string {
	if pos.Line <= 0 {
		return "<unknown>"
	}
	if src, ok := pos.Context.(token.Sourcer); ok {
		return fmt.Sprintf("%s:%d:%d", src.Source(), pos.Line, pos.Column)
	}
	return fmt.Sprintf("%d:%d", pos.Line, pos.Column)
}

func isBuiltin(name string) bool {
	_, ok := extension.BuiltinsModule.Members[name]
	return ok
}

func (c *checker) checkCallArguments(args []ast.CallArgument) error {
	seenKeyword := false
	seenKeywordNames := make(map[string]struct{})
	for i, arg := range args {
		switch arg.Kind {
		case ast.CallArgumentStarred:
			if seenKeyword {
				return c.newError(arg.Expr.Position(), "positional argument cannot appear after keyword arguments")
			}
			if i != len(args)-1 {
				return c.newError(arg.Expr.Position(), "starred argument must be the last argument")
			}
		case ast.CallArgumentKeyword, ast.CallArgumentKeywordUnpack:
			seenKeyword = true
			if arg.Kind == ast.CallArgumentKeyword {
				if _, ok := seenKeywordNames[arg.Name]; ok {
					return c.newError(arg.NamePos, "duplicate keyword argument: %s", arg.Name)
				}
				seenKeywordNames[arg.Name] = struct{}{}
			}
		case ast.CallArgumentPositional:
			if seenKeyword {
				return c.newError(arg.Expr.Position(), "positional argument cannot appear after keyword arguments")
			}
		}
		if err := c.checkExpression(arg.Expr); err != nil {
			return err
		}
	}
	return nil
}
