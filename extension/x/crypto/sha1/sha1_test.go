package sha1

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestSHA1Hex(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "hex", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.String("42a2711c8294fe3a96bfc0f845a2395332de159a") {
		t.Fatalf("sha1.hex() = %v", got)
	}
}
