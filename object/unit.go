package object

import (
	"fmt"
)

var Nil Object = Unit{}

type Unit struct{}

func (n Unit) Repr() string {
	return "object.None"
}

func (n Unit) String() string {
	return "none"
}

func (n Unit) Bool() bool {
	return false
}

func (n Unit) Compare(other Object) (int, error) {
	return 0, ErrNotImplmeneted
}

func (n Unit) Add(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot add to Nil")
}

func (n Unit) Minus(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot subtract from Nil")
}

func (n Unit) Multiply(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot multiply Nil")
}

func (n Unit) Divide(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot divide Nil")
}

func (n Unit) And(other Object) (Object, error) {
	return Bool(n.Bool() && other.Bool()), nil
}

func (n Unit) Or(other Object) (Object, error) {
	return Bool(n.Bool() || other.Bool()), nil
}

func (n Unit) Not() (Object, error) {
	return Bool(!n.Bool()), nil
}

func (n Unit) Iter() ([]Object, error) {
	return nil, fmt.Errorf("Nil does not support iteration")
}

var _ Object = Unit{}
