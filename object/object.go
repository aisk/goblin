package object

type Object interface {
	String() string
	Bool() bool
	Compare(other Object) (int, error)
	Add(other Object) (Object, error)
	Minus(other Object) (Object, error)
	Multiply(other Object) (Object, error)
	Divide(other Object) (Object, error)
	And(other Object) (Object, error)
	Or(other Object) (Object, error)
	Not() (Object, error)
	Iter() ([]Object, error)
	Index(index Object) (Object, error)
	GetAttr(name string) (Object, error)
}

func Call(obj Object, args CallArgs) (Object, error) {
	switch v := obj.(type) {
	case *Function:
		return v.Call(args)
	}
	return nil, NewTypeError("%s is not callable", obj.String())
}

// IndexSetter is implemented by objects that support index assignment,
// e.g. `list[0] = x` or `dict["k"] = v`.
type IndexSetter interface {
	SetIndex(index Object, value Object) error
}

// Represented is implemented by objects whose string conversion may fail, e.g.
// a user type whose `__str` method raises. It provides an error-propagating
// alternative to the infallible Stringer String() method.
type Represented interface {
	Repr() (string, error)
}

// Truthful is implemented by objects whose truthiness test may fail, e.g. a
// user type whose `__bool` method raises. It provides an error-propagating
// alternative to the infallible Bool() method.
type Truthful interface {
	Truthy() (bool, error)
}

// Repr returns an object's string representation, propagating any error from a
// user-defined __str method. Objects that cannot fail fall back to String().
func Repr(obj Object) (string, error) {
	if r, ok := obj.(Represented); ok {
		return r.Repr()
	}
	return obj.String(), nil
}

// Truthy returns an object's truth value, propagating any error from a
// user-defined __bool method. Objects that cannot fail fall back to Bool().
func Truthy(obj Object) (bool, error) {
	if t, ok := obj.(Truthful); ok {
		return t.Truthy()
	}
	return obj.Bool(), nil
}

// AttrSetter is implemented by objects that support member assignment,
// e.g. `obj.field = x`.
type AttrSetter interface {
	SetAttr(name string, value Object) error
}

// SetItem performs an index assignment, dispatching to the object's SetIndex
// method when available.
func SetItem(obj Object, index Object, value Object) error {
	if s, ok := obj.(IndexSetter); ok {
		return s.SetIndex(index, value)
	}
	return NewTypeError("%s does not support index assignment", obj.String())
}

// SetAttribute performs a member assignment, dispatching to the object's
// SetAttr method when available.
func SetAttribute(obj Object, name string, value Object) error {
	if s, ok := obj.(AttrSetter); ok {
		return s.SetAttr(name, value)
	}
	return NewTypeError("%s does not support attribute assignment", obj.String())
}
