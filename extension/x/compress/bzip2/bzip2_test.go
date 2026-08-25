package bzip2

import (
	"encoding/base64"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestBzip2Decompress(t *testing.T) {
	compressed, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWfJg5WUAAACVgEAAAIAaJ9gAIAAimAaNCAaAL8F0GoxLDaaPC7kinChIeTBysoA=")
	if err != nil {
		t.Fatal(err)
	}
	got := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{object.NewBytes(compressed)}})
	if !modtest.Equal(got, object.NewBytes([]byte("Goblin compression"))) {
		t.Fatalf("decompress() = %v", got)
	}
}
