package fnv

import (
	"encoding/hex"
	"hash"
	"hash/fnv"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "fnv", Members: map[string]object.Object{
		"sum":      &object.Function{Name: "sum", Fn: fnvSum},
		"hex":      &object.Function{Name: "hex", Fn: fnvHex},
		"FNV_32":   object.String("32"),
		"FNV_32A":  object.String("32a"),
		"FNV_64":   object.String("64"),
		"FNV_64A":  object.String("64a"),
		"FNV_128":  object.String("128"),
		"FNV_128A": object.String("128a"),
	}}, nil
}

func fnvDigest(fnName string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(fnName, args)
	data := p.BytesLike("data")
	variant := p.StrOr("variant", object.String("64a"))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var digest hash.Hash
	switch variant {
	case "32":
		digest = fnv.New32()
	case "32a":
		digest = fnv.New32a()
	case "64":
		digest = fnv.New64()
	case "64a":
		digest = fnv.New64a()
	case "128":
		digest = fnv.New128()
	case "128a":
		digest = fnv.New128a()
	default:
		return nil, object.NewValueError("%s() unsupported variant %q", fnName, variant)
	}
	_, _ = digest.Write(data)
	return digest.Sum(nil), nil
}

func fnvSum(args object.CallArgs) (object.Object, error) {
	value, err := fnvDigest("sum", args)
	if err != nil {
		return nil, err
	}
	return object.NewBytes(value), nil
}

func fnvHex(args object.CallArgs) (object.Object, error) {
	value, err := fnvDigest("hex", args)
	if err != nil {
		return nil, err
	}
	return object.String(hex.EncodeToString(value)), nil
}
