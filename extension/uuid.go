package extension

import (
	googleuuid "github.com/google/uuid"

	"github.com/aisk/goblin/object"
)

// ExecuteUUID builds the uuid module.
func ExecuteUUID() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"new":      &object.Function{Name: "new", Fn: uuidNew},
			"parse":    &object.Function{Name: "parse", Fn: uuidParse},
			"validate": &object.Function{Name: "validate", Fn: uuidValidate},
		},
	}, nil
}

func uuidNew(args object.CallArgs) (object.Object, error) {
	if err := noArgs("new", args); err != nil {
		return nil, err
	}
	return NewUUID(googleuuid.New()), nil
}

func uuidParse(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse", args)
	value := p.Str("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	id, err := googleuuid.Parse(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "parse() failed", err)
	}
	return NewUUID(id), nil
}

func uuidValidate(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("validate", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	switch value := value.(type) {
	case *UUID:
		return object.True, nil
	case object.String:
		return object.Bool(googleuuid.Validate(string(value)) == nil), nil
	default:
		return nil, object.NewTypeError("validate() argument must be a UUID or string")
	}
}
