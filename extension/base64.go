package extension

import (
	"encoding/base64"

	"github.com/aisk/goblin/object"
)

func ExecuteBase64() (object.Object, error) {
	return &object.Module{Name: "base64", Members: map[string]object.Object{
		"encode": &object.Function{Name: "encode", Fn: base64Encode},
		"decode": &object.Function{Name: "decode", Fn: base64Decode},
	}}, nil
}

// base64Encoding selects among Go's encodings from the two independent
// config knobs: the alphabet (standard or URL-safe) and padding. The former
// url_encode/url_decode variants hard-coupled the URL alphabet to unpadded
// output; expressed as keywords the axes compose freely.
func base64Encoding(url, padding bool) *base64.Encoding {
	enc := base64.StdEncoding
	if url {
		enc = base64.URLEncoding
	}
	if !padding {
		enc = enc.WithPadding(base64.NoPadding)
	}
	return enc
}

func base64Encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	data := p.BytesLike("data")
	url := p.BoolOr("url", false)
	padding := p.BoolOr("padding", true)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(base64Encoding(bool(url), bool(padding)).EncodeToString(data)), nil
}

func base64Decode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	value := p.Str("data")
	url := p.BoolOr("url", false)
	padding := p.BoolOr("padding", true)
	if err := p.Finish(); err != nil {
		return nil, err
	}

	data, err := base64Encoding(bool(url), bool(padding)).DecodeString(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decode() invalid base64 data", err)
	}
	return object.NewBytes(data), nil
}
