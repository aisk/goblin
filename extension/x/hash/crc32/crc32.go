package crc32

import (
	"hash/crc32"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
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
