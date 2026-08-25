package flate

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestFlateRoundTrip(t *testing.T) {
	compressed := modtest.CallModule(t, Execute, "compress", object.CallArgs{Positional: object.Args{object.String("Goblin compression")}})
	decompressed := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{compressed}})
	if !modtest.Equal(decompressed, object.NewBytes([]byte("Goblin compression"))) {
		t.Fatalf("decompress() = %v", decompressed)
	}
}
