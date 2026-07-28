package extension

import (
	"encoding/hex"

	"github.com/aisk/goblin/object"
)

func ExecuteHex() (object.Object, error) {
	return &object.Module{Name: "hex", Members: map[string]object.Object{
		"encode": &object.Function{Name: "encode", Fn: hexEncode},
		"decode": &object.Function{Name: "decode", Fn: hexDecode},
		"dump":   &object.Function{Name: "dump", Fn: hexDump},
	}}, nil
}

func hexBytes(name string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(name, args)
	value := p.Any("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case object.Bytes:
		return []byte(v), nil
	case object.String:
		return []byte(v), nil
	default:
		return nil, object.NewTypeError("%s() argument 'data' must be Bytes or str, got %s", name, value.TypeName())
	}
}

func hexEncode(args object.CallArgs) (object.Object, error) {
	data, err := hexBytes("encode", args)
	if err != nil {
		return nil, err
	}
	return object.String(hex.EncodeToString(data)), nil
}

func hexDecode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	value := p.Str("s")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	data, err := hex.DecodeString(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decode() failed", err)
	}
	return object.NewBytes(data), nil
}

func hexDump(args object.CallArgs) (object.Object, error) {
	data, err := hexBytes("dump", args)
	if err != nil {
		return nil, err
	}
	return object.String(hex.Dump(data)), nil
}
