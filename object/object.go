package object

import "fmt"

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
	Index(index Object) (Object, error)
	GetAttr(name string) (Object, error)
}

type Args []Object
type KwArgs map[string]Object

func Call(obj Object, args Args, kwargs KwArgs) (Object, error) {
	switch v := obj.(type) {
	case *Function:
		return v.Call(args, kwargs)
	}
	return nil, fmt.Errorf("%s is not callable", obj.Repr())
}
