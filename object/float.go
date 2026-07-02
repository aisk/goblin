package object

import (
	"fmt"
	"strconv"
)

type Float float64

var _ Object = Float(0)

func (f Float) Bool() bool {
	if f == 0 {
		return false
	}
	return true
}

func (f Float) String() string {
	return strconv.FormatFloat(float64(f), 'f', -1, 64)
}

func (f Float) Compare(other Object) (int, error) {
	switch v := other.(type) {
	case Float:
		a, b := float64(f), float64(v)
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
		return 0, nil
	case Integer:
		a, b := float64(f), float64(v)
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, NewTypeError("cannot compare Float and %T", other)
	}
}

func (f Float) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		return Float(float64(f) + float64(v)), nil
	case Integer:
		return Float(float64(f) + float64(v)), nil
	default:
		return nil, NewTypeError("cannot add Float and %T", other)
	}
}

func (f Float) Minus(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		return Float(float64(f) - float64(v)), nil
	case Integer:
		return Float(float64(f) - float64(v)), nil
	default:
		return nil, NewTypeError("cannot subtract Float and %T", other)
	}
}

func (f Float) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		return Float(float64(f) * float64(v)), nil
	case Integer:
		return Float(float64(f) * float64(v)), nil
	default:
		return nil, NewTypeError("cannot multiply Float and %T", other)
	}
}

func (f Float) Divide(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		if float64(v) == 0 {
			return nil, NewZeroDivisionError("division by zero")
		}
		return Float(float64(f) / float64(v)), nil
	case Integer:
		if int64(v) == 0 {
			return nil, NewZeroDivisionError("division by zero")
		}
		return Float(float64(f) / float64(v)), nil
	default:
		return nil, NewTypeError("cannot divide Float and %T", other)
	}
}

func (f Float) And(other Object) (Object, error) {
	return Bool(f.Bool() && other.Bool()), nil
}

func (f Float) Or(other Object) (Object, error) {
	return Bool(f.Bool() || other.Bool()), nil
}

func (f Float) Not() (Object, error) {
	return Bool(!f.Bool()), nil
}

func (f Float) Iter() ([]Object, error) {
	return nil, NewTypeError("Float does not support iteration")
}

func (f Float) Index(index Object) (Object, error) {
	return nil, NewTypeError("Float is not indexable")
}

func (f Float) GetAttr(name string) (Object, error) {
	switch name {
	case "constructor":
		return FloatConstructorFn, nil
	default:
		return nil, fmt.Errorf("Float has no attribute '%s'", name)
	}
}

var FloatConstructorFn = &Function{Name: "Float", Fn: FloatConstructor}

func FloatConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("Float", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return Float(0), nil
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("Float() takes at most 1 argument, got %d", len(args.Positional))
	}
	switch v := args.Positional[0].(type) {
	case Float:
		return v, nil
	case Integer:
		return Float(float64(int64(v))), nil
	case String:
		n, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return nil, NewValueError("Float() invalid literal for Float: %q", string(v))
		}
		return Float(n), nil
	case Bool:
		if bool(v) {
			return Float(1), nil
		}
		return Float(0), nil
	default:
		return nil, NewTypeError("Float() argument must be a string or a number, not %T", args.Positional[0])
	}
}
