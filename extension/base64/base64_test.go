package base64

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func call(args ...object.Object) object.CallArgs {
	return object.CallArgs{Positional: args}
}

func with(c object.CallArgs, name string, value object.Object) object.CallArgs {
	c.Keyword = append(c.Keyword, object.KwArg{Name: name, Value: value})
	return c
}

// RFC 4648 section 10 test vectors.
func TestRFC4648Vectors(t *testing.T) {
	vectors := [][2]string{
		{"", ""}, {"f", "Zg=="}, {"fo", "Zm8="}, {"foo", "Zm9v"},
		{"foob", "Zm9vYg=="}, {"fooba", "Zm9vYmE="}, {"foobar", "Zm9vYmFy"},
	}
	for _, v := range vectors {
		got := modtest.CallModule(t, Execute, "encode", call(object.String(v[0])))
		if got != object.String(v[1]) {
			t.Errorf("encode(%q) = %v, want %q", v[0], got, v[1])
		}
		back := modtest.CallModule(t, Execute, "decode", call(object.String(v[1])))
		if !modtest.Equal(back, object.NewBytes([]byte(v[0]))) {
			t.Errorf("decode(%q) = %v, want %q", v[1], back, v[0])
		}
	}
}

func TestAlphabetAndPadding(t *testing.T) {
	data := object.NewBytes([]byte{0xfb, 0xff})
	cases := []struct {
		args object.CallArgs
		want string
	}{
		{call(data), "+/8="},
		{with(call(data), "url_safe", object.True), "-_8="},
		{with(call(data), "padding", object.False), "+/8"},
		{with(with(call(data), "url_safe", object.True), "padding", object.False), "-_8"},
	}
	for _, c := range cases {
		if got := modtest.CallModule(t, Execute, "encode", c.args); got != object.String(c.want) {
			t.Errorf("encode = %v, want %q", got, c.want)
		}
	}
	for _, c := range []struct {
		args object.CallArgs
	}{
		{call(object.String("+/8="))},
		{call(object.String("+/8"))},
		{call(object.String("+/\n8=\r\n"))},
		{with(call(object.String("-_8")), "url_safe", object.True)},
		{with(call(object.String("-_8=")), "url_safe", object.True)},
	} {
		if got := modtest.CallModule(t, Execute, "decode", c.args); !modtest.Equal(got, data) {
			t.Errorf("decode(%v) = %v", c.args.Positional[0], got)
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	module, _ := Execute()
	decode := module.(*object.Module).Members["decode"].(*object.Function)
	cases := []struct {
		args object.CallArgs
		kind *object.Error
		msg  string
	}{
		{call(object.String("Zm9v!mFy")), object.ParseError, "decode() invalid Base64 at position 4"},
		{call(object.String("éZm9v")), object.ParseError, "decode() invalid Base64 at position 0"},
		{call(object.String("Zm9vé")), object.ParseError, "decode() invalid Base64 at position 4"},
		{call(object.String("-_8")), object.ParseError, "decode() invalid Base64 at position 0"},
		{call(object.NewBytes([]byte("Zg=="))), object.TypeError, "decode() argument 'text' must be String, got Bytes"},
	}
	for _, c := range cases {
		_, err := decode.Call(c.args)
		if err == nil || !errors.Is(err, c.kind) || err.Error() != c.msg {
			t.Errorf("decode(%v): err = %v, want %q", c.args.Positional[0], err, c.msg)
		}
	}
}
