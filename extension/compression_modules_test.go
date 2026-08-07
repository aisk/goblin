package extension

import (
	"encoding/base64"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestFlateRoundTrip(t *testing.T) {
	compressed := callModuleFunction(t, ExecuteFlate, "compress", object.CallArgs{Positional: object.Args{object.String("Goblin compression")}})
	decompressed := callModuleFunction(t, ExecuteFlate, "decompress", object.CallArgs{Positional: object.Args{compressed}})
	if !objectEquals(decompressed, object.NewBytes([]byte("Goblin compression"))) {
		t.Fatalf("decompress() = %v", decompressed)
	}
}

func TestBzip2Decompress(t *testing.T) {
	compressed, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWfJg5WUAAACVgEAAAIAaJ9gAIAAimAaNCAaAL8F0GoxLDaaPC7kinChIeTBysoA=")
	if err != nil {
		t.Fatal(err)
	}
	got := callModuleFunction(t, ExecuteBzip2, "decompress", object.CallArgs{Positional: object.Args{object.NewBytes(compressed)}})
	if !objectEquals(got, object.NewBytes([]byte("Goblin compression"))) {
		t.Fatalf("decompress() = %v", got)
	}
}

func TestLZWRoundTrip(t *testing.T) {
	compressed := callModuleFunction(t, ExecuteLZW, "compress", object.CallArgs{Positional: object.Args{object.String("Goblin LZW data")}})
	decompressed := callModuleFunction(t, ExecuteLZW, "decompress", object.CallArgs{Positional: object.Args{compressed}})
	if !objectEquals(decompressed, object.NewBytes([]byte("Goblin LZW data"))) {
		t.Fatalf("decompress() = %v", decompressed)
	}
}
