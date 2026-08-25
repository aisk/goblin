package quotedprintable

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestQuotedPrintableRoundTrip(t *testing.T) {
	encoded := modtest.CallModule(t, Execute, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin = ゴブリン")}})
	decoded := modtest.CallModule(t, Execute, "decode", object.CallArgs{Positional: object.Args{encoded}})
	if !modtest.Equal(decoded, object.NewBytes([]byte("Goblin = ゴブリン"))) {
		t.Fatalf("decode() = %v", decoded)
	}
}
