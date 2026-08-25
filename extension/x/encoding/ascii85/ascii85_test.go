package ascii85

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestASCII85RoundTrip(t *testing.T) {
	encoded := modtest.CallModule(t, Execute, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin")}})
	decoded := modtest.CallModule(t, Execute, "decode", object.CallArgs{Positional: object.Args{encoded}})
	if !modtest.Equal(decoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode() = %v", decoded)
	}
}

func TestASCII85DecodeReturnsParseError(t *testing.T) {
	module, _ := Execute()
	_, err := module.(*object.Module).Members["decode"].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.String("v")}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("decode() error = %v, want ParseError", err)
	}
}
