package interpreter

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
)

// goblinType is the runtime representation of a user-defined `type`. Its
// constructor is a callable object.Function; the same pointer is what
// `instance.constructor` returns, so identity comparisons (`p.constructor ==
// Point`) work via object.Equals.
type goblinType struct {
	name        string
	fields      []*ast.TypeField
	methods     map[string]*ast.FunctionDefine
	attributes  []string
	constructor *object.Function
	env         *Environment
}

// defineType registers a user type's constructor in the current scope.
func defineType(def *ast.TypeDefine, env *Environment) {
	methods := make(map[string]*ast.FunctionDefine, len(def.Methods))
	attributes := make([]string, 0, len(def.Methods)+len(def.Fields)+2)
	seen := make(map[string]bool, cap(attributes))
	for _, f := range def.Fields {
		if !seen[f.Name] {
			attributes = append(attributes, f.Name)
			seen[f.Name] = true
		}
	}
	for _, m := range def.Methods {
		methods[m.Name] = m
		if !seen[m.Name] {
			attributes = append(attributes, m.Name)
			seen[m.Name] = true
		}
	}
	if !seen["constructor"] {
		attributes = append(attributes, "constructor")
		seen["constructor"] = true
	}
	if !seen["attributes"] {
		attributes = append(attributes, "attributes")
	}
	t := &goblinType{
		name:       def.Name,
		fields:     def.Fields,
		methods:    methods,
		attributes: attributes,
		env:        env,
	}
	t.constructor = &object.Function{
		Name: def.Name,
		Fn:   t.construct,
	}
	env.Define(def.Name, t.constructor)
}

// construct binds call arguments to fields, applying defaults, and returns a
// new instance.
func (t *goblinType) construct(args object.CallArgs) (object.Object, error) {
	if len(args.Positional) > len(t.fields) {
		return nil, object.NewTypeError("%s() takes %d positional arguments, got %d", t.name, len(t.fields), len(args.Positional))
	}

	fields := make(map[string]object.Object, len(t.fields))
	for i, p := range args.Positional {
		fields[t.fields[i].Name] = p
	}

	for key, value := range args.Keyword {
		if !t.hasField(key) {
			return nil, object.NewTypeError("%s() got an unexpected keyword argument '%s'", t.name, key)
		}
		if _, exists := fields[key]; exists {
			return nil, object.NewTypeError("%s() got multiple values for argument '%s'", t.name, key)
		}
		fields[key] = value
	}

	for _, f := range t.fields {
		if _, ok := fields[f.Name]; ok {
			continue
		}
		if f.HasDefault() {
			dv, err := evalExpr(f.DefaultValue, t.env)
			if err != nil {
				return nil, err
			}
			fields[f.Name] = dv
			continue
		}
		return nil, object.NewTypeError("%s() missing required positional argument: '%s'", t.name, f.Name)
	}

	return &instance{typ: t, fields: fields}, nil
}

