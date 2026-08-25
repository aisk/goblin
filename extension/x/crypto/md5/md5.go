package md5

import (
	"crypto/md5"

	"github.com/aisk/goblin/extension/internal/digest"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return digest.Module("md5", func(data []byte) []byte {
		sum := md5.Sum(data)
		return sum[:]
	}), nil
}
