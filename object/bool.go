package object

import (
	"fmt"
)

var (
	True  = Bool(true)
	False = Bool(false)
)

type Bool bool

func (b Bool) Repr() string {
	return fmt.Sprintf("object.Bool(%s)", b.String())
}

func (b Bool) String() string {
	switch b {
	case true:
		return "true"
	case false:
		return "false"
	}
	panic("never happen")
}

func (b Bool) Bool() bool {
	return bool(b)
}

func (b Bool) Compare(other Object) (int, error) {
	return 0, ErrNotImplmeneted
}

func (b Bool) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case String:
		return String(b.String() + string(v)), nil
	default:
		return nil, fmt.Errorf("cannot add Bool and %T", other)
	}
}

func (b Bool) Minus(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot subtract from Bool")
}

func (b Bool) Multiply(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot multiply Bool")
}

func (b Bool) Divide(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot divide Bool")
}

func (b Bool) And(other Object) (Object, error) {
	return Bool(b.Bool() && other.Bool()), nil
}

func (b Bool) Or(other Object) (Object, error) {
	return Bool(b.Bool() || other.Bool()), nil
}

func (b Bool) Not() (Object, error) {
	return Bool(!b.Bool()), nil
}

func (b Bool) Iter() ([]Object, error) {
	return nil, fmt.Errorf("Bool does not support iteration")
}

var _ Object = Bool(true)
