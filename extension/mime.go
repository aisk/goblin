package extension

import (
	"mime"

	"github.com/aisk/goblin/object"
)

func ExecuteMime() (object.Object, error) {
	return &object.Module{
		Name: "mime",
		Members: map[string]object.Object{
			"type_by_extension":  &object.Function{Name: "type_by_extension", Fn: mimeTypeByExtension},
			"extensions_by_type": &object.Function{Name: "extensions_by_type", Fn: mimeExtensionsByType},
		},
	}, nil
}

func mimeTypeByExtension(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("type_by_extension", args)
	ext := p.Str("extension")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	mimeType := mime.TypeByExtension(string(ext))
	if mimeType == "" {
		return object.Nil, nil
	}
	return object.String(mimeType), nil
}

func mimeExtensionsByType(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("extensions_by_type", args)
	mimeType := p.Str("mime_type")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	extensions, err := mime.ExtensionsByType(string(mimeType))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "extensions_by_type() invalid MIME type", err)
	}

	elements := make([]object.Object, 0, len(extensions))
	for _, ext := range extensions {
		elements = append(elements, object.String(ext))
	}

	return &object.List{Elements: elements}, nil
}
