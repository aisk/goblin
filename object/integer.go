package object

import (
	"fmt"
	"strconv"
)

type Integer int64

var _ Object = Integer(0)

func (i Integer) Repr() string {
	return fmt.Sprintf("object.Integer(%s)", i.String())
}

func (i Integer) Bool() bool {
	if i == 0 {
		return false
	}
	return true
}

func (i Integer) String() string {
	return strconv.FormatInt(int64(i), 10)
}

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
		return 0, fmt.Errorf("cannot compare Integer and %T", other)
	}
}

func (i Integer) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) + int64(v)), nil
	case Float:
		return Float(float64(i) + float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot add Integer and %T", other)
	}
}

func (i Integer) Minus(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) - int64(v)), nil
	case Float:
		return Float(float64(i) - float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot subtract Integer and %T", other)
	}
}

func (i Integer) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) * int64(v)), nil
	case Float:
		return Float(float64(i) * float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot multiply Integer and %T", other)
	}
}

func (i Integer) Divide(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		if int64(v) == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return Integer(int64(i) / int64(v)), nil
	case Float:
		if float64(v) == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return Float(float64(i) / float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot divide Integer and %T", other)
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
	return nil, fmt.Errorf("Integer does not support iteration")
}

func (i Integer) Index(index Object) (Object, error) {
	return nil, fmt.Errorf("Integer is not indexable")
}

func (i Integer) GetAttr(name string) (Object, error) {
	return nil, fmt.Errorf("Integer has no attribute '%s'", name)
}

func IntConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("Int", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return Integer(0), nil
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("Int() takes at most 1 argument, got %d", len(args.Positional))
	}
	switch v := args.Positional[0].(type) {
	case Integer:
		return v, nil
	case Float:
		return Integer(int64(v)), nil
	case String:
		n, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Int() invalid literal for Int: %q", string(v))
		}
		return Integer(n), nil
	case Bool:
		if bool(v) {
			return Integer(1), nil
		}
		return Integer(0), nil
	default:
		return nil, fmt.Errorf("Int() argument must be a string or a number, not %T", args.Positional[0])
	}
}
