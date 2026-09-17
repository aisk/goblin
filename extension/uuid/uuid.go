package uuid

import (
	uuid "uuid"

	"github.com/aisk/goblin/object"
)

var uuidType = object.NewNativeConstructor("UUID", uuidConstructor)

// Execute builds the uuid module.
func Execute() (object.Object, error) {
	return &object.Module{
		Name: "uuid",
		Members: map[string]object.Object{
			"UUID": uuidType.Function,
			"new":  &object.Function{Name: "new", Fn: uuidNew},
			"NIL":  NewUUID(uuid.Nil()),
			"MAX":  NewUUID(uuid.Max()),
		},
	}, nil
}

func uuidConstructor(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("UUID", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	switch value := value.(type) {
	case *UUID:
		return value, nil
	case object.String:
		id, err := uuid.Parse(string(value))
		if err != nil {
			return nil, object.WrapError(object.ParseError, "UUID() invalid UUID string", err)
		}
		return NewUUID(id), nil
	case object.Bytes:
		if len(value) != len(uuid.UUID{}) {
			return nil, object.NewParseError("UUID() invalid UUID bytes: want 16 bytes, got %d", len(value))
		}
		return NewUUID(uuid.UUID(value)), nil
	default:
		return nil, object.NewTypeError("UUID() argument 'value' must be UUID, str, or Bytes, got %s", value.TypeName())
	}
}

func uuidNew(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("new", args)
	version, hasVersion := p.OptionalInt("version")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	if !hasVersion {
		return NewUUID(uuid.New()), nil
	}
	switch version {
	case 4:
		return NewUUID(uuid.NewV4()), nil
	case 7:
		return NewUUID(uuid.NewV7()), nil
	default:
		return nil, object.NewValueError("new() argument 'version' must be 4 or 7, got %d", int64(version))
	}
}
