package json

import (
	"errors"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func call(name string, positional object.Args, keyword ...object.KwArg) (object.Object, error) {
	module, err := Execute()
	if err != nil {
		return nil, err
	}
	fn := module.(*object.Module).Members[name].(*object.Function)
	return fn.Call(object.CallArgs{Positional: positional, Keyword: keyword})
}

func encodeText(t *testing.T, v object.Object, keyword ...object.KwArg) string {
	t.Helper()
	got, err := call("encode", object.Args{v}, keyword...)
	if err != nil {
		t.Fatalf("encode(%v) error: %v", v, err)
	}
	return string(got.(object.String))
}

func decodeValue(t *testing.T, text string) object.Object {
	t.Helper()
	got, err := call("decode", object.Args{object.String(text)})
	if err != nil {
		t.Fatalf("decode(%q) error: %v", text, err)
	}
	return got
}

func dict(pairs ...object.Object) *object.Dict {
	d := object.NewDict()
	for i := 0; i < len(pairs); i += 2 {
		d.Set(pairs[i], pairs[i+1])
	}
	return d
}

func list(elements ...object.Object) *object.List {
	return &object.List{Elements: elements}
}

func TestModuleMembers(t *testing.T) {
	module, _ := Execute()
	got := module.(*object.Module).Attributes()
	if strings.Join(got, ",") != "decode,encode" {
		t.Fatalf("json members = %v", got)
	}
}

func TestDecodeScalars(t *testing.T) {
	cases := []struct {
		in   string
		want object.Object
	}{
		{"42", object.Integer(42)},
		{"-7", object.Integer(-7)},
		{"3.14", object.Float(3.14)},
		{"1e2", object.Float(100)},
		{"9223372036854775808", object.Float(9223372036854775808)},
		{"true", object.True},
		{"false", object.False},
		{"null", object.Nil},
		{` "hi" `, object.String("hi")},
	}
	for _, c := range cases {
		got := decodeValue(t, c.in)
		if got.TypeName() != c.want.TypeName() || !modtest.Equal(got, c.want) {
			t.Errorf("decode(%q) = %v (%s), want %v (%s)", c.in, got, got.TypeName(), c.want, c.want.TypeName())
		}
	}
}

func TestDecodeKeepsDocumentOrder(t *testing.T) {
	got := decodeValue(t, `{"z": 1, "a": [true, null], "m": {"y": 2, "b": 3}, "z": 4}`)
	if s := got.(*object.Dict).String(); s != `{"z": 4, "a": [true, nil], "m": {"y": 2, "b": 3}}` {
		t.Fatalf("decode order = %s", s)
	}
}

func TestDecodeAcceptsBytesAndReadables(t *testing.T) {
	got, err := call("decode", object.Args{object.NewBytes([]byte(`[1]`))})
	if err != nil || !modtest.Equal(got, list(object.Integer(1))) {
		t.Fatalf("decode(Bytes) = %v, %v", got, err)
	}
	if _, err := call("decode", object.Args{object.Integer(1)}); err == nil ||
		err.Error() != "decode() argument 'data' must be String, Bytes or have a read(size) method, got Int" {
		t.Fatalf("decode(1) error = %v", err)
	}
}

func TestDecodeErrorsCarryPosition(t *testing.T) {
	cases := []struct {
		in           string
		message      string
		line, column int64
	}{
		{"", "decode() invalid JSON at line 1, column 1: unexpected end of input", 1, 1},
		{"  ", "decode() invalid JSON at line 1, column 3: unexpected end of input", 1, 3},
		{"{\n  \"port\": 80,\n}", "decode() invalid JSON at line 3, column 1: unexpected \"}\"", 3, 1},
		{"[1, 2", "decode() invalid JSON at line 1, column 6: unexpected end of input", 1, 6},
		{"{\"é\": tru!}", "decode() invalid JSON at line 1, column 10: unexpected \"!\"", 1, 10},
		{"1 2", "decode() invalid JSON at line 1, column 3: unexpected \"2\"", 1, 3},
		{"\"a\nb\"", "decode() invalid JSON at line 1, column 3: unexpected \"\\n\"", 1, 3},
		{"\xef\xbb\xbf1", "decode() invalid JSON at line 1, column 1: unexpected \"\\ufeff\"", 1, 1},
		{"[1e999]", "decode() invalid JSON at line 1, column 2: number 1e999 is out of range", 1, 2},
	}
	for _, c := range cases {
		_, err := call("decode", object.Args{object.String(c.in)})
		var e *object.Error
		if !errors.As(err, &e) || !errors.Is(err, object.ParseError) {
			t.Fatalf("decode(%q) error = %v, want ParseError", c.in, err)
		}
		if e.Error() != c.message {
			t.Errorf("decode(%q) message = %q, want %q", c.in, e.Error(), c.message)
		}
		line, _ := e.Field("line")
		column, _ := e.Field("column")
		if line != object.Integer(c.line) || column != object.Integer(c.column) {
			t.Errorf("decode(%q) data = line %v, column %v", c.in, line, column)
		}
	}
}

func TestEncodeKeepsDictOrder(t *testing.T) {
	d := dict(object.String("b"), object.Integer(2), object.String("a"), list(object.Integer(1), object.Nil))
	if got := encodeText(t, d); got != `{"b":2,"a":[1,null]}` {
		t.Errorf("encode = %s", got)
	}
}

func TestEncodeIndent(t *testing.T) {
	d := dict(object.String("a"), list(object.Integer(1), object.Integer(2)), object.String("e"), list())
	indent := object.KwArg{Name: "indent", Value: object.Integer(2)}
	if got, want := encodeText(t, d, indent), "{\n  \"a\": [\n    1,\n    2\n  ],\n  \"e\": []\n}"; got != want {
		t.Errorf("encode indent=2 = %q, want %q", got, want)
	}
	zero := object.KwArg{Name: "indent", Value: object.Integer(0)}
	if got, want := encodeText(t, list(object.Integer(1)), zero), "[\n1\n]"; got != want {
		t.Errorf("encode indent=0 = %q, want %q", got, want)
	}
	none := object.KwArg{Name: "indent", Value: object.Nil}
	if got := encodeText(t, list(object.Integer(1)), none); got != "[1]" {
		t.Errorf("encode indent=nil = %q", got)
	}
}

func TestEncodeScalars(t *testing.T) {
	cases := map[string]object.Object{
		"null": object.Nil, "true": object.True, "2.0": object.Float(2), "0.1": object.Float(0.1),
		`"a\"\n"`: object.String("a\"\n"), "-3": object.Integer(-3),
	}
	for want, v := range cases {
		if got := encodeText(t, v); got != want {
			t.Errorf("encode(%v) = %s, want %s", v, got, want)
		}
	}
}

func TestEncodeErrors(t *testing.T) {
	selfList := list(object.Integer(1))
	selfList.Elements = append(selfList.Elements, selfList)
	cases := []struct {
		value   object.Object
		kw      []object.KwArg
		kind    *object.Error
		message string
	}{
		{object.NewChan(0), nil, object.TypeError, "encode() cannot encode Chan (give its type a to_json() method)"},
		{object.NewBytes([]byte("x")), nil, object.TypeError, "encode() cannot encode Bytes (give its type a to_json() method)"},
		{dict(object.Integer(1), object.Integer(2)), nil, object.TypeError, "encode() Dict keys must be String, got Int"},
		{list(), []object.KwArg{{Name: "indent", Value: object.Integer(-1)}}, object.ValueError, "encode() argument 'indent' must be non-negative, got -1"},
		{selfList, nil, object.RecursionError, object.ErrRecursionDepth},
	}
	for _, c := range cases {
		_, err := call("encode", object.Args{c.value}, c.kw...)
		if err == nil || !errors.Is(err, c.kind) || err.Error() != c.message {
			t.Errorf("encode(%s) error = %v, want %s %q", c.value.TypeName(), err, c.kind, c.message)
		}
	}
}

func TestEncodeRejectsNaNAndInfinity(t *testing.T) {
	for _, v := range []float64{inf(1), inf(-1), nan()} {
		_, err := call("encode", object.Args{object.Float(v)})
		if err == nil || !errors.Is(err, object.ValueError) || !strings.Contains(err.Error(), "JSON has no NaN or infinity") {
			t.Errorf("encode(%v) error = %v", v, err)
		}
	}
}

func inf(sign int) float64 {
	big := 1e308
	return big * 10 * float64(sign)
}

func nan() float64 {
	x := inf(1)
	return x - x
}

func TestRoundTrip(t *testing.T) {
	original := dict(
		object.String("name"), object.String("Bob"),
		object.String("tags"), list(object.String("x"), object.Float(1.5), object.Nil),
		object.String("ok"), object.True,
	)
	back := decodeValue(t, encodeText(t, original))
	if !modtest.Equal(back, original) || back.(*object.Dict).String() != original.String() {
		t.Fatalf("round trip = %v", back)
	}
}
