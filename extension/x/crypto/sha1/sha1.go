package sha1

import (
	"crypto/sha1"

	"github.com/aisk/goblin/extension/internal/digest"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return digest.Module("sha1", func(data []byte) []byte {
		sum := sha1.Sum(data)
		return sum[:]
	}), nil
}
