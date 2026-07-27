package object

import (
	"strconv"
)

type Integer int64

var _ Object = Integer(0)

func (i Integer) ToBool() (bool, error) { return i != 0, nil }

func (i Integer) TypeName() string { return "Integer" }

func (i Integer) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Integer) ToString() (string, error) { return i.String(), nil }

func (i Integer) Equals(other Object) (bool, error) {
	eq, _ := numericEquals(i, other)
	return eq, nil
}

func (i Integer) Compare(other Object) (int, error) {
	if c, ok := numericCompare(i, other); ok {
		return c, nil
	}
	return 0, NewTypeError("cannot compare Integer and %s", other.TypeName())
}

func (i Integer) Add(other Object) (Object, error) {
	if result, ok := numericAdd(i, other); ok {
		return result, nil
	}
	return nil, NewTypeError("cannot add Integer and %s", other.TypeName())
}

func (i Integer) Minus(other Object) (Object, error) {
	if result, ok := numericMinus(i, other); ok {
		return result, nil
	}
	return nil, NewTypeError("cannot subtract Integer and %s", other.TypeName())
}

func (i Integer) Multiply(other Object) (Object, error) {
	if result, ok := numericMultiply(i, other); ok {
		return result, nil
	}
	return nil, NewTypeError("cannot multiply Integer and %s", other.TypeName())
}

func (i Integer) Divide(other Object) (Object, error) {
	if result, ok, err := numericDivide(i, other); ok {
		return result, err
	}
	return nil, NewTypeError("cannot divide Integer and %s", other.TypeName())
}

// A named int64 type cannot embed NoAssignment and NoReflectedOps, so Integer
// declines assignment and the reflected operators itself.
func (i Integer) SetIndex(Object, Object) (bool, error)  { return false, nil }
func (i Integer) SetAttr(string, Object) (bool, error)   { return false, nil }
func (i Integer) RAdd(Object) (Object, bool, error)      { return nil, false, nil }
func (i Integer) RMinus(Object) (Object, bool, error)    { return nil, false, nil }
func (i Integer) RMultiply(Object) (Object, bool, error) { return nil, false, nil }
func (i Integer) RDivide(Object) (Object, bool, error)   { return nil, false, nil }

func (i Integer) Not() (Object, error) {
	return Bool(i == 0), nil
}

func (i Integer) Iter() ([]Object, error) {
	return nil, NewTypeError("Integer does not support iteration")
}

func (i Integer) Index(index Object) (Object, error) {
	return nil, NewTypeError("Integer is not indexable")
}

func (i Integer) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(i), nil
	case "constructor":
		return IntConstructorFn, nil
	default:
		return nil, NewAttributeError("Integer has no attribute '%s'", name)
	}
}

func (i Integer) Attributes() []string { return []string{"attributes", "constructor"} }

var IntConstructorFn = &Function{Name: "Int", Fn: IntConstructor}

func IntConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Int", args)
	value := ap.AnyOr("value", Integer(0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case Integer:
		return v, nil
	case Float:
		return Integer(int64(v)), nil
	case String:
		n, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return nil, NewValueError("Int() invalid literal for Int: %q", string(v))
		}
		return Integer(n), nil
	case Bool:
		if bool(v) {
			return Integer(1), nil
		}
		return Integer(0), nil
	default:
		return nil, NewTypeError("Int() argument 'value' must be a string or a number, got %s", value.TypeName())
	}
}
