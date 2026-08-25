package utf8

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestUTF8(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "valid", object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0xff})}}); got != object.False {
		t.Fatalf("valid() = %v", got)
	}
	encoded := modtest.CallModule(t, Execute, "encode", object.CallArgs{Positional: object.Args{object.Integer(0x1f47a)}})
	if !modtest.Equal(encoded, object.NewBytes([]byte("👺"))) {
		t.Fatalf("encode() = %v", encoded)
	}
	module, _ := Execute()
	_, err := module.(*object.Module).Members["decode"].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0xff})}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("decode() error = %v, want ParseError", err)
	}
}
