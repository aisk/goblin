package object

import "fmt"

type Function struct {
	Name string
	Fn   func(CallArgs) (Object, error)
}

func (f *Function) Call(args CallArgs) (Object, error) {
	return f.Fn(args)
}

func (f *Function) String() string { return fmt.Sprintf("<function %s>", f.Name) }

func (f *Function) ToString() (string, error) { return f.String(), nil }
func (f *Function) Bool() bool                { return true }
func (f *Function) ToBool() (bool, error)     { return f.Bool(), nil }
// Compare always fails: functions support equality (identity, handled by
// Equals) but have no ordering.
func (f *Function) Compare(other Object) (int, error) {
	return 0, NewTypeError("cannot compare Function and %T", other)
}
func (f *Function) Add(Object) (Object, error) { return nil, NewTypeError("cannot add Function") }
func (f *Function) Minus(Object) (Object, error) {
	return nil, NewTypeError("cannot subtract Function")
}
func (f *Function) Multiply(Object) (Object, error) {
	return nil, NewTypeError("cannot multiply Function")
}
func (f *Function) Divide(Object) (Object, error) { return nil, NewTypeError("cannot divide Function") }
func (f *Function) Not() (Object, error) { return nil, NewTypeError("cannot perform NOT on Function") }
func (f *Function) Iter() ([]Object, error) {
	return nil, NewTypeError("Function does not support iteration")
}
func (f *Function) Index(Object) (Object, error) {
	return nil, NewTypeError("Function is not indexable")
}
func (f *Function) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(f), nil
	case "constructor":
		return FunctionConstructorFn, nil
	default:
		return nil, NewAttributeError("Function has no attribute '%s'", name)
	}
}

func (f *Function) Attributes() []string { return []string{"attributes", "constructor"} }

var _ Object = (*Function)(nil)

// FunctionConstructorFn is the constructor shared by every callable, including
// the built-in type objects (Int, Str, ...) and user-defined type
// constructors. It is its own constructor, so `Function.constructor` returns
// itself, mirroring how a metaclass is its own type.
var FunctionConstructorFn = &Function{Name: "Function", Fn: FunctionConstructor}

func FunctionConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Function", args)
	f := ap.Func("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return f, nil
}
