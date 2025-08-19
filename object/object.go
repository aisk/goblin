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
	Add(other Object) (Object, error)
	Minus(other Object) (Object, error)
	Multiply(other Object) (Object, error)
	Divide(other Object) (Object, error)
	Not() (Object, error)
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

func (s String) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case String:
		return String(string(s) + string(v)), nil
	case Integer:
		return String(string(s) + v.String()), nil
	case Bool:
		return String(string(s) + v.String()), nil
	default:
		return nil, fmt.Errorf("cannot add String and %T", other)
	}
}

func (s String) Minus(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot subtract from String")
}

func (s String) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		result := ""
		for i := int64(0); i < int64(v); i++ {
			result += string(s)
		}
		return String(result), nil
	default:
		return nil, fmt.Errorf("cannot multiply String and %T", other)
	}
}

func (s String) Divide(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot divide String")
}

func (s String) Not() (Object, error) {
	return Bool(!s.Bool()), nil
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

func (b Bool) Not() (Object, error) {
	return Bool(!b.Bool()), nil
}

var _ Object = Bool(true)

var (
	True  = Bool(true)
	False = Bool(false)
)

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

func (n Unit) Not() (Object, error) {
	return Bool(!n.Bool()), nil
}

var Nil Object = Unit{}

type Args []Object
type KwArgs map[string]Object
