package object

import "fmt"

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
	return nil, fmt.Errorf("%s is not callable", obj.String())
}

// IndexSetter is implemented by objects that support index assignment,
// e.g. `list[0] = x` or `dict["k"] = v`.
type IndexSetter interface {
	SetIndex(index Object, value Object) error
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
	return fmt.Errorf("%s does not support index assignment", obj.String())
}

// SetAttribute performs a member assignment, dispatching to the object's
// SetAttr method when available.
func SetAttribute(obj Object, name string, value Object) error {
	if s, ok := obj.(AttrSetter); ok {
		return s.SetAttr(name, value)
	}
	return fmt.Errorf("%s does not support attribute assignment", obj.String())
}
