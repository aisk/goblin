package object

type Object interface {
	// String returns an infallible representation for diagnostics, formatting,
	// and code paths that must always be able to produce text.
	String() string
	// ToString performs Goblin's string conversion protocol. It may invoke a
	// user-defined __str method and propagate its error.
	ToString() (string, error)
	Bool() bool
	ToBool() (bool, error)
	Compare(other Object) (int, error)
	Add(other Object) (Object, error)
	Minus(other Object) (Object, error)
	Multiply(other Object) (Object, error)
	Divide(other Object) (Object, error)
	Not() (Object, error)
	Iter() ([]Object, error)
	Index(index Object) (Object, error)
	GetAttr(name string) (Object, error)
	Attributes() []string
}

// literal returns an object's representation inside a collection literal.
// Strings need quoting; other objects already provide an appropriate String
// representation, including nested collections.
func literal(obj Object) string {
	if s, ok := obj.(String); ok {
		return s.Literal()
	}
	return obj.String()
}

// AttributesFunction exposes an object's attribute names as the bound
// attributes() method. A fresh List is returned on every call so callers
// cannot mutate shared runtime metadata.
func AttributesFunction(obj Object) *Function {
	return &Function{Name: "attributes", Fn: func(args CallArgs) (Object, error) {
		if err := RequireNoKeyword("attributes", args); err != nil {
			return nil, err
		}
		if len(args.Positional) != 0 {
			return nil, NewTypeError("attributes() takes exactly 0 arguments, got %d", len(args.Positional))
		}
		names := obj.Attributes()
		elements := make([]Object, len(names))
		for i, name := range names {
			elements[i] = String(name)
		}
		return &List{Elements: elements}, nil
	}}
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
