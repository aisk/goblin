package interpreter

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
)

// goblinType is the runtime representation of a user-defined `type`. Its
// constructor is a callable object.Function; the same pointer is what
// `instance.constructor` returns, so identity comparisons (`p.constructor ==
// Point`) work via Function.Compare.
type goblinType struct {
	name        string
	fields      []*ast.TypeField
	methods     map[string]*ast.FunctionDefine
	constructor *object.Function
	env         *Environment
}

// defineType registers a user type's constructor in the current scope.
func defineType(def *ast.TypeDefine, env *Environment) {
	methods := make(map[string]*ast.FunctionDefine, len(def.Methods))
	for _, m := range def.Methods {
		methods[m.Name] = m
	}
	t := &goblinType{
		name:    def.Name,
		fields:  def.Fields,
		methods: methods,
		env:     env,
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
	return nil, object.NewTypeError("%s has no attribute '%s'", in.typ.name, name)
}

func (in *instance) SetAttr(name string, value object.Object) error {
	if in.typ.hasField(name) {
		in.fields[name] = value
		return nil
	}
	return object.NewTypeError("%s has no attribute '%s'", in.typ.name, name)
}

func (in *instance) String() string { return fmt.Sprintf("<%s@%p>", in.typ.name, in) }
func (in *instance) Bool() bool     { return true }

func (in *instance) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("cannot compare %s", in.typ.name)
}
func (in *instance) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add %s", in.typ.name)
}
func (in *instance) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract %s", in.typ.name)
}
func (in *instance) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply %s", in.typ.name)
}
func (in *instance) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot divide %s", in.typ.name)
}
func (in *instance) And(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot perform AND on %s", in.typ.name)
}
func (in *instance) Or(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot perform OR on %s", in.typ.name)
}
func (in *instance) Not() (object.Object, error) {
	return nil, object.NewTypeError("cannot perform NOT on %s", in.typ.name)
}
func (in *instance) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("%s does not support iteration", in.typ.name)
}
func (in *instance) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("%s is not indexable", in.typ.name)
}
