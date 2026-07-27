package regexp

import "github.com/aisk/goblin/object"

// objectBase provides the boilerplate Object methods shared by Pattern and
// Match. Both are opaque values: always truthy, and supporting neither
// arithmetic, ordering, iteration, nor indexing.
type objectBase struct {
	object.NoReflectedOps
	object.NoAssignment
	typeName string
}

func (b objectBase) TypeName() string { return b.typeName }

func (b objectBase) ToBool() (bool, error)       { return true, nil }
func (b objectBase) Not() (object.Object, error) { return object.False, nil }

// Equals returns false so that object.Equals falls back to its identity
// backstop. Pattern overrides this with value equality.
func (b objectBase) Equals(object.Object) (bool, error) { return false, nil }

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
