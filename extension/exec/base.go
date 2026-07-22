package exec

import "github.com/aisk/goblin/object"

type objectBase struct{ typeName string }

func (b objectBase) Bool() bool                  { return true }
func (b objectBase) ToBool() (bool, error)       { return true, nil }
func (b objectBase) Equals(object.Object) bool   { return false }
func (b objectBase) Not() (object.Object, error) { return object.False, nil }
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
func (b objectBase) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("%s does not support iteration", b.typeName)
}
func (b objectBase) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("%s is not indexable", b.typeName)
}
