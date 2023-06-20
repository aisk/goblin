package object

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrNotImplmeneted = errors.New("not implemented")
)

type Object interface {
	Repr() string
	String() string
	Bool() bool
	Compare(other Object) (int, error)
}

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

var _ Object = Integer(0)

type String string

func (s String) Repr() string {
	return fmt.Sprintf("object.String(`%s`)", s.String())
}

func (s String) String() string {
	return string(s)
}

func (s String) Bool() bool {
	if s == "" {
		return false
	}
	return true
}

func (s String) Compare(other Object) (int, error) {
	return 0, ErrNotImplmeneted
}

var _ Object = String("")

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

var _ Bool = Bool(true)

var (
	True  = Bool(true)
	False = Bool(false)
)

type Args []Object
type KwArgs map[string]Object
