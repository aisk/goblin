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
		return nil, object.NewTypeError("%s() missing required argument: '%s'", t.name, f.Name)
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

// bindMethod returns the method as a callable with the receiver bound as the
// leading `self` argument.
func (in *instance) bindMethod(def *ast.FunctionDefine) *object.Function {
	fn := makeFunction(def, in.typ.env)
	return &object.Function{
		Name: def.Name,
		Fn: func(args object.CallArgs) (object.Object, error) {
			bound := object.CallArgs{
				Positional: append(object.Args{in}, args.Positional...),
				Keyword:    args.Keyword,
			}
			return fn.Fn(bound)
		},
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

func (in *instance) Attributes() []string {
	return append([]string(nil), in.typ.attributes...)
}

func (in *instance) SetAttr(name string, value object.Object) error {
	if in.typ.hasField(name) {
		in.fields[name] = value
		return nil
	}
	return object.NewAttributeError("%s has no attribute '%s'", in.typ.name, name)
}

// String satisfies fmt.Stringer. It falls back to the default representation
// when __str fails because fmt.Stringer cannot return an error.
func (in *instance) String() string {
	if v, ok, err := in.callProto("__str"); ok && err == nil {
		return v.String()
	}
	return fmt.Sprintf("<%s@%p>", in.typ.name, in)
}

// ToString performs Goblin's potentially failing __str conversion.
func (in *instance) ToString() (string, error) {
	if v, ok, err := in.callProto("__str"); ok {
		if err != nil {
			return "", err
		}
		return v.String(), nil
	}
	return fmt.Sprintf("<%s@%p>", in.typ.name, in), nil
}

// Bool returns an infallible truth value, falling back to true when __bool fails.
func (in *instance) Bool() bool {
	if v, ok, err := in.callProto("__bool"); ok && err == nil {
		return v.Bool()
	}
	return true
}

func (in *instance) ToBool() (bool, error) {
	if v, ok, err := in.callProto("__bool"); ok {
		if err != nil {
			return false, err
		}
		return v.Bool(), nil
	}
	return true, nil
}

func (in *instance) Compare(other object.Object) (int, error) {
	if v, ok, err := in.callProto("__cmp", other); ok {
		if err != nil {
			return 0, err
		}
		i, isInt := v.(object.Integer)
		if !isInt {
			return 0, object.NewTypeError("%s.__cmp must return Int, got %s", in.typ.name, v.String())
		}
		return int(i), nil
	}
	return 0, object.NewTypeError("cannot compare %s", in.typ.name)
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

func (in *instance) And(other object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__and", other); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot perform AND on %s", in.typ.name)
}

func (in *instance) Or(other object.Object) (object.Object, error) {
	if v, ok, err := in.callProto("__or", other); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot perform OR on %s", in.typ.name)
}

func (in *instance) Not() (object.Object, error) {
	if v, ok, err := in.callProto("__not"); ok {
		return v, err
	}
	return nil, object.NewTypeError("cannot perform NOT on %s", in.typ.name)
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

// SetIndex implements object.IndexSetter, dispatching `obj[i] = v` to a
// user-defined "__setitem" method.
func (in *instance) SetIndex(index object.Object, value object.Object) error {
	if _, ok, err := in.callProto("__setitem", index, value); ok {
		return err
	}
	return object.NewTypeError("%s does not support index assignment", in.typ.name)
}
