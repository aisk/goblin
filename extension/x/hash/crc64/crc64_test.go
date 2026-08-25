package crc64

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestCRC64Hex(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "hex", object.CallArgs{Positional: object.Args{object.String("123456789")}}); got != object.String("995dc9bbdf1939fa") {
		t.Fatalf("crc64.hex() = %v", got)
	}
}
