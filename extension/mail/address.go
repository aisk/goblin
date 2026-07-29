package mail

import (
	stdmail "net/mail"
	"strings"

	"github.com/aisk/goblin/object"
)

type Address struct {
	object.NoReflectedOps
	object.NoAssignment
	value *stdmail.Address
}

func NewAddress(value *stdmail.Address) *Address { return &Address{value: value} }

func (a *Address) TypeName() string          { return "Address" }
func (a *Address) String() string            { return a.value.String() }
func (a *Address) ToString() (string, error) { return a.String(), nil }
func (a *Address) ToBool() (bool, error)     { return a.value.Address != "", nil }
func (a *Address) Not() (object.Object, error) {
	return object.Bool(a.value.Address == ""), nil
}
func (a *Address) Equals(other object.Object) (bool, error) {
	value, ok := other.(*Address)
	return ok && a.value.Name == value.value.Name && a.value.Address == value.value.Address, nil
}
func (a *Address) Compare(other object.Object) (int, error) {
	value, ok := other.(*Address)
	if !ok {
		return 0, object.NewTypeError("cannot compare Address with %s", other.TypeName())
	}
	return strings.Compare(a.String(), value.String()), nil
}
func (a *Address) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Address does not support addition")
}
func (a *Address) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Address does not support subtraction")
}
func (a *Address) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Address does not support multiplication")
}
func (a *Address) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Address does not support division")
}
func (a *Address) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Address does not support iteration")
}
func (a *Address) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Address is not indexable")
}
func (a *Address) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(a), nil
	case "name":
		return object.String(a.value.Name), nil
	case "address":
		return object.String(a.value.Address), nil
	}
	return nil, object.NewAttributeError("Address has no attribute '%s'", name)
}
func (a *Address) Attributes() []string { return []string{"attributes", "name", "address"} }

var _ object.Object = (*Address)(nil)
