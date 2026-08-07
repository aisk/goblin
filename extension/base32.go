package extension

import (
	"encoding/base32"

	"github.com/aisk/goblin/object"
)

func ExecuteBase32() (object.Object, error) {
	return &object.Module{Name: "base32", Members: map[string]object.Object{
		"encode": &object.Function{Name: "encode", Fn: base32Encode},
		"decode": &object.Function{Name: "decode", Fn: base32Decode},
	}}, nil
}

// base32Encoding selects the alphabet (standard or extended hex) and padding
// from the two config knobs, mirroring base64's url/padding keywords. The
// former hex_encode/hex_decode variants are folded into these keywords.
func base32Encoding(hex, padding bool) *base32.Encoding {
	enc := base32.StdEncoding
	if hex {
		enc = base32.HexEncoding
	}
	if !padding {
		enc = enc.WithPadding(base32.NoPadding)
	}
	return enc
}

func base32Encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	data := p.BytesLike("data")
	hex := p.BoolOr("hex", false)
	padding := p.BoolOr("padding", true)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(base32Encoding(bool(hex), bool(padding)).EncodeToString(data)), nil
}

func base32Decode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	data := p.Str("data")
	hex := p.BoolOr("hex", false)
	padding := p.BoolOr("padding", true)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	decoded, err := base32Encoding(bool(hex), bool(padding)).DecodeString(string(data))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decode() invalid base32 data", err)
	}
	return object.NewBytes(decoded), nil
}
