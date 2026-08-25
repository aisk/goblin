package base32

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestBase32Variants(t *testing.T) {
	encoded := modtest.CallModule(t, Execute, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin")}, Keyword: object.Kwargs{"padding": object.False}})
	if encoded != object.String("I5XWE3DJNY") {
		t.Fatalf("encode() = %v", encoded)
	}
	decoded := modtest.CallModule(t, Execute, "decode", object.CallArgs{Positional: object.Args{encoded}, Keyword: object.Kwargs{"padding": object.False}})
	if !modtest.Equal(decoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode() = %v", decoded)
	}
	hexEncoded := modtest.CallModule(t, Execute, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin")}, Keyword: object.Kwargs{"hex": object.True}})
	hexDecoded := modtest.CallModule(t, Execute, "decode", object.CallArgs{Positional: object.Args{hexEncoded}, Keyword: object.Kwargs{"hex": object.True}})
	if !modtest.Equal(hexDecoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode(hex=true) = %v", hexDecoded)
	}
}
