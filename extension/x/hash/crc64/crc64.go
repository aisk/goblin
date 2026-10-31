package crc64

import (
	"encoding/binary"
	"encoding/hex"
	"hash/crc64"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("crc64", map[string]object.Object{
		"ECMA": object.String("ecma"),
		"ISO":  object.String("iso"),
	}, object.Funcs{
		"sum": crc64Sum,
		"hex": crc64Hex,
	}), nil
}

func crc64Digest(fnName string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(fnName, args)
	data := p.BytesLike("data")
	polynomial := p.StrOr("polynomial", object.String("ecma"))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var poly uint64
	switch polynomial {
	case "ecma":
		poly = crc64.ECMA
	case "iso":
		poly = crc64.ISO
	default:
		return nil, object.NewValueError("%s() argument 'polynomial' must be crc64.ECMA or crc64.ISO", fnName)
	}
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, crc64.Checksum(data, crc64.MakeTable(poly)))
	return value, nil
}

func crc64Sum(args object.CallArgs) (object.Object, error) {
	value, err := crc64Digest("sum", args)
	if err != nil {
		return nil, err
	}
	return object.NewBytes(value), nil
}

func crc64Hex(args object.CallArgs) (object.Object, error) {
	value, err := crc64Digest("hex", args)
	if err != nil {
		return nil, err
	}
	return object.String(hex.EncodeToString(value)), nil
}
