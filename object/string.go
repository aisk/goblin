package object

import (
	"fmt"
	"strings"
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
	switch v := other.(type) {
	case String:
		a, b := string(s), string(v)
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot compare String and %T", other)
	}
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

func (s String) And(other Object) (Object, error) {
	return Bool(s.Bool() && other.Bool()), nil
}

func (s String) Or(other Object) (Object, error) {
	return Bool(s.Bool() || other.Bool()), nil
}

func (s String) Not() (Object, error) {
	return Bool(!s.Bool()), nil
}

func (s String) Iter() ([]Object, error) {
	// String can be iterated character by character
	var result []Object
	for _, char := range string(s) {
		result = append(result, String(string(char)))
	}
	return result, nil
}

func (s String) Index(index Object) (Object, error) {
	return nil, fmt.Errorf("String is not indexable")
}

func (s String) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return Integer(len([]rune(string(s)))), nil
	case "upper":
		return &Method{Fn: func(args Args, kwargs KwArgs) (Object, error) {
			return String(strings.ToUpper(string(s))), nil
		}}, nil
	case "lower":
		return &Method{Fn: func(args Args, kwargs KwArgs) (Object, error) {
			return String(strings.ToLower(string(s))), nil
		}}, nil
	default:
		return nil, fmt.Errorf("String has no attribute '%s'", name)
	}
}

var _ Object = String("")
