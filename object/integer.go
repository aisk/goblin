package object

import (
	"fmt"
	"strconv"
)

type Integer int64

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

var _ Object = Integer(0)
