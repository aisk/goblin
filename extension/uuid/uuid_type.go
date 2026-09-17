package uuid

import (
	"encoding/binary"
	"hash/maphash"
	stdtime "time"
	stduuid "uuid"

	goblintime "github.com/aisk/goblin/extension/time"
	"github.com/aisk/goblin/object"
)

// hashSeed keys UUID hashes. Hashes never leave the process, so a per-process
// seed is fine.
var hashSeed = maphash.MakeSeed()

// UUID is Goblin's UUID value, a thin wrapper around Go's uuid.UUID. It takes
// part in the built-in traits Eq, Ord, Hashable and Show through its Object
// methods. Truth keeps OpaqueBase's always-true answer, as Go has no notion of
// a zero UUID beyond Nil() being an ordinary value.
type UUID struct {
	object.OpaqueBase
	Value stduuid.UUID
}

func NewUUID(value stduuid.UUID) *UUID {
	return &UUID{OpaqueBase: object.MakeOpaqueBase("UUID"), Value: value}
}

func (u *UUID) String() string            { return u.Value.String() }
func (u *UUID) ToString() (string, error) { return u.String(), nil }

func (u *UUID) Equals(other object.Object) (bool, error) {
	v, ok := other.(*UUID)
	return ok && u.Value == v.Value, nil
}

func (u *UUID) Compare(other object.Object) (int, error) {
	v, ok := other.(*UUID)
	if !ok {
		return 0, object.NewTypeError("cannot compare UUID with %s", other.TypeName())
	}
	return u.Value.Compare(v.Value), nil
}

func (u *UUID) Hash() (uint64, error) { return maphash.Bytes(hashSeed, u.Value[:]), nil }

func (u *UUID) GetAttr(name string) (object.Object, error) {
	if value, ok := uuidType.Attribute(name); ok {
		return value, nil
	}
	switch name {
	case "attributes":
		return object.AttributesFunction(u), nil
	case "version":
		return object.Integer(u.version()), nil
	case "bytes":
		return object.Bytes(append([]byte(nil), u.Value[:]...)), nil
	case "urn":
		return object.String("urn:uuid:" + u.Value.String()), nil
	case "time":
		if u.version() != 7 {
			return nil, object.NewValueError("UUID.time is only defined for version 7")
		}
		// RFC 9562 §5.7: the top 48 bits are milliseconds since the Unix epoch.
		var ms [8]byte
		copy(ms[2:], u.Value[:6])
		return goblintime.NewTime(stdtime.UnixMilli(int64(binary.BigEndian.Uint64(ms[:])))), nil
	default:
		return nil, object.NewAttributeError("UUID has no attribute '%s'", name)
	}
}

func (u *UUID) Attributes() []string {
	return uuidType.Attributes("attributes", "bytes", "urn", "version", "time")
}

// version reads the version field from the high nibble of octet 6.
func (u *UUID) version() int { return int(u.Value[6] >> 4) }

var (
	_ object.Object   = (*UUID)(nil)
	_ object.Hashable = (*UUID)(nil)
)
