package sha256

import (
	"crypto/sha256"

	"github.com/aisk/goblin/extension/internal/digest"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "sha256", Members: map[string]object.Object{
		"sum":    digest.Function("sum", sha256Digest),
		"hex":    digest.HexFunction("hex", sha256Digest),
		"sum224": digest.Function("sum224", sha224Digest),
		"hex224": digest.HexFunction("hex224", sha224Digest),
	}}, nil
}

func sha256Digest(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func sha224Digest(data []byte) []byte {
	sum := sha256.Sum224(data)
	return sum[:]
}
