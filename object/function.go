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

// GetAttr exposes the read-only property name: "Int", "Point",
// "Point.area".
func (f *Function) GetAttr(name string) (Object, error) {
	if name == "name" {
		return String(f.Name), nil
	}
	return nil, NewAttributeError("Function has no attribute '%s'", name)
}

func (f *Function) Attributes() []string { return []string{"name"} }

var _ Object = (*Function)(nil)

// FunctionConstructorFn is the constructor shared by every callable, including
// the built-in type objects (Int, String, ...) and user-defined type
// constructors. It is its own type, so `type_of(Function) == Function`,
// mirroring how a metaclass is its own type.
var FunctionConstructorFn = &Function{Name: "Function", Fn: FunctionConstructor}

// FunctionConstructor is Function(value): it returns a callable value
// unchanged, an error kind included, and raises TypeError for anything else.
func FunctionConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Function", args)
	v := ap.Any("value")
	if v != nil && !Callable(v) {
		ap.typeErr("value", "callable", v)
	}
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return v, nil
}

// Callable reports whether obj can be called, i.e. whether Call accepts it:
// a function, a constructor, a bound method, or an error kind.
func Callable(obj Object) bool {
	switch obj.(type) {
	case *Function, *Error:
		return true
	}
	return false
}

// AsFunction returns a callable value as a *Function, for Go code that holds
// on to something to call: a *Function as it is, and an error, whose call
// makes an error of that kind, wrapped in one. ok is false for a value that
// is not callable. Every check for "a function here" goes through it, so
// whatever Call accepts is accepted everywhere a function is expected.
func AsFunction(obj Object) (fn *Function, ok bool) {
	switch v := obj.(type) {
	case *Function:
		return v, true
	case *Error:
		return &Function{Name: v.origin().Value, Fn: v.New}, true
	}
	return nil, false
}

type functionName struct{}

func (functionName) typeName() string { return "Function" }
