package extension

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/aisk/goblin/object"
)

func callModuleFunction(t *testing.T, execute object.ModuleExecutor, name string, args ...object.Object) object.Object {
	t.Helper()
	module, err := execute()
	if err != nil {
		t.Fatalf("execute module: %v", err)
	}
	fn, ok := module.(*object.Module).Members[name].(*object.Function)
	if !ok {
		t.Fatalf("module member %q is not a function", name)
	}
	value, err := fn.Call(object.CallArgs{Positional: args})
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return value
}

func TestBase32Variants(t *testing.T) {
	encoded := callModuleFunction(t, ExecuteBase32, "encode", object.String("Goblin"), object.False)
	if encoded != object.String("I5XWE3DJNY") {
		t.Fatalf("encode() = %v", encoded)
	}
	decoded := callModuleFunction(t, ExecuteBase32, "decode", encoded, object.False)
	if !objectEquals(decoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode() = %v", decoded)
	}
	hexEncoded := callModuleFunction(t, ExecuteBase32, "hex_encode", object.String("Goblin"))
	hexDecoded := callModuleFunction(t, ExecuteBase32, "hex_decode", hexEncoded)
	if !objectEquals(hexDecoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("hex_decode() = %v", hexDecoded)
	}
}

func TestASCII85RoundTrip(t *testing.T) {
	encoded := callModuleFunction(t, ExecuteASCII85, "encode", object.String("Goblin"))
	decoded := callModuleFunction(t, ExecuteASCII85, "decode", encoded)
	if !objectEquals(decoded, object.NewBytes([]byte("Goblin"))) {
		t.Fatalf("decode() = %v", decoded)
	}
}

func TestHTML(t *testing.T) {
	escaped := callModuleFunction(t, ExecuteHTML, "escape", object.String(`<Goblin & "Go">`))
	if escaped != object.String("&lt;Goblin &amp; &#34;Go&#34;&gt;") {
		t.Fatalf("escape() = %v", escaped)
	}
	if got := callModuleFunction(t, ExecuteHTML, "unescape", escaped); got != object.String(`<Goblin & "Go">`) {
		t.Fatalf("unescape() = %v", got)
	}
}

func TestQuotedPrintableRoundTrip(t *testing.T) {
	encoded := callModuleFunction(t, ExecuteQuotedPrintable, "encode", object.String("Goblin = ゴブリン"))
	decoded := callModuleFunction(t, ExecuteQuotedPrintable, "decode", encoded)
	if !objectEquals(decoded, object.NewBytes([]byte("Goblin = ゴブリン"))) {
		t.Fatalf("decode() = %v", decoded)
	}
}

func TestDigestModules(t *testing.T) {
	if got := callModuleFunction(t, ExecuteMD5, "hex", object.String("Goblin")); got != object.String("8e81e2940511f7152ba4462fe53e35b8") {
		t.Fatalf("md5.hex() = %v", got)
	}
	if got := callModuleFunction(t, ExecuteSHA1, "hex", object.String("Goblin")); got != object.String("42a2711c8294fe3a96bfc0f845a2395332de159a") {
		t.Fatalf("sha1.hex() = %v", got)
	}
}

func TestChecksums(t *testing.T) {
	if got := callModuleFunction(t, ExecuteCRC32, "checksum", object.String("Goblin")); got != object.Integer(1982524054) {
		t.Fatalf("crc32.checksum() = %v", got)
	}
	if got := callModuleFunction(t, ExecuteAdler32, "checksum", object.String("Goblin")); got != object.Integer(132579932) {
		t.Fatalf("adler32.checksum() = %v", got)
	}
}

func TestFlateRoundTrip(t *testing.T) {
	compressed := callModuleFunction(t, ExecuteFlate, "compress", object.String("Goblin compression"))
	decompressed := callModuleFunction(t, ExecuteFlate, "decompress", compressed)
	if !objectEquals(decompressed, object.NewBytes([]byte("Goblin compression"))) {
		t.Fatalf("decompress() = %v", decompressed)
	}
}

func TestBzip2Decompress(t *testing.T) {
	compressed, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWfJg5WUAAACVgEAAAIAaJ9gAIAAimAaNCAaAL8F0GoxLDaaPC7kinChIeTBysoA=")
	if err != nil {
		t.Fatal(err)
	}
	got := callModuleFunction(t, ExecuteBzip2, "decompress", object.NewBytes(compressed))
	if !objectEquals(got, object.NewBytes([]byte("Goblin compression"))) {
		t.Fatalf("decompress() = %v", got)
	}
}

func TestNewDecodersReturnParseError(t *testing.T) {
	module, _ := ExecuteASCII85()
	_, err := module.(*object.Module).Members["decode"].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.String("v")}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("decode() error = %v, want ParseError", err)
	}
}
