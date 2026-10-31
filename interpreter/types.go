package interpreter

import (
	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
)

// goblinType is the runtime representation of a user-defined `type`. Its
// constructor is a callable object.Function; the same pointer is what
// `instance.constructor` returns, so identity comparisons (`p.constructor ==
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
	// info carries the impls trait dispatch consults.
	info *object.UserType
}

// defineType binds a user type's constructor in the module's globals. Each
// impl block resolves its trait now, so the trait must already be defined.
func defineType(tp *typeProto, g *globals, f *frame) error {
	def := tp.def
	methods := make(map[string]*closure, len(def.Methods))
	attributes := make([]string, 0, len(def.Methods)+len(def.Fields)+2)
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
	for _, name := range []string{"constructor", "attributes", "traits"} {
		if !seen[name] {
			attributes = append(attributes, name)
			seen[name] = true
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
		trait, err := resolveTrait(block.Trait, "impl "+block.Trait.String(), g)
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
		t.info.Implement(trait, methods)
	}
	if err := t.info.Seal(); err != nil {
		return err
	}
	t.constructor = &object.Function{
		Name: def.Name,
		Fn:   t.construct,
	}
	g.Define(def.Name, t.constructor)
	return nil
}

// defineTrait binds a trait declaration's trait object. Default methods are
// closures over the module frame taking self as their first argument.
func defineTrait(tp *traitProto, g *globals, f *frame) error {
	def := tp.def
	deps := make([]*object.Trait, len(def.Deps))
	for i, ref := range def.Deps {
		dep, err := resolveTrait(ref, "trait "+def.Name, g)
		if err != nil {
			return err
		}
		deps[i] = dep
	}
	methods := make([]object.TraitMethod, len(def.Methods))
	for i, m := range def.Methods {
		methods[i] = object.TraitMethod{Name: m.Name, Arity: len(m.Parameters), Required: m.Body == nil}
		if p := tp.defaults[i]; p != nil {
			methods[i].Default = p.function(f)
		}
	}
	g.Define(def.Name, object.NewTrait(def.Name, deps, methods))
	return nil
}

// resolveTrait looks up the trait an impl block or dependency list names.
// site says where the reference appears, for the messages.
func resolveTrait(ref *ast.TraitRef, site string, g *globals) (*object.Trait, error) {
	notDefined := object.NewNameError(object.ErrFmtTraitNotDefined, site, ref)
	if ref.Module != "" {
		module, err := g.resolve(ref.Module)
		if err != nil {
			return nil, notDefined
		}
		return object.ModuleTrait(module, ref.Name, site, ref.String())
	}
	value, err := g.resolve(ref.Name)
	if err != nil {
		return nil, notDefined
	}
	return object.AsTrait(value, site, ref.String())
}

// construct binds call arguments to fields through the shared
// object.BindArguments, so diagnostics match the transpiled backend exactly.
func (t *goblinType) construct(args object.CallArgs) (object.Object, error) {
	fields, err := object.BindArguments(t.name, t.params, t.defaults, "", "", args)
	if err != nil {
		return nil, err
	}
	return &instance{typ: t, fields: fields}, nil
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
// method (a field, "constructor", "attributes") is declined, so the caller
// falls back to GetAttr and behavior is unchanged.
func (in *instance) CallMethod(name string, args object.CallArgs) (object.Object, bool, error) {
	m, ok := in.typ.methods[name]
	if !ok {
		return nil, false, nil
	}
	v, err := m.callSelf(in, args)
	return v, true, err
}

func (in *instance) GetAttr(name string) (object.Object, error) {
	// A user-defined method (including one named "constructor") shadows the
	// built-in constructor attribute and any field.
	if m, ok := in.typ.methods[name]; ok {
		return in.bindMethod(m), nil
	}
	if name == "constructor" {
		return in.typ.constructor, nil
	}
	if i := in.typ.fieldIndex(name); i >= 0 {
		return in.fields[i], nil
	}
	if name == "attributes" {
		return object.AttributesFunction(in), nil
	}
	if name == "traits" {
		return object.TraitsFunction(in), nil
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
// object package dispatches operators and conversions to the type's impls.
func (in *instance) UserType() *object.UserType { return in.typ.info }

func (in *instance) FieldValues() []object.Object { return in.fields }

func (in *instance) String() string            { return object.UserString(in) }
func (in *instance) ToString() (string, error) { return object.UserToString(in) }
func (in *instance) ToBool() (bool, error)     { return object.UserToBool(in) }
func (in *instance) Hash() (uint64, error)     { return object.UserHash(in) }

func (in *instance) Equals(other object.Object) (bool, error) {
	return object.UserEquals(in, other)
}

func (in *instance) Compare(other object.Object) (int, error) {
	return object.UserCompare(in, other)
}

func (in *instance) Add(other object.Object) (object.Object, error) {
	return object.UserArith(in, object.ArithAdd, other)
}

func (in *instance) Minus(other object.Object) (object.Object, error) {
	return object.UserArith(in, object.ArithSub, other)
}

func (in *instance) Multiply(other object.Object) (object.Object, error) {
	return object.UserArith(in, object.ArithMul, other)
}

func (in *instance) Divide(other object.Object) (object.Object, error) {
	return object.UserArith(in, object.ArithDiv, other)
}

func (in *instance) Modulo(other object.Object) (object.Object, error) {
	return object.UserArith(in, object.ArithMod, other)
}

func (in *instance) RAdd(left object.Object) (object.Object, bool, error) {
	return object.UserReflected(in, object.ArithAdd, left)
}

func (in *instance) RMinus(left object.Object) (object.Object, bool, error) {
	return object.UserReflected(in, object.ArithSub, left)
}

func (in *instance) RMultiply(left object.Object) (object.Object, bool, error) {
	return object.UserReflected(in, object.ArithMul, left)
}

func (in *instance) RDivide(left object.Object) (object.Object, bool, error) {
	return object.UserReflected(in, object.ArithDiv, left)
}

func (in *instance) RModulo(left object.Object) (object.Object, bool, error) {
	return object.UserReflected(in, object.ArithMod, left)
}

func (in *instance) Iter() ([]object.Object, error) { return object.UserIter(in) }

func (in *instance) Index(index object.Object) (object.Object, error) {
	return object.UserIndex(in, index)
}

func (in *instance) SetIndex(index object.Object, value object.Object) (bool, error) {
	return object.UserSetIndex(in, index, value)
}
