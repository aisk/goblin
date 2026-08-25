package sha512

import (
	"crypto/sha512"

	"github.com/aisk/goblin/extension/internal/digest"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "sha512", Members: map[string]object.Object{
		"sum":    digest.Function("sum", sha512Digest),
		"hex":    digest.HexFunction("hex", sha512Digest),
		"sum384": digest.Function("sum384", sha384Digest),
		"hex384": digest.HexFunction("hex384", sha384Digest),
		"sum224": digest.Function("sum224", sha512224Digest),
		"hex224": digest.HexFunction("hex224", sha512224Digest),
		"sum256": digest.Function("sum256", sha512256Digest),
		"hex256": digest.HexFunction("hex256", sha512256Digest),
	}}, nil
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
