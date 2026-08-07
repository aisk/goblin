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
	data := p.BytesLike("data")
	return data, p.Finish()
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
	value := p.Str("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	data, err := hex.DecodeString(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decode() invalid hex data", err)
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
