package adler32

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestAdler32Checksum(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "checksum", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.Integer(132579932) {
		t.Fatalf("adler32.checksum() = %v", got)
	}
}
