package md5

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestMD5Hex(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "hex", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.String("8e81e2940511f7152ba4462fe53e35b8") {
		t.Fatalf("md5.hex() = %v", got)
	}
}
