package regexp

import "github.com/aisk/goblin/object"

type objectBase struct{}

func (objectBase) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("regexp value does not support addition")
}
func (objectBase) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("regexp value does not support subtraction")
}
func (objectBase) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("regexp value does not support multiplication")
}
func (objectBase) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("regexp value does not support division")
}
func (objectBase) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("regexp values are not ordered")
}
func (objectBase) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("regexp value does not support iteration")
}
func (objectBase) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("regexp value is not indexable")
}