func (t *goblinType) hasField(name string) bool {
	for _, f := range t.fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// instance is a value of a user-defined type. It implements object.Object.
type instance struct {
	typ    *goblinType
	fields map[string]object.Object
}

var _ object.Object = (*instance)(nil)

// bindMethod returns the method as a callable with the receiver bound as
// `self`. The closure carries the Type.method qualified name and binds the
// parameters after self, so traceback frames and binding diagnostics (names
// and argument counts alike) match the transpiled backend.
func (in *instance) bindMethod(def *ast.FunctionDefine) *object.Function {
	selfEnv := NewEnvironment(in.typ.env)
	selfEnv.Define("self", in)
	fn := makeClosure(in.typ.name+"."+def.Name, def.Position(), def.Parameters[1:], def.Body, selfEnv)
	return &object.Function{
		Name: def.Name,
		Fn:   fn.Fn,
	}
}

// callProto invokes a user-defined protocol method (e.g. "add", "compare",
// "str") with the given arguments if the type defines it. ok reports whether
// the method exists; when false the caller falls back to the default behavior.
func (in *instance) callProto(name string, args ...object.Object) (result object.Object, ok bool, err error) {
	m, defined := in.typ.methods[name]
	if !defined {
		return nil, false, nil
	}
	result, err = in.bindMethod(m).Fn(object.CallArgs{Positional: append(object.Args{}, args...)})
	return result, true, err
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
	if v, ok := in.fields[name]; ok {
		return v, nil
	}
	if name == "attributes" {
		return object.AttributesFunction(in), nil
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
	if in.typ.hasField(name) {
		in.fields[name] = value
		return true, nil
	}
	return true, object.NewAttributeError("%s has no attribute '%s'", in.typ.name, name)
}

// String satisfies fmt.Stringer. It falls back to the default representation
// when __str fails because fmt.Stringer cannot return an error.
func (in *instance) String() string {
	if v, ok, err := in.callProto("__str"); ok && err == nil {
		return fmt.Sprint(v)
	}
	return fmt.Sprintf("<%s@%p>", in.typ.name, in)
}

// ToString performs Goblin's potentially failing __str conversion.
func (in *instance) ToString() (string, error) {
	if v, ok, err := in.callProto("__str"); ok {
		if err != nil {
			return "", err
		}
		return v.ToString()
	}
	return fmt.Sprintf("<%s@%p>", in.typ.name, in), nil
}

func (in *instance) ToBool() (bool, error) {
	if v, ok, err := in.callProto("__bool"); ok {
		if err != nil {
			return false, err
		}
		return v.ToBool()
	}
	return true, nil
}

// Equals dispatches __cmp when defined; without it an instance is equal only
// to itself. A __cmp that fails makes == fail too: a raised error is not the
// same answer as "not equal".
func (in *instance) Equals(other object.Object) (bool, error) {
	c, ok, err := in.compareProto(other)
	if !ok {
		o, isInstance := other.(*instance)
		return isInstance && o == in, nil
	}
	if err != nil {
		return false, err
	}
	return c == 0, nil
}

func (in *instance) Compare(other object.Object) (int, error) {
	c, ok, err := in.compareProto(other)
	if !ok {
		return 0, object.NewTypeError("cannot compare %s", in.typ.name)
	}
	return c, err
}

// compareProto runs __cmp and validates its result. ok reports whether the
// type defines the method at all, which == and the ordering operators answer
// differently.
func (in *instance) compareProto(other object.Object) (int, bool, error) {
	v, ok, err := in.callProto("__cmp", other)
	if !ok {
		return 0, false, nil
	}
	if err != nil {
		return 0, true, err
	}
	i, isInt := v.(object.Integer)
	if !isInt {
		return 0, true, object.NewTypeError("%s.__cmp must return Int, got %s", in.typ.name, fmt.Sprint(v))
	}
	return int(i), true, nil
}

func (in *instance) Add(other object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__add", other); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot add %s", in.typ.name)
}

func (in *instance) Minus(other object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__sub", other); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot subtract %s", in.typ.name)
}

func (in *instance) Multiply(other object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__mul", other); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot multiply %s", in.typ.name)
}

func (in *instance) Divide(other object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__div", other); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot divide %s", in.typ.name)
}

func (in *instance) Modulo(other object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__mod", other); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot modulo %s", in.typ.name)
}

// RAdd, RMinus, RMultiply, RDivide and RModulo are the reflected operators, reached
// when the value stands on the right of an operand that does not know it. They
// report handled == false when the type defines no such method, leaving the
// left operand's error in place.
func (in *instance) RAdd(left object.Object) (object.Object, bool, error) {
	return in.callProto("__radd", left)
}

func (in *instance) RMinus(left object.Object) (object.Object, bool, error) {
	return in.callProto("__rsub", left)
}

func (in *instance) RMultiply(left object.Object) (object.Object, bool, error) {
	return in.callProto("__rmul", left)
}

func (in *instance) RDivide(left object.Object) (object.Object, bool, error) {
	return in.callProto("__rdiv", left)
}

func (in *instance) RModulo(left object.Object) (object.Object, bool, error) {
	return in.callProto("__rmod", left)
}

func (in *instance) Not() (object.Object, error) {
	if v, ok, err := in.callProto("__not"); ok {
		return v, err
	}
	// Without __not, ! negates the instance's truthiness, matching the
	// default behavior of built-in types.
	b, err := in.ToBool()
	if err != nil {
		return nil, err
	}
	return object.Bool(!b), nil
}

func (in *instance) Iter() ([]object.Object, error) {
	if v, ok, err := in.callProto("__iter"); ok {
		if err != nil {
			return nil, err
		}
		return v.Iter()
	}
	return nil, object.NewTypeError("%s does not support iteration", in.typ.name)
}

func (in *instance) Index(index object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__getitem", index); ok {
		return v, err
	}
	return nil, object.NewTypeError("%s is not indexable", in.typ.name)
}

// SetIndex dispatches `obj[i] = v` to a user-defined "__setitem" method. It
// reports handled == false without one, leaving object.SetIndex
// to raise the same "does not support index assignment" error every other type
// gets.
func (in *instance) SetIndex(index object.Object, value object.Object) (bool, error) {
	_, ok, err := in.callProto("__setitem", index, value)
	return ok, err
}
