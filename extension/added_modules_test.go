package extension

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/object"
)

func callModuleFunctionWithArgs(t *testing.T, execute object.ModuleExecutor, name string, args object.CallArgs) (object.Object, error) {
	t.Helper()
	module, err := execute()
	if err != nil {
		t.Fatalf("execute module: %v", err)
	}
	fn, ok := module.(*object.Module).Members[name].(*object.Function)
	if !ok {
		t.Fatalf("module member %q is not a function", name)
	}
	return fn.Call(args)
}

func TestHMAC(t *testing.T) {
	got := callModuleFunction(t, ExecuteHMAC, "hex", object.String("key"), object.String("The quick brown fox jumps over the lazy dog"))
	if got != object.String("f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8") {
		t.Fatalf("hex() = %v", got)
	}
	signature := callModuleFunction(t, ExecuteHMAC, "sum", object.String("key"), object.String("data"))
	verified := callModuleFunction(t, ExecuteHMAC, "verify", signature, object.String("key"), object.String("data"))
	if verified != object.True {
		t.Fatalf("verify() = %v", verified)
	}
}

func TestCRC64AndFNV(t *testing.T) {
	if got := callModuleFunction(t, ExecuteCRC64, "hex", object.String("123456789")); got != object.String("995dc9bbdf1939fa") {
		t.Fatalf("crc64.hex() = %v", got)
	}
	if got := callModuleFunction(t, ExecuteFNV, "hex", object.String("hello")); got != object.String("a430d84680aabd0b") {
		t.Fatalf("fnv.hex() = %v", got)
	}
}

func TestLZWRoundTrip(t *testing.T) {
	compressed := callModuleFunction(t, ExecuteLZW, "compress", object.String("Goblin LZW data"))
	decompressed := callModuleFunction(t, ExecuteLZW, "decompress", compressed)
	if !objectEquals(decompressed, object.NewBytes([]byte("Goblin LZW data"))) {
		t.Fatalf("decompress() = %v", decompressed)
	}
}

func TestPEMRoundTrip(t *testing.T) {
	module, _ := ExecutePEM()
	constructor := module.(*object.Module).Members["Block"].(*object.Function)
	blockObj, err := constructor.Call(object.CallArgs{Positional: object.Args{object.String("MESSAGE"), object.String("Goblin")}})
	if err != nil {
		t.Fatal(err)
	}
	encodedFn, err := blockObj.GetAttr("encode")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodedFn.(*object.Function).Call(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := callModuleFunctionWithArgs(t, ExecutePEM, "decode", object.CallArgs{Positional: object.Args{encoded}})
	if err != nil {
		t.Fatal(err)
	}
	items := decoded.(*object.List).Elements
	equal, err := blockObj.Equals(items[0])
	if err != nil || !equal || len(items[1].(object.Bytes)) != 0 {
		t.Fatalf("decode() = %v, equal=%v, err=%v", decoded, equal, err)
	}
}

func TestUTF8AndUnicode(t *testing.T) {
	if got := callModuleFunction(t, ExecuteUTF8, "valid", object.NewBytes([]byte{0xff})); got != object.False {
		t.Fatalf("valid() = %v", got)
	}
	encoded := callModuleFunction(t, ExecuteUTF8, "encode", object.Integer(0x1f47a))
	if !objectEquals(encoded, object.NewBytes([]byte("👺"))) {
		t.Fatalf("encode() = %v", encoded)
	}
	if got := callModuleFunction(t, ExecuteUnicode, "is_letter", object.String("界")); got != object.True {
		t.Fatalf("is_letter() = %v", got)
	}
	_, err := callModuleFunctionWithArgs(t, ExecuteUTF8, "decode", object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0xff})}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("decode() error = %v, want ParseError", err)
	}
}
