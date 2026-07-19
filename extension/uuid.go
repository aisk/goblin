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
	if err := object.RequireNoKeyword("new", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("new() requires no arguments")
	}
	return NewUUID(googleuuid.New()), nil
}

func uuidParse(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("parse", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, object.NewTypeError("parse() requires exactly 1 argument")
	}

	value, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, object.NewTypeError("parse() argument must be a string")
	}

	id, err := googleuuid.Parse(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "parse() failed", err)
	}
	return NewUUID(id), nil
}

func uuidValidate(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("validate", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, object.NewTypeError("validate() requires exactly 1 argument")
	}

	switch value := args.Positional[0].(type) {
	case *UUID:
		return object.True, nil
	case object.String:
		return object.Bool(googleuuid.Validate(string(value)) == nil), nil
	default:
		return nil, object.NewTypeError("validate() argument must be a UUID or string")
	}
}
