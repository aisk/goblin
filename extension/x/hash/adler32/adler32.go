package adler32

import (
	"hash/adler32"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
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
