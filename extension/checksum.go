package extension

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"hash/adler32"
	"hash/crc32"

	"github.com/aisk/goblin/object"
)

func digestModule(name string, sum func([]byte) []byte) *object.Module {
	digest := func(fnName string, hexOutput bool) *object.Function {
		return &object.Function{Name: fnName, Fn: func(args object.CallArgs) (object.Object, error) {
			data, err := shaData(fnName, args)
			if err != nil {
				return nil, err
			}
			value := sum(data)
			if hexOutput {
				return object.String(hex.EncodeToString(value)), nil
			}
			return object.NewBytes(value), nil
		}}
	}
	return &object.Module{Name: name, Members: map[string]object.Object{
		"sum": digest("sum", false),
		"hex": digest("hex", true),
	}}
}

func ExecuteMD5() (object.Object, error) {
	return digestModule("md5", func(data []byte) []byte {
		sum := md5.Sum(data)
		return sum[:]
	}), nil
}

func ExecuteSHA1() (object.Object, error) {
	return digestModule("sha1", func(data []byte) []byte {
		sum := sha1.Sum(data)
		return sum[:]
	}), nil
}

func ExecuteCRC32() (object.Object, error) {
	return &object.Module{Name: "crc32", Members: map[string]object.Object{
		"IEEE":       object.Integer(crc32.IEEE),
		"CASTAGNOLI": object.Integer(crc32.Castagnoli),
		"KOOPMAN":    object.Integer(crc32.Koopman),
		"checksum":   &object.Function{Name: "checksum", Fn: crc32Checksum},
	}}, nil
}

func crc32Checksum(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("checksum", args)
	data := p.BytesLike("data")
	polynomial := p.IntOr("polynomial", object.Integer(crc32.IEEE))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Integer(crc32.Checksum(data, crc32.MakeTable(uint32(polynomial)))), nil
}

func ExecuteAdler32() (object.Object, error) {
	return &object.Module{Name: "adler32", Members: map[string]object.Object{
		"checksum": &object.Function{Name: "checksum", Fn: adler32Checksum},
	}}, nil
}

func adler32Checksum(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("checksum", args)
	data := p.BytesLike("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Integer(adler32.Checksum(data)), nil
}
