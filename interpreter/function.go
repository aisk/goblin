package interpreter

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// funcProto is everything about a function that does not depend on the frame
// it closes over: its compiled body, frame shape and parameter list. Each
// evaluation of the definition binds it to a frame (see closure).
type funcProto struct {
	// name is used for repr-free diagnostics: binding errors and traceback
	// frames. fnName is the name of the object.Function handed out, which
	// for a method is the bare method name.
	name   string
	fnName string
	pos    token.Pos
	module string
	frame  object.Frame

	// self reserves slot 0 for a method receiver, which is bound apart from
	// the arguments.
	self     bool
	fixed    []string
	defaults []exprFn
	varArgs  string
	kwArgs   string
	// plain reports a parameter list without *args or **kwargs, the shape
	// whose all-positional calls skip object.BindArguments.
	plain bool

	nvars  int
	ntemps int
	body   stmtFn
}

// function compiles a function body. Parameter defaults are compiled in the
// current (defining) scope, where they are evaluated on every call; the body
// gets a frame of its own. With self set, params[0] is the receiver and the
// remaining parameters are the ones calls bind.
func (c *compiler) function(name, fnName string, pos token.Pos, params []*ast.Parameter, body []ast.Statement, self bool) *funcProto {
	p := &funcProto{name: name, fnName: fnName, pos: pos, self: self}
	var selfName string
	if self {
		selfName, params = params[0].Name, params[1:]
	}

	var fixedDefaults []ast.Expression
	anyDefault := false
	for _, param := range params {
		switch {
		case param.VarArgs:
			p.varArgs = param.Name
		case param.KwArgs:
			p.kwArgs = param.Name
		default:
			p.fixed = append(p.fixed, param.Name)
			fixedDefaults = append(fixedDefaults, param.Default)
			if param.HasDefault() {
				anyDefault = true
			}
		}
	}
	if anyDefault {
		p.defaults = make([]exprFn, len(fixedDefaults))
		for i, d := range fixedDefaults {
			if d != nil {
				p.defaults[i] = c.outOfBand(d)
			}
		}
	}
	p.plain = p.varArgs == "" && p.kwArgs == ""

	saved, savedNoTemps := c.scope, c.noTemps
	fs := &frameScope{parent: saved.fs}
	c.scope = &scope{parent: saved, fs: fs, names: make(map[string]int)}
	c.noTemps = false
	// Slots follow object.BindArguments' result order: fixed parameters,
	// then *args, then **kwargs, after the receiver.
	bindParam := func(name string) {
		c.scope.names[name] = fs.slot()
	}
	if self {
		bindParam(selfName)
	}
	for _, name := range p.fixed {
		bindParam(name)
	}
	if p.varArgs != "" {
		bindParam(p.varArgs)
	}
	if p.kwArgs != "" {
		bindParam(p.kwArgs)
	}
	p.body = c.statements(body)
	c.scope, c.noTemps = saved, savedNoTemps

	p.nvars, p.ntemps = fs.nvars, fs.maxTemp
	if src, ok := pos.Context.(token.Sourcer); ok && src != nil {
		p.module = moduleName(src.Source())
	}
	p.frame = stackFrame(p.module, name, pos)
	return p
}

// closure is a funcProto bound to the frame it was defined in.
type closure struct {
	proto  *funcProto
	parent *frame
	// defaults evaluate in the defining frame on every call that needs
	// them; built once per closure since that frame never changes.
	defaults []object.ParamDefault
}

func (p *funcProto) bind(parent *frame) *closure {
	cl := &closure{proto: p, parent: parent}
	if p.defaults != nil {
		cl.defaults = make([]object.ParamDefault, len(p.defaults))
		for i, d := range p.defaults {
			if d == nil {
				continue
			}
			d := d
			cl.defaults[i] = func() (object.Object, error) { return d(parent) }
		}
	}
	return cl
}

// function binds the prototype to parent and wraps it as a callable.
func (p *funcProto) function(parent *frame) *object.Function {
	cl := p.bind(parent)
	return &object.Function{Name: p.fnName, Fn: cl.call}
}

func (cl *closure) call(args object.CallArgs) (object.Object, error) {
	return cl.proto.invoke(cl.parent, cl.defaults, nil, args)
}

// callSelf runs a method with the receiver bound as self.
func (cl *closure) callSelf(self object.Object, args object.CallArgs) (object.Object, error) {
	return cl.proto.invoke(cl.parent, cl.defaults, self, args)
}

