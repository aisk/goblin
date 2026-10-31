package object

import "fmt"

type Function struct {
	inert[functionName]
	Name string
	Fn   func(CallArgs) (Object, error)
}

func (f *Function) Call(args CallArgs) (Object, error) {
	return f.Fn(args)
}

func (f *Function) TypeName() string { return "Function" }

func (f *Function) String() string { return fmt.Sprintf("<function %s>", f.Name) }

func (f *Function) ToString() (string, error) { return f.String(), nil }
func (f *Function) ToBool() (bool, error)     { return true, nil }

// Equals is identity: two functions are equal only when they are the same
// function value.
func (f *Function) Equals(other Object) (bool, error) {
	v, ok := other.(*Function)
	return ok && f == v, nil
}

// Hash follows identity, so functions can be dict keys.
func (f *Function) Hash() (uint64, error) { return identityHash(f), nil }

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

type functionName struct{}

func (functionName) typeName() string { return "Function" }
