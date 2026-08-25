package hmac

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
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
	key := p.BytesLike("key")
	data := p.BytesLike("data")
	algorithm := p.StrOr("algorithm", object.String("sha256"))
	if err := p.Finish(); err != nil {
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
	expected := p.BytesLike("signature")
	key := p.BytesLike("key")
	data := p.BytesLike("data")
	algorithm := p.StrOr("algorithm", object.String("sha256"))
	if err := p.Finish(); err != nil {
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
