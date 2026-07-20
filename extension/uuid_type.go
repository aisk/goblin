package extension

import (
	"bytes"

	googleuuid "github.com/google/uuid"

	"github.com/aisk/goblin/object"
)

// UUID is Goblin's UUID value. It is deliberately defined in extension so the
// core object package remains independent of the Google UUID implementation.
type UUID struct {
	Value googleuuid.UUID
}

func NewUUID(value googleuuid.UUID) *UUID { return &UUID{Value: value} }

func (u *UUID) String() string              { return u.Value.String() }
func (u *UUID) ToString() (string, error)   { return u.String(), nil }
func (u *UUID) Bool() bool                  { return u.Value != googleuuid.Nil }
func (u *UUID) ToBool() (bool, error)       { return u.Bool(), nil }
func (u *UUID) Not() (object.Object, error) { return object.Bool(!u.Bool()), nil }

func (u *UUID) Compare(other object.Object) (int, error) {
	v, ok := other.(*UUID)
	if !ok {
		return 0, object.NewTypeError("cannot compare UUID with %T", other)
	}
	return bytes.Compare(u.Value[:], v.Value[:]), nil
}

func (u *UUID) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add UUID")
}
func (u *UUID) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract UUID")
}
func (u *UUID) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply UUID")
}
func (u *UUID) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot divide UUID")
}
func (u *UUID) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("UUID does not support iteration")
}
func (u *UUID) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("UUID is not indexable")
}

func (u *UUID) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(u), nil
	case "version":
		return object.Integer(u.Value.Version()), nil
	case "variant":
		return object.String(u.Value.Variant().String()), nil
	default:
		return nil, object.NewAttributeError("UUID has no attribute '%s'", name)
	}
}

func (u *UUID) Attributes() []string {
	return []string{"attributes", "version", "variant"}
}

var _ object.Object = (*UUID)(nil)
