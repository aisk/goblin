package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestDigestModules(t *testing.T) {
	if got := callModuleFunction(t, ExecuteMD5, "hex", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.String("8e81e2940511f7152ba4462fe53e35b8") {
		t.Fatalf("md5.hex() = %v", got)
	}
	if got := callModuleFunction(t, ExecuteSHA1, "hex", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.String("42a2711c8294fe3a96bfc0f845a2395332de159a") {
		t.Fatalf("sha1.hex() = %v", got)
	}
}

func TestChecksums(t *testing.T) {
	if got := callModuleFunction(t, ExecuteCRC32, "checksum", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.Integer(1982524054) {
		t.Fatalf("crc32.checksum() = %v", got)
	}
	if got := callModuleFunction(t, ExecuteAdler32, "checksum", object.CallArgs{Positional: object.Args{object.String("Goblin")}}); got != object.Integer(132579932) {
		t.Fatalf("adler32.checksum() = %v", got)
	}
}

func TestHMAC(t *testing.T) {
	got := callModuleFunction(t, ExecuteHMAC, "hex", object.CallArgs{Positional: object.Args{object.String("key"), object.String("The quick brown fox jumps over the lazy dog")}})
	if got != object.String("f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8") {
		t.Fatalf("hex() = %v", got)
	}
	signature := callModuleFunction(t, ExecuteHMAC, "sum", object.CallArgs{Positional: object.Args{object.String("key"), object.String("data")}})
	verified := callModuleFunction(t, ExecuteHMAC, "verify", object.CallArgs{Positional: object.Args{signature, object.String("key"), object.String("data")}})
	if verified != object.True {
		t.Fatalf("verify() = %v", verified)
	}
}

func TestCRC64AndFNV(t *testing.T) {
	if got := callModuleFunction(t, ExecuteCRC64, "hex", object.CallArgs{Positional: object.Args{object.String("123456789")}}); got != object.String("995dc9bbdf1939fa") {
		t.Fatalf("crc64.hex() = %v", got)
	}
	if got := callModuleFunction(t, ExecuteFNV, "hex", object.CallArgs{Positional: object.Args{object.String("hello")}}); got != object.String("a430d84680aabd0b") {
		t.Fatalf("fnv.hex() = %v", got)
	}
}
