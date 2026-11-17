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
	for _, name := range []string{"user", "password"} {
		if got := attr(t, u, name); got != object.String("") {
			t.Errorf("%s without credentials = %v", name, got)
		}
	}
	db := modtest.CallModule(t, Execute, "parse", object.CallArgs{Positional: object.Args{object.String("postgres://ada:p%40ss+w@db.example.com:5432/shop")}})
	if user, password := attr(t, db, "user"), attr(t, db, "password"); user != object.String("ada") || password != object.String("p@ss+w") {
		t.Errorf("credentials = %v, %v", user, password)
	}
	userOnly := modtest.CallModule(t, Execute, "parse", object.CallArgs{Positional: object.Args{object.String("ftp://a%20b@host/")}})
	if user, password := attr(t, userOnly, "user"), attr(t, userOnly, "password"); user != object.String("a b") || password != object.String("") {
		t.Errorf("user only = %v, %v", user, password)
	}
	if u.(*urlvalue.URL).Constructor() != urlvalue.Type.Function {
		t.Fatal("type_of(URL) is not url.URL")
	}
}

func TestErrorsCarryText(t *testing.T) {
	for _, c := range []struct{ fn, text string }{{"parse", "ht!tp://x"}, {"decode_query", "a=%zz"}, {"query_values", "b=1&a=%zz"}} {
		args := []object.Object{object.String(c.text)}
		if c.fn == "query_values" {
			args = append(args, object.String("b"))
		}
		_, err := callFn(c.fn, args...)
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

func TestQueryValues(t *testing.T) {
	cases := []struct{ text, name, want string }{
		{"tag=a&tag=b&x=1&tag=c%20d", "tag", `["a", "b", "c d"]`},
		{"?tag=a+b", "tag", `["a b"]`},
		{"tag=a", "x", `[]`},
		{"", "x", `[]`},
		{"flag&flag=&flag=1", "flag", `["", "", "1"]`},
	}
	for _, c := range cases {
		got, err := callFn("query_values", object.String(c.text), object.String(c.name))
		if err != nil {
			t.Fatalf("query_values(%q, %q): %v", c.text, c.name, err)
		}
		if s, _ := got.ToString(); s != c.want {
			t.Errorf("query_values(%q, %q) = %s, want %s", c.text, c.name, s, c.want)
		}
	}
}
