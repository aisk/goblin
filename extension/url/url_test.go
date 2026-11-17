package url

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

func callFn(name string, args ...object.Object) (object.Object, error) {
	m, _ := Execute()
	return m.(*object.Module).Members[name].(*object.Function).Call(object.CallArgs{Positional: args})
}

func attr(t *testing.T, v object.Object, name string) object.Object {
	t.Helper()
	got, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestParseParts(t *testing.T) {
	u := modtest.CallModule(t, Execute, "parse", object.CallArgs{Positional: object.Args{object.String("HTTPS://example.com:8443/a%20b?q=a+b#f%201")}})
	want := map[string]object.Object{
		"scheme": object.String("https"), "host": object.String("example.com"), "port": object.Integer(8443),
		"path": object.String("/a b"), "query": object.String("q=a+b"), "fragment": object.String("f 1"),
	}
	for name, v := range want {
		if got := attr(t, u, name); !modtest.Equal(got, v) {
			t.Errorf("%s = %v, want %v", name, got, v)
		}
	}
	if u.(*urlvalue.URL).Constructor() != urlvalue.Type.Function {
		t.Fatal("type_of(URL) is not url.URL")
	}
}

func TestErrorsCarryText(t *testing.T) {
	for _, c := range []struct{ fn, text string }{{"parse", "ht!tp://x"}, {"decode_query", "a=%zz"}} {
		_, err := callFn(c.fn, object.String(c.text))
		var e *object.Error
		if !errors.As(err, &e) || !errors.Is(err, object.ParseError) {
			t.Fatalf("%s(%q) = %v", c.fn, c.text, err)
		}
		if text, _ := e.Field("text"); text != object.String(c.text) {
			t.Fatalf("%s text = %v", c.fn, text)
		}
	}
}

func TestQueryRoundTrip(t *testing.T) {
	d := object.NewDict()
	d.Set(object.String("q"), object.String("a b&c"))
	d.Set(object.String("tag"), &object.List{Elements: []object.Object{object.String("x"), object.Integer(2)}})
	d.Set(object.String("none"), object.Nil)
	text, err := callFn("encode_query", d)
	if err != nil || text != object.String("q=a+b%26c&tag=x&tag=2") {
		t.Fatalf("encode_query = %v, %v", text, err)
	}
	back, err := callFn("decode_query", text)
	if err != nil || back.(*object.Dict).Size() != object.Integer(2) {
		t.Fatalf("decode_query = %v, %v", back, err)
	}
}
