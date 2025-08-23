package object

import (
	"fmt"
)

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
