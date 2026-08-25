package zlib

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestZlibRoundTrip(t *testing.T) {
	compressed, err := zlibCompress(object.CallArgs{Positional: object.Args{object.String("goblin goblin goblin")}})
	if err != nil {
		t.Fatal(err)
	}
	decompressed, err := zlibDecompress(object.CallArgs{Positional: object.Args{compressed}})
	if err != nil {
		t.Fatal(err)
	}
	if string(decompressed.(object.Bytes)) != "goblin goblin goblin" {
		t.Fatalf("decompressed = %v", decompressed)
	}
}
