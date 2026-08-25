package ascii85

import (
	"encoding/ascii85"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "ascii85", Members: map[string]object.Object{
		"encode": &object.Function{Name: "encode", Fn: ascii85Encode},
		"decode": &object.Function{Name: "decode", Fn: ascii85Decode},
	}}, nil
}

func ascii85Encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	data := p.BytesLike("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	dst := make([]byte, ascii85.MaxEncodedLen(len(data)))
	n := ascii85.Encode(dst, data)
	return object.String(dst[:n]), nil
}

func ascii85Decode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	data := p.Str("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	dst := make([]byte, len(data))
	n, consumed, err := ascii85.Decode(dst, []byte(data), true)
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decode() invalid ascii85 data", err)
	}
	if consumed != len(data) {
		return nil, object.NewParseError("decode() invalid ascii85 data")
	}
	return object.NewBytes(dst[:n]), nil
}
