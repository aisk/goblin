package extension

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"hash/crc64"
	"hash/fnv"

	"github.com/aisk/goblin/object"
)

func ExecuteHMAC() (object.Object, error) {
	return &object.Module{Name: "hmac", Members: map[string]object.Object{
		"sum":    &object.Function{Name: "sum", Fn: hmacSum},
		"hex":    &object.Function{Name: "hex", Fn: hmacHex},
		"verify": &object.Function{Name: "verify", Fn: hmacVerify},
	}}, nil
}

func hmacHash(name string) (func() hash.Hash, bool) {
	switch name {
	case "sha256":
		return sha256.New, true
	case "sha512":
		return sha512.New, true
	case "sha1":
		return sha1.New, true
	case "md5":
		return md5.New, true
	default:
		return nil, false
	}
}

func hmacDigest(fnName string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(fnName, args)
	keyObj := p.Any("key")
	dataObj := p.Any("data")
	algorithm := p.StrOr("algorithm", object.String("sha256"))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	key, err := bytesOrString(fnName, "key", keyObj)
	if err != nil {
		return nil, err
	}
	data, err := bytesOrString(fnName, "data", dataObj)
	if err != nil {
		return nil, err
	}
	newHash, ok := hmacHash(string(algorithm))
	if !ok {
		return nil, object.NewValueError("%s() unsupported algorithm %q", fnName, algorithm)
	}
	mac := hmac.New(newHash, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil), nil
}

func hmacSum(args object.CallArgs) (object.Object, error) {
	value, err := hmacDigest("sum", args)
	if err != nil {
		return nil, err
	}
	return object.NewBytes(value), nil
}

func hmacHex(args object.CallArgs) (object.Object, error) {
	value, err := hmacDigest("hex", args)
	if err != nil {
		return nil, err
	}
	return object.String(hex.EncodeToString(value)), nil
}

func hmacVerify(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("verify", args)
	expectedObj := p.Any("signature")
	keyObj := p.Any("key")
	dataObj := p.Any("data")
	algorithm := p.StrOr("algorithm", object.String("sha256"))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	expected, err := bytesOrString("verify", "signature", expectedObj)
	if err != nil {
		return nil, err
	}
	key, err := bytesOrString("verify", "key", keyObj)
	if err != nil {
		return nil, err
	}
	data, err := bytesOrString("verify", "data", dataObj)
	if err != nil {
		return nil, err
	}
	newHash, ok := hmacHash(string(algorithm))
	if !ok {
		return nil, object.NewValueError("verify() unsupported algorithm %q", algorithm)
	}
	mac := hmac.New(newHash, key)
	_, _ = mac.Write(data)
	return object.Bool(hmac.Equal(expected, mac.Sum(nil))), nil
}

func ExecuteCRC64() (object.Object, error) {
	return &object.Module{Name: "crc64", Members: map[string]object.Object{
		"sum": &object.Function{Name: "sum", Fn: crc64Sum},
		"hex": &object.Function{Name: "hex", Fn: crc64Hex},
	}}, nil
}

func crc64Digest(fnName string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(fnName, args)
	dataObj := p.Any("data")
	polynomial := p.StrOr("polynomial", object.String("ecma"))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	data, err := bytesOrString(fnName, "data", dataObj)
	if err != nil {
		return nil, err
	}
	var poly uint64
	switch polynomial {
	case "ecma":
		poly = crc64.ECMA
	case "iso":
		poly = crc64.ISO
	default:
		return nil, object.NewValueError("%s() argument 'polynomial' must be \"ecma\" or \"iso\"", fnName)
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

func ExecuteFNV() (object.Object, error) {
	return &object.Module{Name: "fnv", Members: map[string]object.Object{
		"sum": &object.Function{Name: "sum", Fn: fnvSum},
		"hex": &object.Function{Name: "hex", Fn: fnvHex},
	}}, nil
}

func fnvDigest(fnName string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(fnName, args)
	dataObj := p.Any("data")
	variant := p.StrOr("variant", object.String("64a"))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	data, err := bytesOrString(fnName, "data", dataObj)
	if err != nil {
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
