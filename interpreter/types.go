package interpreter

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
)

// goblinType is the runtime representation of a user-defined `type`. Its
// constructor is a callable object.Function; the same pointer is what
// `type_of(instance)` returns, so identity comparisons (`type_of(p) ==
// Point`) work via object.Equals.
type goblinType struct {
	name     string
	fields   []*ast.TypeField
	params   []string
	defaults []object.ParamDefault
	// methods are bound to the module frame, so calling one costs a frame
	// and nothing else.
	methods     map[string]*closure
	attributes  []string
	constructor *object.Function
	// info carries the impls protocol dispatch consults.
	info *object.UserType
}

// defineType binds a user type's constructor in the module's globals. The
// name after each impl is resolved by object.LookupProtocol, never in a
// scope; the REPL does not run the semantic checker, so this is where it
// reports an unknown protocol.
func defineType(tp *typeProto, g *globals, f *frame) error {
	def := tp.def
	methods := make(map[string]*closure, len(def.Methods))
	attributes := make([]string, 0, len(def.Methods)+len(def.Fields))
	seen := make(map[string]bool, cap(attributes))
	for _, field := range def.Fields {
		if !seen[field.Name] {
			attributes = append(attributes, field.Name)
			seen[field.Name] = true
		}
	}
	for _, m := range def.Methods {
		methods[m.Name] = tp.methods[m.Name].bind(f)
		if !seen[m.Name] {
			attributes = append(attributes, m.Name)
			seen[m.Name] = true
		}
	}
	t := &goblinType{
		name:       def.Name,
		fields:     def.Fields,
		params:     make([]string, len(def.Fields)),
		defaults:   make([]object.ParamDefault, len(def.Fields)),
		methods:    methods,
		attributes: attributes,
	}
	for i, field := range def.Fields {
		t.params[i] = field.Name
		if d := tp.fieldDefaults[i]; d != nil {
			t.defaults[i] = func() (object.Object, error) { return d(f) }
		}
	}
	t.info = object.NewUserType(def.Name, t.params)
	for i, block := range def.Impls {
		protocol, err := object.LookupProtocol(def.Name, block.Protocol)
		if err != nil {
			return err
		}
		methods := make([]object.ImplMethod, len(block.Methods))
		for j, m := range block.Methods {
			methods[j] = object.ImplMethod{
				Name:  m.Name,
				Arity: len(m.Parameters),
				Fn:    tp.impls[i][j].function(f),
			}
		}
		t.info.Implement(protocol, methods)
	}
	if err := t.info.Seal(); err != nil {
		return err
	}
	t.constructor = &object.Function{
		Name: def.Name,
		Fn:   t.construct,
	}
	t.info.Constructor = t.constructor
	g.Define(def.Name, t.constructor)
	return nil
}

// construct binds call arguments to fields through the shared
// object.BindArguments, so diagnostics match the transpiled backend exactly.
func (t *goblinType) construct(args object.CallArgs) (object.Object, error) {
	fields, err := object.BindArguments(t.name, t.params, t.defaults, "", "", args)
	if err != nil {
		return nil, err
	}
	return object.NewUser(&instance{typ: t, fields: fields}), nil
}

// fieldIndex locates a declared field by name, returning -1 when the type has
// no such field. Instances store their fields as a slice in declaration order,
// so this is how a name reaches a slot.
func (t *goblinType) fieldIndex(name string) int {
	for i, f := range t.fields {
		if f.Name == name {
			return i
		}
	}
	return -1
}

// instance is a value of a user-defined type. It implements object.Object.
// Fields are stored in declaration order, matching what object.BindArguments
// returns, so construction needs no per-instance map.
type instance struct {
	object.UserBase
	typ    *goblinType
	fields []object.Object
}

var _ object.Object = (*instance)(nil)

// bindMethod returns the method as a callable with the receiver bound as
// `self`. Its prototype carries the Type.method qualified name and binds the
// parameters after self, so traceback frames and binding diagnostics (names
// and argument counts alike) match the transpiled backend.
func (in *instance) bindMethod(m *closure) *object.Function {
	return &object.Function{
		Name: m.proto.fnName,
		Fn: func(args object.CallArgs) (object.Object, error) {
			return m.callSelf(in, args)
		},
	}
}

// CallMethod satisfies object.MethodCaller: `p.step()` runs the method body
// without building the callable that GetAttr hands out. Anything that is not a
// method (a field) is declined, so the caller falls back to GetAttr and
// behavior is unchanged.
func (in *instance) CallMethod(name string, args object.CallArgs) (object.Object, bool, error) {
	m, ok := in.typ.methods[name]
	if !ok {
		return nil, false, nil
	}
	v, err := m.callSelf(in, args)
	return v, true, err
}

func (in *instance) GetAttr(name string) (object.Object, error) {
	if m, ok := in.typ.methods[name]; ok {
		return in.bindMethod(m), nil
	}
	if i := in.typ.fieldIndex(name); i >= 0 {
		return in.fields[i], nil
	}
	return nil, object.NewAttributeError("%s has no attribute '%s'", in.typ.name, name)
}

// TypeName reports the declared name of the user type. Without it diagnostics
// would name the interpreter's internal representation, which the transpiler
// backend has no counterpart for.
func (in *instance) TypeName() string {
	return in.typ.name
}

func (in *instance) Attributes() []string {
	return append([]string(nil), in.typ.attributes...)
}

// SetAttr always handles the assignment: an instance accepts writes to its own
// fields and reports any other name as a missing attribute.
func (in *instance) SetAttr(name string, value object.Object) (bool, error) {
	if i := in.typ.fieldIndex(name); i >= 0 {
		in.fields[i] = value
		return true, nil
	}
	return true, object.NewAttributeError("%s has no attribute '%s'", in.typ.name, name)
}

// UserType and FieldValues satisfy object.UserValue, through which the
// embedded object.UserBase dispatches operators and conversions to the
// type's impls.
func (in *instance) UserType() *object.UserType { return in.typ.info }

func (in *instance) FieldValues() []object.Object { return in.fields }
