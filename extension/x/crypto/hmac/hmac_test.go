package hmac

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestHMAC(t *testing.T) {
	got := modtest.CallModule(t, Execute, "hex", object.CallArgs{Positional: object.Args{object.String("key"), object.String("The quick brown fox jumps over the lazy dog")}})
	if got != object.String("f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8") {
		t.Fatalf("hex() = %v", got)
	}
	signature := modtest.CallModule(t, Execute, "sum", object.CallArgs{Positional: object.Args{object.String("key"), object.String("data")}})
	verified := modtest.CallModule(t, Execute, "verify", object.CallArgs{Positional: object.Args{signature, object.String("key"), object.String("data")}})
	if verified != object.True {
		t.Fatalf("verify() = %v", verified)
	}
}
