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
	if !objectEquals(decoded, object.NewBytes([]byte{0, 1, 2, 253, 254, 255})) {
		t.Fatalf("decode() = %v", decoded)
	}
}

func TestBase64URLRoundTripWithoutPadding(t *testing.T) {
	urlUnpadded := object.Kwargs{"url": object.True, "padding": object.False}
	encoded, err := base64Function(t, "encode").Call(object.CallArgs{
		Positional: object.Args{object.String("Goblin?")},
		Keyword:    urlUnpadded,
	})
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if encoded != object.String("R29ibGluPw") {
		t.Fatalf("encode(url=true, padding=false) = %v", encoded)
	}
	decoded, err := base64Function(t, "decode").Call(object.CallArgs{
		Positional: object.Args{encoded},
		Keyword:    object.Kwargs{"url": object.True, "padding": object.False},
	})
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if !objectEquals(decoded, object.NewBytes([]byte("Goblin?"))) {
		t.Fatalf("decode(url=true, padding=false) = %v", decoded)
	}
}

// The alphabet and padding knobs compose independently: URL alphabet with
// padding kept.
func TestBase64URLAlphabetKeepsPaddingByDefault(t *testing.T) {
	encoded, err := base64Function(t, "encode").Call(object.CallArgs{
		Positional: object.Args{object.NewBytes([]byte{251, 255})},
		Keyword:    object.Kwargs{"url": object.True},
	})
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if encoded != object.String("-_8=") {
		t.Fatalf("encode(url=true) = %v", encoded)
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
