package mail

import (
	stdmail "net/mail"
	"strings"

	"github.com/aisk/goblin/object"
)

type Address struct {
	object.OpaqueBase
	value *stdmail.Address
}

func NewAddress(value *stdmail.Address) *Address {
	return &Address{OpaqueBase: object.MakeOpaqueBase("Address"), value: value}
}

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
func (a *Address) GetAttr(name string) (object.Object, error) {
	if value, ok := addressType.Attribute(name); ok {
		return value, nil
	}
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
func (a *Address) Attributes() []string {
	return addressType.Attributes("attributes", "name", "address")
}

var _ object.Object = (*Address)(nil)
