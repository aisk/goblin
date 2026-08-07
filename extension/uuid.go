package extension

import (
	googleuuid "github.com/google/uuid"

	"github.com/aisk/goblin/object"
)

var uuidType = object.NewNativeConstructor("UUID", uuidConstructor)

// ExecuteUUID builds the uuid module.
func ExecuteUUID() (object.Object, error) {
	return &object.Module{
		Name: "uuid",
		Members: map[string]object.Object{
			"UUID":           uuidType.Function,
			"new":            &object.Function{Name: "new", Fn: uuidNew},
			"is_valid":       &object.Function{Name: "is_valid", Fn: uuidIsValid},
			"NAMESPACE_DNS":  NewUUID(googleuuid.NameSpaceDNS),
			"NAMESPACE_URL":  NewUUID(googleuuid.NameSpaceURL),
			"NAMESPACE_OID":  NewUUID(googleuuid.NameSpaceOID),
			"NAMESPACE_X500": NewUUID(googleuuid.NameSpaceX500),
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
		id, err := googleuuid.Parse(string(value))
		if err != nil {
			return nil, object.WrapError(object.ParseError, "UUID() invalid UUID string", err)
		}
		return NewUUID(id), nil
	case object.Bytes:
		id, err := googleuuid.FromBytes([]byte(value))
		if err != nil {
			return nil, object.WrapError(object.ParseError, "UUID() invalid UUID bytes", err)
		}
		return NewUUID(id), nil
	default:
		return nil, object.NewTypeError("UUID() argument 'value' must be UUID, str, or Bytes, got %s", value.TypeName())
	}
}

func uuidNew(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("new", args)
	version := p.IntOr("version", object.Integer(4))
	namespace := p.AnyOr("namespace", object.Nil)
	data := p.AnyOr("data", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}

	v := int64(version)
	if v != 1 && v != 3 && v != 4 && v != 5 && v != 6 && v != 7 {
		return nil, object.NewValueError("new() argument 'version' must be one of 1, 3, 4, 5, 6, or 7, got %d", v)
	}

	if v == 3 || v == 5 {
		if namespace == object.Nil {
			return nil, object.NewTypeError("new() missing required argument for version %d: 'namespace'", v)
		}
		space, ok := namespace.(*UUID)
		if !ok {
			return nil, object.NewTypeError("new() argument 'namespace' must be UUID, got %s", namespace.TypeName())
		}
		if data == object.Nil {
			return nil, object.NewTypeError("new() missing required argument for version %d: 'data'", v)
		}
		dp := object.NewArgParser("new", object.CallArgs{Positional: object.Args{data}})
		raw := dp.BytesLike("data")
		if err := dp.Finish(); err != nil {
			return nil, err
		}
		if v == 3 {
			return NewUUID(googleuuid.NewMD5(space.Value, raw)), nil
		}
		return NewUUID(googleuuid.NewSHA1(space.Value, raw)), nil
	}

	if namespace != object.Nil || data != object.Nil {
		return nil, object.NewTypeError("new() arguments 'namespace' and 'data' are only valid for versions 3 and 5")
	}

	var id googleuuid.UUID
	var err error
	switch v {
	case 1:
		id, err = googleuuid.NewUUID()
	case 4:
		id, err = googleuuid.NewRandom()
	case 6:
		id, err = googleuuid.NewV6()
	case 7:
		id, err = googleuuid.NewV7()
	}
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "new() failed to generate UUID", err)
	}
	return NewUUID(id), nil
}

func uuidIsValid(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("is_valid", args)
	value := p.Str("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Bool(googleuuid.Validate(string(value)) == nil), nil
}
