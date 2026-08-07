package extension

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"

	"github.com/aisk/goblin/object"
)

func ExecuteSHA256() (object.Object, error) {
	return &object.Module{Name: "sha256", Members: map[string]object.Object{
		"sum":    sha2Function("sum", sha256Digest),
		"hex":    sha2HexFunction("hex", sha256Digest),
		"sum224": sha2Function("sum224", sha224Digest),
		"hex224": sha2HexFunction("hex224", sha224Digest),
	}}, nil
}

func ExecuteSHA512() (object.Object, error) {
	return &object.Module{Name: "sha512", Members: map[string]object.Object{
		"sum":    sha2Function("sum", sha512Digest),
		"hex":    sha2HexFunction("hex", sha512Digest),
		"sum384": sha2Function("sum384", sha384Digest),
		"hex384": sha2HexFunction("hex384", sha384Digest),
		"sum224": sha2Function("sum224", sha512224Digest),
		"hex224": sha2HexFunction("hex224", sha512224Digest),
		"sum256": sha2Function("sum256", sha512256Digest),
		"hex256": sha2HexFunction("hex256", sha512256Digest),
	}}, nil
}

func shaData(name string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(name, args)
	data := p.BytesLike("data")
	return data, p.Finish()
}

func sha2Function(name string, digest func([]byte) []byte) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		data, err := shaData(name, args)
		if err != nil {
			return nil, err
		}
		return object.NewBytes(digest(data)), nil
	}}
}

func sha2HexFunction(name string, digest func([]byte) []byte) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		data, err := shaData(name, args)
		if err != nil {
			return nil, err
		}
		return object.String(hex.EncodeToString(digest(data))), nil
	}}
}

func sha256Digest(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func sha224Digest(data []byte) []byte {
	sum := sha256.Sum224(data)
	return sum[:]
}

func sha512Digest(data []byte) []byte {
	sum := sha512.Sum512(data)
	return sum[:]
}

func sha384Digest(data []byte) []byte {
	sum := sha512.Sum384(data)
	return sum[:]
}

func sha512224Digest(data []byte) []byte {
	sum := sha512.Sum512_224(data)
	return sum[:]
}

func sha512256Digest(data []byte) []byte {
	sum := sha512.Sum512_256(data)
	return sum[:]
}
