package gzip

import (
	"bytes"
	gogzip "compress/gzip"
	"errors"
	"io"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func fn(t *testing.T, name string) *object.Function {
	t.Helper()
	module, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	return module.(*object.Module).Members[name].(*object.Function)
}

func goGzip(t *testing.T, data string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gogzip.NewWriter(&buf)
	_, _ = w.Write([]byte(data))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRoundTripAndInterop(t *testing.T) {
	for _, level := range []object.Object{object.Nil, object.Integer(1), object.Integer(9)} {
		call := object.CallArgs{Positional: object.Args{object.String("hello, Goblin")}, Keyword: object.Kwargs{{Name: "level", Value: level}}}
		packed := modtest.CallModule(t, Execute, "compress", call).(object.Bytes)
		// Go's own reader (the format gzip(1) writes) reads it back.
		r, err := gogzip.NewReader(bytes.NewReader(packed))
		if err != nil {
			t.Fatal(err)
		}
		plain, _ := io.ReadAll(r)
		if string(plain) != "hello, Goblin" {
			t.Fatalf("level %v: decompressed %q", level, plain)
		}
		back := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{packed}})
		if !modtest.Equal(back, object.NewBytes([]byte("hello, Goblin"))) {
			t.Fatalf("decompress = %v", back)
		}
	}
	// Data written by another implementation, several members in a row.
	data := append(goGzip(t, "one "), goGzip(t, "two")...)
	back := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{object.NewBytes(data)}})
	if !modtest.Equal(back, object.NewBytes([]byte("one two"))) {
		t.Fatalf("multi-member decompress = %v", back)
	}
	empty := modtest.CallModule(t, Execute, "compress", object.CallArgs{Positional: object.Args{object.NewBytes(nil)}})
	if back := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{empty}}); back.(object.Bytes).Size() != object.Integer(0) {
		t.Fatalf("empty round trip = %v", back)
	}
}

func TestErrors(t *testing.T) {
	packed := goGzip(t, "hello, Goblin")
	cases := []struct {
		fn   string
		args object.CallArgs
		kind *object.Error
		msg  string
	}{
		{"compress", object.CallArgs{Positional: object.Args{object.String("x"), object.Integer(0)}}, object.ValueError, "compress() argument 'level' must be from 1 to 9, got 0"},
		{"compress", object.CallArgs{Positional: object.Args{object.String("x"), object.Integer(10)}}, object.ValueError, "compress() argument 'level' must be from 1 to 9, got 10"},
		{"compress", object.CallArgs{Positional: object.Args{object.String("x"), object.String("9")}}, object.TypeError, "compress() argument 'level' must be Int or Nil, got String"},
		{"compress", object.CallArgs{Positional: object.Args{object.Integer(1)}}, object.TypeError, "compress() argument 'data' must be String, Bytes or have a read(size) method, got Int"},
		{"decompress", object.CallArgs{Positional: object.Args{object.NewBytes(nil)}}, object.ParseError, "decompress() not gzip data"},
		{"decompress", object.CallArgs{Positional: object.Args{object.NewBytes([]byte("plain text"))}}, object.ParseError, "decompress() not gzip data"},
		{"decompress", object.CallArgs{Positional: object.Args{object.NewBytes(packed[:len(packed)-4])}}, object.ParseError, "decompress() corrupt gzip data"},
		{"decompress", object.CallArgs{Positional: object.Args{object.String("text")}}, object.TypeError, "decompress() argument 'data' must be Bytes or have a read(size) method, got String"},
	}
	for _, c := range cases {
		_, err := fn(t, c.fn).Call(c.args)
		if err == nil || !errors.Is(err, c.kind) || err.Error() != c.msg {
			t.Errorf("%s(%v): err = %v, want %q", c.fn, c.args.Positional, err, c.msg)
		}
	}
}
