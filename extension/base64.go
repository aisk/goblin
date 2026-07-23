package extension

import (
	"encoding/base64"

	"github.com/aisk/goblin/object"
)

func ExecuteBase64() (object.Object, error) {
	return &object.Module{Name: "base64", Members: map[string]object.Object{
		"encode":     &object.Function{Name: "encode", Fn: base64Encode},
		"decode":     &object.Function{Name: "decode", Fn: base64Decode},
		"url_encode": &object.Function{Name: "url_encode", Fn: base64URLEncode},
		"url_decode": &object.Function{Name: "url_decode", Fn: base64URLDecode},
	}}, nil
}

func base64Encode(args object.CallArgs) (object.Object, error) {
	return encodeBase64("encode", base64.StdEncoding, args)
}

func base64Decode(args object.CallArgs) (object.Object, error) {
	return decodeBase64("decode", base64.StdEncoding, args)
}

func base64URLEncode(args object.CallArgs) (object.Object, error) {
	return encodeBase64("url_encode", base64.RawURLEncoding, args)
}

func base64URLDecode(args object.CallArgs) (object.Object, error) {
	return decodeBase64("url_decode", base64.RawURLEncoding, args)
}

func encodeBase64(name string, encoding *base64.Encoding, args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser(name, args)
	value := ap.Any("data")
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	var data []byte
	switch value := value.(type) {
	case object.Bytes:
		data = []byte(value)
	case object.String:
		data = []byte(value)
	default:
		return nil, object.NewTypeError("%s() argument 'data' must be str or Bytes, got %T", name, value)
	}
	return object.String(encoding.EncodeToString(data)), nil
}

func decodeBase64(name string, encoding *base64.Encoding, args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser(name, args)
	value := ap.Str("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	data, err := encoding.DecodeString(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() failed", err)
	}
	return object.NewBytes(data), nil
}
