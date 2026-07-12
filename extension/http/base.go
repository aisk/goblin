package http

import (
	"github.com/aisk/goblin/object"
)

// objectBase provides the boilerplate Object methods shared by the http
// module's value types (Response, Client, Request, Header). These types are
// opaque handles: they are always truthy and support neither arithmetic,
// comparison, iteration, nor indexing. Concrete types embed this base and
// implement only String() and GetAttr().
type objectBase struct {
	typeName string
}

func (b objectBase) Bool() bool { return true }

func (b objectBase) ToBool() (bool, error) { return b.Bool(), nil }

func (b objectBase) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("cannot compare %s", b.typeName)
}
func (b objectBase) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add %s", b.typeName)
}
func (b objectBase) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract %s", b.typeName)
}
func (b objectBase) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply %s", b.typeName)
}
func (b objectBase) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot divide %s", b.typeName)
}
func (b objectBase) And(other object.Object) (object.Object, error) {
	return object.Bool(other.Bool()), nil
}
func (b objectBase) Or(object.Object) (object.Object, error) {
	return object.Bool(true), nil
}
func (b objectBase) Not() (object.Object, error) {
	return object.Bool(false), nil
}
func (b objectBase) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("%s does not support iteration", b.typeName)
}
func (b objectBase) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("%s is not indexable", b.typeName)
}
