package hash

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func member(t *testing.T, name string) *object.Function {
	t.Helper()
	module, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	return module.(*object.Module).Members[name].(*object.Function)
}

func hexOf(t *testing.T, value object.Object) string {
	t.Helper()
	b, ok := value.(object.Bytes)
	if !ok {
		t.Fatalf("got %s, want Bytes", value.TypeName())
	}
	return hex.EncodeToString(b)
}

// FIPS 180 / RFC 1321 test vectors.
func TestDigestVectors(t *testing.T) {
	cases := []struct{ fn, data, want string }{
		{"sha256", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"sha256", "abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"sha512", "abc", "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
		{"sha1", "abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{"md5", "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"md5", "abc", "900150983cd24fb0d6963f7d28e17f72"},
	}
	for _, c := range cases {
		for _, data := range []object.Object{object.String(c.data), object.NewBytes([]byte(c.data))} {
			got := hexOf(t, modtest.CallModule(t, Execute, c.fn, object.CallArgs{Positional: object.Args{data}}))
			if got != c.want {
				t.Errorf("%s(%q) = %s, want %s", c.fn, c.data, got, c.want)
			}
		}
	}
}

// RFC 4231 test case 2 and the widely published "quick brown fox" HMACs.
func TestHMACVectors(t *testing.T) {
	fox := object.String("The quick brown fox jumps over the lazy dog")
	cases := []struct {
		key, data object.Object
		algorithm string
		want      string
	}{
		{object.String("Jefe"), object.String("what do ya want for nothing?"), "", "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"},
		{object.String("key"), fox, "sha256", "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"},
		{object.NewBytes([]byte("key")), fox, "sha1", "de7c9b85b8b78aa6bc8a7a36f70a90701c9db4d9"},
		{object.String("key"), fox, "md5", "80070713463e7749b90c2dc24911e275"},
		{object.String("key"), fox, "sha512", "b42af09057bac1e2d41708e48a902e09b5ff7f12ab428a4fe86653c73dd248fb82f948a549f7b791a5b41915ee4d1ec3935357e4e2317250d0372afa2ebeeb3a"},
	}
	for _, c := range cases {
		call := object.CallArgs{Positional: object.Args{c.key, c.data}}
		if c.algorithm != "" {
			call.Keyword = object.Kwargs{{Name: "algorithm", Value: member(t, c.algorithm)}}
		}
		if got := hexOf(t, modtest.CallModule(t, Execute, "hmac", call)); got != c.want {
			t.Errorf("hmac(%s) = %s, want %s", c.algorithm, got, c.want)
		}
	}
}

// chunks is a readable handing out its parts one read at a time.
func chunks(parts ...string) object.Object {
	return &object.Module{Name: "chunks", Members: map[string]object.Object{
		"read": &object.Function{Name: "read", Fn: func(object.CallArgs) (object.Object, error) {
			if len(parts) == 0 {
				return object.NewBytes(nil), nil
			}
			part := parts[0]
			parts = parts[1:]
			return object.NewBytes([]byte(part)), nil
		}},
	}}
}

func TestReadableIsHashedToTheEnd(t *testing.T) {
	got := hexOf(t, modtest.CallModule(t, Execute, "sha256", object.CallArgs{Positional: object.Args{chunks("a", "b", "c")}}))
	if got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256(readable) = %s", got)
	}
}

func TestReadableErrorPassesThrough(t *testing.T) {
	failing := &object.Module{Name: "failing", Members: map[string]object.Object{
		"read": &object.Function{Name: "read", Fn: func(object.CallArgs) (object.Object, error) {
			return nil, object.WrapError(object.IOError, "read() disk gone", errors.New("eio"))
		}},
	}}
	_, err := member(t, "md5").Call(object.CallArgs{Positional: object.Args{failing}})
	if !errors.Is(err, object.IOError) || !strings.Contains(err.Error(), "disk gone") {
		t.Fatalf("err = %v, want the readable's IOError", err)
	}
}

func TestErrors(t *testing.T) {
	sha256 := member(t, "sha256")
	cases := []struct {
		fn   string
		call object.CallArgs
		kind *object.Error
		msg  string
	}{
		{"sha256", object.CallArgs{Positional: object.Args{object.Integer(1)}}, object.TypeError, "sha256() argument 'data' must be String, Bytes or have a read(size) method, got Int"},
		{"sha256", object.CallArgs{}, object.TypeError, "sha256() missing required argument: 'data'"},
		{"hmac", object.CallArgs{Positional: object.Args{object.Integer(1), object.String("d")}}, object.TypeError, "hmac() argument 'key' must be String or Bytes, got Int"},
		{"hmac", object.CallArgs{Positional: object.Args{object.String("k"), object.String("d"), object.String("sha256")}}, object.ValueError, "hmac() argument 'algorithm' must be hash.md5, hash.sha1, hash.sha256 or hash.sha512"},
		{"hmac", object.CallArgs{Positional: object.Args{object.String("k"), object.String("d"), &object.Function{Name: "sha256", Fn: sha256.Fn}}}, object.ValueError, "hmac() argument 'algorithm' must be hash.md5, hash.sha1, hash.sha256 or hash.sha512"},
	}
	for _, c := range cases {
		_, err := member(t, c.fn).Call(c.call)
		if err == nil || !errors.Is(err, c.kind) || err.Error() != c.msg {
			t.Errorf("%s: err = %v, want %s %q", c.fn, err, c.kind.Value, c.msg)
		}
	}
}

// hmac recognises its algorithm by identity, which must hold across loads of
// the module.
func TestDigestFunctionsAreStableAcrossLoads(t *testing.T) {
	if member(t, "sha1") != member(t, "sha1") {
		t.Fatal("hash.sha1 differs between module loads")
	}
}