// invoke runs the body in a fresh frame whose parent is the defining frame.
func (p *funcProto) invoke(parent *frame, defaults []object.ParamDefault, self object.Object, args object.CallArgs) (object.Object, error) {
	f := newFrame(parent, p.nvars, p.ntemps)
	slots := f.slots
	if p.self {
		slots[0] = self
		slots = slots[1:]
	}
	if p.plain && len(args.Keyword) == 0 && len(args.Positional) == len(p.fixed) {
		copy(slots, args.Positional)
	} else {
		bound, err := object.BindArguments(p.name, p.fixed, defaults, p.varArgs, p.kwArgs, args)
		if err != nil {
			return nil, object.WithFrame(err, p.frame)
		}
		copy(slots, bound)
	}

	ct, v, err := p.body(f)
	if err == nil && ct != ctrlNone && ct != ctrlReturn {
		err = strayCtrl(ct)
	}
	if err != nil {
		// The frame points at the failing statement inside the function;
		// the definition position is only a fallback.
		inner, errPos := takePosition(err, p.pos)
		return nil, object.WithFrame(inner, stackFrame(p.module, p.name, errPos))
	}
	if ct == ctrlReturn && v != nil {
		return v, nil
	}
	return object.Nil, nil
}

// typeProto is a compiled user type: field defaults, methods and impl
// methods, bound to the module frame when the module loads.
type typeProto struct {
	def           *ast.TypeDefine
	fieldDefaults []exprFn
	methods       map[string]*funcProto
	impls         [][]*funcProto
}

func (c *compiler) typeDef(def *ast.TypeDefine) *typeProto {
	tp := &typeProto{
		def:           def,
		fieldDefaults: make([]exprFn, len(def.Fields)),
		methods:       make(map[string]*funcProto, len(def.Methods)),
		impls:         make([][]*funcProto, len(def.Impls)),
	}
	for i, f := range def.Fields {
		if f.HasDefault() {
			tp.fieldDefaults[i] = c.outOfBand(f.DefaultValue)
		}
	}
	for _, m := range def.Methods {
		tp.methods[m.Name] = c.function(def.Name+"."+m.Name, m.Name, m.Position(), m.Parameters, m.Body, true)
	}
	for i, block := range def.Impls {
		tp.impls[i] = make([]*funcProto, len(block.Methods))
		for j, m := range block.Methods {
			tp.impls[i][j] = c.function(def.Name+"."+m.Name, m.Name, m.Position(), m.Parameters, m.Body, false)
		}
	}
	return tp
}

// traitProto is a compiled trait declaration; defaults holds a prototype for
// each method with a body and nil for a required one.
type traitProto struct {
	def      *ast.TraitDefine
	defaults []*funcProto
}

func (c *compiler) traitDef(def *ast.TraitDefine) *traitProto {
	tp := &traitProto{def: def, defaults: make([]*funcProto, len(def.Methods))}
	for i, m := range def.Methods {
		if m.Body != nil {
			tp.defaults[i] = c.function(def.Name+"."+m.Name, m.Name, m.Position(), m.Parameters, m.Body, false)
		}
	}
	return tp
}

// load resolves imports and hoists function, type and trait definitions of a
// compiled module body into its globals, so references resolve regardless of
// source order. Types and traits are bound here once, in source order, so a
// type's impls resolve the traits declared above it. argv is the script
// command line closed over by import "os".
func (c *compiler) load(mod *ast.Module, f *frame, baseDir string, reg *object.Registry, argv []string) error {
	for _, stmt := range mod.Body {
		if imp, ok := stmt.(*ast.Import); ok {
			m, err := resolveImport(imp, baseDir, reg, argv)
			if err != nil {
				return err
			}
			c.g.Define(imp.Name, m)
		}
	}
	for _, stmt := range mod.Body {
		switch s := stmt.(type) {
		case *ast.FunctionDefine:
			c.g.Define(s.Name, c.funcs[s].function(f))
		case *ast.TypeDefine:
			if err := defineType(c.types[s], c.g, f); err != nil {
				return positionError(err, s.Position())
			}
		case *ast.TraitDefine:
			if err := defineTrait(c.traits[s], c.g, f); err != nil {
				return positionError(err, s.Position())
			}
		}
	}
	return nil
}
