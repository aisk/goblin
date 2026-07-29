package extension

import (
	"encoding/base32"

	"github.com/aisk/goblin/object"
)

func ExecuteBase32() (object.Object, error) {
	return &object.Module{Name: "base32", Members: map[string]object.Object{
		"encode":     &object.Function{Name: "encode", Fn: base32Encode},
		"decode":     &object.Function{Name: "decode", Fn: base32Decode},
		"hex_encode": &object.Function{Name: "hex_encode", Fn: base32HexEncode},
		"hex_decode": &object.Function{Name: "hex_decode", Fn: base32HexDecode},
	}}, nil
}

func base32Encoding(enc *base32.Encoding, padding bool) *base32.Encoding {
	if padding {
		return enc
	}
	return enc.WithPadding(base32.NoPadding)
}

func encodeBase32(name string, enc *base32.Encoding, args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser(name, args)
	value := p.Any("data")
	padding := p.BoolOr("padding", true)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	data, err := bytesOrString(name, "data", value)
	if err != nil {
		return nil, err
	}
	return object.String(base32Encoding(enc, bool(padding)).EncodeToString(data)), nil
}

func decodeBase32(name string, enc *base32.Encoding, args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser(name, args)
	data := p.Str("data")
	padding := p.BoolOr("padding", true)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	decoded, err := base32Encoding(enc, bool(padding)).DecodeString(string(data))
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() invalid base32 data", err)
	}
	return object.NewBytes(decoded), nil
}

func base32Encode(args object.CallArgs) (object.Object, error) {
	return encodeBase32("encode", base32.StdEncoding, args)
}

func base32Decode(args object.CallArgs) (object.Object, error) {
	return decodeBase32("decode", base32.StdEncoding, args)
}

func base32HexEncode(args object.CallArgs) (object.Object, error) {
	return encodeBase32("hex_encode", base32.HexEncoding, args)
}

func base32HexDecode(args object.CallArgs) (object.Object, error) {
	return decodeBase32("hex_decode", base32.HexEncoding, args)
}
