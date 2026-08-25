package crc32

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestCRC32Checksum(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "checksum", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.Integer(1982524054) {
		t.Fatalf("crc32.checksum() = %v", got)
	}
}
