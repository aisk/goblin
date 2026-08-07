package extension

import (
	"bytes"
	stdtime "time"

	googleuuid "github.com/google/uuid"

	goblintime "github.com/aisk/goblin/extension/time"
	"github.com/aisk/goblin/object"
)

// UUID is Goblin's UUID value. It is deliberately defined in extension so the
// core object package remains independent of the Google UUID implementation.
type UUID struct {
	object.OpaqueBase
	Value googleuuid.UUID
}

func NewUUID(value googleuuid.UUID) *UUID {
	return &UUID{OpaqueBase: object.MakeOpaqueBase("UUID"), Value: value}
}

func (u *UUID) String() string              { return u.Value.String() }
func (u *UUID) ToString() (string, error)   { return u.String(), nil }
func (u *UUID) ToBool() (bool, error)       { return u.Value != googleuuid.Nil, nil }
func (u *UUID) Not() (object.Object, error) { return object.Bool(u.Value == googleuuid.Nil), nil }

func (u *UUID) Equals(other object.Object) (bool, error) {
	v, ok := other.(*UUID)
	return ok && u.Value == v.Value, nil
}

func (u *UUID) Compare(other object.Object) (int, error) {
	v, ok := other.(*UUID)
	if !ok {
		return 0, object.NewTypeError("cannot compare UUID with %s", other.TypeName())
	}
	return bytes.Compare(u.Value[:], v.Value[:]), nil
}

func (u *UUID) GetAttr(name string) (object.Object, error) {
	if value, ok := uuidType.Attribute(name); ok {
		return value, nil
	}
	switch name {
	case "attributes":
		return object.AttributesFunction(u), nil
	case "version":
		return object.Integer(u.Value.Version()), nil
	case "variant":
		return object.String(u.Value.Variant().String()), nil
	case "bytes":
		return object.Bytes(append([]byte(nil), u.Value[:]...)), nil
	case "urn":
		return object.String(u.Value.URN()), nil
	case "time":
		if !u.hasTime() {
			return nil, object.NewValueError("UUID.time is only defined for versions 1, 6, and 7")
		}
		sec, nsec := u.Value.Time().UnixTime()
		return goblintime.NewTime(stdtime.Unix(sec, nsec)), nil
	case "clock_sequence":
		if u.Value.Version() != 1 {
			return nil, object.NewValueError("UUID.clock_sequence is only defined for version 1")
		}
		return object.Integer(u.Value.ClockSequence()), nil
	case "node":
		if u.Value.Version() != 1 && u.Value.Version() != 6 {
			return nil, object.NewValueError("UUID.node is only defined for versions 1 and 6")
		}
		return object.Bytes(u.Value.NodeID()), nil
	default:
		return nil, object.NewAttributeError("UUID has no attribute '%s'", name)
	}
}

func (u *UUID) Attributes() []string {
	return uuidType.Attributes("attributes", "bytes", "urn", "version", "variant", "time", "clock_sequence", "node")
}

func (u *UUID) hasTime() bool {
	version := u.Value.Version()
	return version == 1 || version == 6 || version == 7
}

var _ object.Object = (*UUID)(nil)
