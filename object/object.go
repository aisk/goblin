package object

import (
	"errors"
)

var (
	ErrNotImplmeneted = errors.New("not implemented")
)

type Object interface {
	Repr() string
	String() string
	Bool() bool
	Compare(other Object) (int, error)
	Add(other Object) (Object, error)
	Minus(other Object) (Object, error)
	Multiply(other Object) (Object, error)
	Divide(other Object) (Object, error)
	And(other Object) (Object, error)
	Or(other Object) (Object, error)
	Not() (Object, error)
	Iter() ([]Object, error)
}

type Args []Object
type KwArgs map[string]Object
