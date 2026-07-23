package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestCompressionRoundTrips(t *testing.T) {
	for _, pair := range []struct {
		compress, decompress func(object.CallArgs) (object.Object, error)
	}{{gzipCompress, gzipDecompress}, {zlibCompress, zlibDecompress}} {
		compressed, err := pair.compress(object.CallArgs{Positional: object.Args{object.String("goblin goblin goblin")}})
		if err != nil {
			t.Fatal(err)
		}
		decompressed, err := pair.decompress(object.CallArgs{Positional: object.Args{compressed}})
		if err != nil {
			t.Fatal(err)
		}
		if string(decompressed.(object.Bytes)) != "goblin goblin goblin" {
			t.Fatalf("decompressed = %v", decompressed)
		}
	}
}
