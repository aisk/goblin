package fnv

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestFNVHex(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "hex", object.CallArgs{Positional: object.Args{object.String("hello")}}); got != object.String("a430d84680aabd0b") {
		t.Fatalf("fnv.hex() = %v", got)
	}
}
