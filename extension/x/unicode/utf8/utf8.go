package utf8

import (
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "utf8", Members: map[string]object.Object{
		"valid":      &object.Function{Name: "valid", Fn: utf8Valid},
		"rune_count": &object.Function{Name: "rune_count", Fn: utf8RuneCount},
		"encode":     &object.Function{Name: "encode", Fn: utf8Encode},
		"decode":     &object.Function{Name: "decode", Fn: utf8Decode},
	}}, nil
}

func utf8Data(fnName string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(fnName, args)
	data := p.BytesLike("data")
	return data, p.Finish()
}

func utf8Valid(args object.CallArgs) (object.Object, error) {
	data, err := utf8Data("valid", args)
	if err != nil {
		return nil, err
	}
	return object.Bool(utf8.Valid(data)), nil
}

func utf8RuneCount(args object.CallArgs) (object.Object, error) {
	data, err := utf8Data("rune_count", args)
	if err != nil {
		return nil, err
	}
	return object.Integer(utf8.RuneCount(data)), nil
}

func utf8Encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	codepoint := p.Int("codepoint")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if codepoint < 0 || codepoint > utf8.MaxRune || codepoint >= 0xD800 && codepoint <= 0xDFFF {
		return nil, object.NewValueError("encode() invalid Unicode code point: %d", codepoint)
	}
	buffer := make([]byte, utf8.UTFMax)
	n := utf8.EncodeRune(buffer, rune(codepoint))
	return object.NewBytes(buffer[:n]), nil
}

func utf8Decode(args object.CallArgs) (object.Object, error) {
	data, err := utf8Data("decode", args)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, object.NewValueError("decode() data must not be empty")
	}
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError && size == 1 {
		return nil, object.NewParseError("decode() invalid UTF-8 data")
	}
	return &object.List{Elements: []object.Object{object.Integer(r), object.Integer(size)}}, nil
}
