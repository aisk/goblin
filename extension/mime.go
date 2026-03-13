package extension

import (
	"fmt"
	stdmime "mime"

	"github.com/aisk/goblin/object"
)

func ExecuteMime() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"TypeByExtension":  &object.Function{Name: "TypeByExtension", Fn: mimeTypeByExtension},
			"ExtensionsByType": &object.Function{Name: "ExtensionsByType", Fn: mimeExtensionsByType},
		},
	}, nil
}

func mimeTypeByExtension(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("TypeByExtension", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("TypeByExtension() requires exactly 1 argument")
	}

	ext, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, fmt.Errorf("TypeByExtension() argument must be a string")
	}

	return object.String(stdmime.TypeByExtension(string(ext))), nil
}

func mimeExtensionsByType(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("ExtensionsByType", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("ExtensionsByType() requires exactly 1 argument")
	}

	mimeType, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, fmt.Errorf("ExtensionsByType() argument must be a string")
	}

	extensions, err := stdmime.ExtensionsByType(string(mimeType))
	if err != nil {
		return nil, err
	}

	elements := make([]object.Object, 0, len(extensions))
	for _, ext := range extensions {
		elements = append(elements, object.String(ext))
	}

	return &object.List{Elements: elements}, nil
}
