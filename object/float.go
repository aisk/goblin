package object

import (
	"fmt"
	"strconv"
)

type Float float64

func (f Float) Repr() string {
	return fmt.Sprintf("object.Float(%s)", f.String())
}

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
		return 0, fmt.Errorf("cannot compare Float and %T", other)
	}
}

func (f Float) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		return Float(float64(f) + float64(v)), nil
	case Integer:
		return Float(float64(f) + float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot add Float and %T", other)
	}
}

func (f Float) Minus(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		return Float(float64(f) - float64(v)), nil
	case Integer:
		return Float(float64(f) - float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot subtract Float and %T", other)
	}
}

func (f Float) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		return Float(float64(f) * float64(v)), nil
	case Integer:
		return Float(float64(f) * float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot multiply Float and %T", other)
	}
}

func (f Float) Divide(other Object) (Object, error) {
	switch v := other.(type) {
	case Float:
		if float64(v) == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return Float(float64(f) / float64(v)), nil
	case Integer:
		if int64(v) == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return Float(float64(f) / float64(v)), nil
	default:
		return nil, fmt.Errorf("cannot divide Float and %T", other)
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
	return nil, fmt.Errorf("Float does not support iteration")
}

var _ Object = Float(0)