package extension

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/object"
)

// callModuleFunction executes a stdlib module, looks up the named function
// member, and calls it with the given positional and/or keyword arguments,
// failing the test on any error. It is shared by the module test files in
// this package (encoding_modules_test.go, digest_modules_test.go,
// compression_modules_test.go, pem_test.go).
func callModuleFunction(t *testing.T, execute object.ModuleExecutor, name string, call object.CallArgs) object.Object {
	t.Helper()
	module, err := execute()
	if err != nil {
		t.Fatalf("execute module: %v", err)
	}
	fn, ok := module.(*object.Module).Members[name].(*object.Function)
	if !ok {
		t.Fatalf("module member %q is not a function", name)
	}
	value, err := fn.Call(call)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return value
}

func TestBase32Variants(t *testing.T) {
	encoded := callModuleFunction(t, ExecuteBase32, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin")}, Keyword: object.Kwargs{"padding": object.False}})
	if encoded != object.String("I5XWE3DJNY") {
		t.Fatalf("encode() = %v", encoded)
	}
	decoded := callModuleFunction(t, ExecuteBase32, "decode", object.CallArgs{Positional: object.Args{encoded}, Keyword: object.Kwargs{"padding": object.False}})
	if !objectEquals(decoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode() = %v", decoded)
	}
	hexEncoded := callModuleFunction(t, ExecuteBase32, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin")}, Keyword: object.Kwargs{"hex": object.True}})
	hexDecoded := callModuleFunction(t, ExecuteBase32, "decode", object.CallArgs{Positional: object.Args{hexEncoded}, Keyword: object.Kwargs{"hex": object.True}})
	if !objectEquals(hexDecoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode(hex=true) = %v", hexDecoded)
	}
}

func TestASCII85RoundTrip(t *testing.T) {
	encoded := callModuleFunction(t, ExecuteASCII85, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin")}})
	decoded := callModuleFunction(t, ExecuteASCII85, "decode", object.CallArgs{Positional: object.Args{encoded}})
	if !objectEquals(decoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode() = %v", decoded)
	}
}

func TestASCII85DecodeReturnsParseError(t *testing.T) {
	module, _ := ExecuteASCII85()
	_, err := module.(*object.Module).Members["decode"].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.String("v")}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("decode() error = %v, want ParseError", err)
	}
}

func TestHTML(t *testing.T) {
	escaped := callModuleFunction(t, ExecuteHTML, "escape", object.CallArgs{Positional: object.Args{object.String(`<Goblin & "Go">`)}})
	if escaped != object.String("&lt;Goblin &amp; &#34;Go&#34;&gt;") {
		t.Fatalf("escape() = %v", escaped)
	}
	if got := callModuleFunction(t, ExecuteHTML, "unescape", object.CallArgs{Positional: object.Args{escaped}}); got != object.String(`<Goblin & "Go">`) {
		t.Fatalf("unescape() = %v", got)
	}
}

func TestQuotedPrintableRoundTrip(t *testing.T) {
	encoded := callModuleFunction(t, ExecuteQuotedPrintable, "encode", object.CallArgs{Positional: object.Args{object.String("Goblin = ゴブリン")}})
	decoded := callModuleFunction(t, ExecuteQuotedPrintable, "decode", object.CallArgs{Positional: object.Args{encoded}})
	if !objectEquals(decoded, object.NewBytes([]byte("Goblin = ゴブリン"))) {
		t.Fatalf("decode() = %v", decoded)
	}
}

func TestUTF8AndUnicode(t *testing.T) {
	if got := callModuleFunction(t, ExecuteUTF8, "valid", object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0xff})}}); got != object.False {
		t.Fatalf("valid() = %v", got)
	}
	encoded := callModuleFunction(t, ExecuteUTF8, "encode", object.CallArgs{Positional: object.Args{object.Integer(0x1f47a)}})
	if !objectEquals(encoded, object.NewBytes([]byte("👺"))) {
		t.Fatalf("encode() = %v", encoded)
	}
	if got := callModuleFunction(t, ExecuteUnicode, "is_letter", object.CallArgs{Positional: object.Args{object.String("界")}}); got != object.True {
		t.Fatalf("is_letter() = %v", got)
	}
	module, _ := ExecuteUTF8()
	_, err := module.(*object.Module).Members["decode"].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0xff})}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("decode() error = %v, want ParseError", err)
	}
}
