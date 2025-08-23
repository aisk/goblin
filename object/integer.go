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
	return 0, ErrNotImplmeneted
}

func (i Integer) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) + int64(v)), nil
	default:
		return nil, fmt.Errorf("cannot add Integer and %T", other)
	}
}

func (i Integer) Minus(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) - int64(v)), nil
	default:
		return nil, fmt.Errorf("cannot subtract Integer and %T", other)
	}
}

func (i Integer) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		return Integer(int64(i) * int64(v)), nil
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
	default:
		return nil, fmt.Errorf("cannot divide Integer and %T", other)
	}
}

func (i Integer) Not() (Object, error) {
	return Bool(!i.Bool()), nil
}

var _ Object = Integer(0)
