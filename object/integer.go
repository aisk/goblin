package object

import (
	"strconv"
)

type Integer int64

var _ Object = Integer(0)

func (i Integer) Bool() bool {
	if i == 0 {
		return false
	}
	return true
}

func (i Integer) ToBool() (bool, error) { return i.Bool(), nil }

func (i Integer) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Integer) ToString() (string, error) { return i.String(), nil }

func (i Integer) Compare(other Object) (int, error) {
	switch v := other.(type) {
	case Integer:
		a, b := int64(i), int64(v)
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
		return 0, nil
	case Float:
		a, b := float64(i), float64(v)
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, NewTypeError("cannot compare Integer and %T", other)
	}
}

func (i Integer) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) + int64(v)), nil
	case Float:
		return Float(float64(i) + float64(v)), nil
	default:
		return nil, NewTypeError("cannot add Integer and %T", other)
	}
}

func (i Integer) Minus(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) - int64(v)), nil
	case Float:
		return Float(float64(i) - float64(v)), nil
	default:
		return nil, NewTypeError("cannot subtract Integer and %T", other)
	}
}

func (i Integer) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) * int64(v)), nil
	case Float:
		return Float(float64(i) * float64(v)), nil
	default:
		return nil, NewTypeError("cannot multiply Integer and %T", other)
	}
}

func (i Integer) Divide(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		if int64(v) == 0 {
			return nil, NewZeroDivisionError("division by zero")
		}
		return Integer(int64(i) / int64(v)), nil
	case Float:
		if float64(v) == 0 {
			return nil, NewZeroDivisionError("division by zero")
		}
		return Float(float64(i) / float64(v)), nil
	default:
		return nil, NewTypeError("cannot divide Integer and %T", other)
	}
}

func (i Integer) And(other Object) (Object, error) {
	return Bool(i.Bool() && other.Bool()), nil
}

func (i Integer) Or(other Object) (Object, error) {
	return Bool(i.Bool() || other.Bool()), nil
}

func (i Integer) Not() (Object, error) {
	return Bool(!i.Bool()), nil
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
		return nil, NewTypeError("Int() argument 'value' must be a string or a number, got %T", value)
	}
}
