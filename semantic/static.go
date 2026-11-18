package semantic

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
)

// Static call and attribute checks.
//
// A name bound by `func`, by `type`, or by a `var` holding a function
// literal or a construction, and never assigned afterwards, means the same
// thing wherever it is visible. Calls through such a name are checked against
// the parameter list, and attribute access on a value known to be an instance
// of a module type against the type's fields and methods. Everything else
// (parameters, loop variables, anything reassigned, values from other
// modules) stays dynamic and is left to the runtime.
//
// Whether a name is ever assigned is only known once the whole module has
// been walked, so the checks are queued while walking and run at the end.

// signature is the parameter list of a function, method or type constructor
// as argument binding sees it (object.BindArguments).
type signature struct {
	name    string
	params  []sigParam
	varArgs bool
	kwArgs  bool
}

type sigParam struct {
	name     string
	required bool
}

// functionSignature describes a function called as name. skip drops the
// leading parameters the caller does not supply (a method's self).
func functionSignature(name string, params []*ast.Parameter, skip int) *signature {
	sig := &signature{name: name}
	for _, p := range params[skip:] {
		switch {
		case p.VarArgs:
			sig.varArgs = true
		case p.KwArgs:
			sig.kwArgs = true
		default:
			sig.params = append(sig.params, sigParam{name: p.Name, required: !p.HasDefault()})
		}
	}
	return sig
}

func constructorSignature(def *ast.TypeDefine) *signature {
	sig := &signature{name: def.Name}
	for _, f := range def.Fields {
		sig.params = append(sig.params, sigParam{name: f.Name, required: !f.HasDefault()})
	}
	return sig
}

// later queues a check that depends on whether sym is ever assigned. It is
// skipped when it is; a nil sym means the check holds unconditionally.
func (c *checker) later(sym *symbol, check func() error) {
	c.pending = append(c.pending, func() error {
		if sym != nil && sym.reassigned {
			return nil
		}
		return check()
	})
}

func (c *checker) runPending() error {
	for _, check := range c.pending {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

// instanceOf reports the module type expr is known to be an instance of: a
// construction `T(...)`, or a name bound to one (a method's self included).
// The symbol is the one the answer depends on, if any.
func (c *checker) instanceOf(expr ast.Expression) (*symbol, *ast.TypeDefine) {
	switch v := expr.(type) {
	case *ast.Identifier:
		if sym, ok := c.currentScope.resolve(v.Name); ok && sym.instance != nil {
			return sym, sym.instance
		}
	case *ast.CallExpression:
		if id, ok := v.Callee.(*ast.Identifier); ok {
			if sym, ok := c.currentScope.resolve(id.Name); ok && sym.typeDef != nil {
				return nil, sym.typeDef
			}
		}
	}
	return nil, nil
}

// valueFacts reports what a variable declared with value is known to hold.
// value must be inspected before the variable itself comes into scope.
func (c *checker) valueFacts(name string, value ast.Expression) (sig *signature, instance *ast.TypeDefine) {
	if fn, ok := value.(*ast.FunctionLiteral); ok {
		// Named as the runtime names a function literal, so the message
		// matches the one a call the checker cannot see raises.
		return functionSignature("<lambda>", fn.Parameters, 0), nil
	}
	// A variable copied from another variable is not tracked: only a
	// construction makes the type certain without following assignments.
	if call, ok := value.(*ast.CallExpression); ok {
		_, instance = c.instanceOf(call)
	}
	return nil, instance
}

// noteCall queues the arity check of a call whose callee is statically
// known: a plain name bound to a function or type, or a method of a known
// instance.
func (c *checker) noteCall(call *ast.CallExpression) {
	switch callee := call.Callee.(type) {
	case *ast.Identifier:
		sym, ok := c.currentScope.resolve(callee.Name)
		if !ok {
			return
		}
		sig := sym.callable
		if sym.typeDef != nil {
			sig = constructorSignature(sym.typeDef)
		}
		if sig != nil {
			c.later(sym, func() error { return c.checkArity(sig, call) })
		}
	case *ast.MemberExpression:
		sym, def := c.instanceOf(callee.Object)
		if def == nil {
			return
		}
		for _, method := range def.Methods {
			if method.Name == callee.Property {
				sig := functionSignature(def.Name+"."+method.Name, method.Parameters, 1)
				c.later(sym, func() error { return c.checkArity(sig, call) })
				return
			}
		}
	}
}

// noteAttr queues the check that name is an attribute of obj when obj is a
// known instance. A store may only target a field.
func (c *checker) noteAttr(obj ast.Expression, name string, at ast.Statement, store bool) {
	sym, def := c.instanceOf(obj)
	if def == nil {
		return
	}
	c.later(sym, func() error {
		if hasAttribute(def, name, store) {
			return nil
		}
		if store && hasAttribute(def, name, false) {
			return c.newError(at.Position(), object.ErrFmtAssignMethod, name, def.Name)
		}
		return c.newError(at.Position(), "%s has no attribute '%s'", def.Name, name)
	})
}

// hasAttribute mirrors an instance's GetAttr and SetAttr: fields can be read
// and written; methods only read. An instance has no other attributes.
func hasAttribute(def *ast.TypeDefine, name string, store bool) bool {
	for _, field := range def.Fields {
		if field.Name == name {
			return true
		}
	}
	if store {
		return false
	}
	for _, method := range def.Methods {
		if method.Name == name {
			return true
		}
	}
	return false
}

// checkArity reports a call that can never bind to sig, with the wording
// object.BindArguments uses at runtime. Unpacked arguments (`*xs`, `**kw`)
// supply an unknown number of values, so only what holds whatever they
// contain is reported.
func (c *checker) checkArity(sig *signature, call *ast.CallExpression) error {
	positional := 0
	unpacked := false
	keywords := map[string]bool{}
	for _, arg := range call.Args {
		switch arg.Kind {
		case ast.CallArgumentPositional:
			positional++
		case ast.CallArgumentKeyword:
			keywords[arg.Name] = true
		default:
			unpacked = true
		}
	}

	if !sig.varArgs && positional > len(sig.params) {
		return c.newError(call.Position(), "%s() takes %d positional arguments, got %d", sig.name, len(sig.params), positional)
	}
	for _, arg := range call.Args {
		if arg.Kind != ast.CallArgumentKeyword {
			continue
		}
		index := -1
		for i, p := range sig.params {
			if p.name == arg.Name {
				index = i
				break
			}
		}
		switch {
		case index >= 0 && index < positional:
			return c.newError(arg.NamePos, "%s() got multiple values for argument '%s'", sig.name, arg.Name)
		case index < 0 && !sig.kwArgs:
			return c.newError(arg.NamePos, "%s() got an unexpected keyword argument '%s'", sig.name, arg.Name)
		}
	}
	if unpacked {
		return nil
	}
	for i, p := range sig.params {
		if i >= positional && p.required && !keywords[p.name] {
			return c.newError(call.Position(), "%s() missing required argument: '%s'", sig.name, p.name)
		}
	}
	return nil
}
