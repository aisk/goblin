// Package json converts between JSON text and Goblin values. The codec lives
// in extension/internal/jsonvalue, which http shares.
package json

import (
	"github.com/aisk/goblin/extension/internal/jsonvalue"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("json", nil, object.Funcs{
		"encode": encode,
		"decode": decode,
	}), nil
}

// encode(value, indent=nil) returns the JSON text of value.
func encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	value := p.Any("value")
	indent, pretty := p.NullableInt("indent")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if indent < 0 {
		return nil, object.NegativeArgumentError("encode", "indent", int64(indent))
	}
	text, err := jsonvalue.Encode("encode", value, int(indent), pretty)
	if err != nil {
		return nil, err
	}
	return object.String(text), nil
}

// decode(data) returns the value of the JSON document data holds: a String,
// Bytes or a readable read to its end.
func decode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	data := p.DataBytes("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return jsonvalue.Decode("decode", data)
}
