package extension

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/object"
)

func base64Function(t *testing.T, name string) *object.Function {
	t.Helper()
	modObj, err := ExecuteBase64()
	if err != nil {
		t.Fatalf("ExecuteBase64() error = %v", err)
	}
	fn, ok := modObj.(*object.Module).Members[name].(*object.Function)
	if !ok {
		t.Fatalf("base64 module member %q is not a function", name)
	}
	return fn
}

func callBase64(t *testing.T, name string, arg object.Object) object.Object {
	t.Helper()
	got, err := base64Function(t, name).Call(object.CallArgs{Positional: object.Args{arg}})
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return got
}

func TestBase64StandardRoundTrip(t *testing.T) {
	encoded := callBase64(t, "encode", object.NewBytes([]byte{0, 1, 2, 253, 254, 255}))
	if encoded != object.String("AAEC/f7/") {
		t.Fatalf("encode() = %v", encoded)
	}
	decoded := callBase64(t, "decode", encoded)
	if !decoded.Equals(object.NewBytes([]byte{0, 1, 2, 253, 254, 255})) {
		t.Fatalf("decode() = %v", decoded)
	}
}

func TestBase64URLRoundTripWithoutPadding(t *testing.T) {
	encoded := callBase64(t, "url_encode", object.String("Goblin?"))
	if encoded != object.String("R29ibGluPw") {
		t.Fatalf("url_encode() = %v", encoded)
	}
	decoded := callBase64(t, "url_decode", encoded)
	if !decoded.Equals(object.NewBytes([]byte("Goblin?"))) {
		t.Fatalf("url_decode() = %v", decoded)
	}
}

func TestBase64DecodeRejectsInvalidInput(t *testing.T) {
	_, err := base64Function(t, "decode").Call(object.CallArgs{
		Positional: object.Args{object.String("not base64!")},
	})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("decode() error = %v, want ParseError", err)
	}
}

func TestBase64EncodeRejectsUnsupportedInput(t *testing.T) {
	_, err := base64Function(t, "encode").Call(object.CallArgs{
		Positional: object.Args{object.Integer(1)},
	})
	if err == nil || !errors.Is(err, object.TypeError) {
		t.Fatalf("encode() error = %v, want TypeError", err)
	}
}
