package extension

import (
	"strings"
	"testing"

	"github.com/aisk/goblin/object"
)

func jsonFunction(t *testing.T, name string) *object.Function {
	t.Helper()

	modObj, err := ExecuteJson()
	if err != nil {
		t.Fatalf("ExecuteJson() error = %v", err)
	}

	mod, ok := modObj.(*object.Module)
	if !ok {
		t.Fatalf("ExecuteJson() returned %T", modObj)
	}

	member, ok := mod.Members[name]
	if !ok {
		t.Fatalf("json module missing %q", name)
	}

	fn, ok := member.(*object.Function)
	if !ok {
		t.Fatalf("json module member %q is %T", name, member)
	}

	return fn
}

func TestJsonUnmarshalScalars(t *testing.T) {
	cases := []struct {
		in   string
		want object.Object
	}{
		{"42", object.Integer(42)},
		{"3.14", object.Float(3.14)},
		{"true", object.True},
		{"false", object.False},
		{"null", object.Unit{}},
		{`"hi"`, object.String("hi")},
	}
	for _, c := range cases {
		got, err := jsonFunction(t, "unmarshal").Call(object.CallArgs{Positional: object.Args{object.String(c.in)}})
		if err != nil {
			t.Fatalf("unmarshal(%q) error: %v", c.in, err)
		}
		if got.String() != c.want.String() {
			t.Errorf("unmarshal(%q) = %q, want %q", c.in, got.String(), c.want.String())
		}
	}
}

func TestJsonUnmarshalNumberTypes(t *testing.T) {
	got, err := jsonFunction(t, "unmarshal").Call(object.CallArgs{Positional: object.Args{object.String(`{"i": 1, "f": 2.5}`)}})
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	d := got.(*object.Dict)
	iv, _ := d.Get(object.String("i"))
	if _, ok := iv.(object.Integer); !ok {
		t.Errorf("i want Integer, got %T", iv)
	}
	fv, _ := d.Get(object.String("f"))
	if _, ok := fv.(object.Float); !ok {
		t.Errorf("f want Float, got %T", fv)
	}
}

func TestJsonUnmarshalRejectsBadInput(t *testing.T) {
	if _, err := jsonFunction(t, "unmarshal").Call(object.CallArgs{Positional: object.Args{object.Integer(1)}}); err == nil {
		t.Fatalf("expected error for non-string argument")
	}
	if _, err := jsonFunction(t, "unmarshal").Call(object.CallArgs{Positional: object.Args{object.String("{bad")}}); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
	for _, in := range []string{"42 43", "42 hello", "{} {}", "[1] [2]"} {
		if _, err := jsonFunction(t, "unmarshal").Call(object.CallArgs{Positional: object.Args{object.String(in)}}); err == nil {
			t.Fatalf("expected error for trailing data: %q", in)
		}
	}
}

func TestJsonMarshalCompact(t *testing.T) {
	d := object.NewDict()
	d.Set(object.String("b"), object.Integer(2))
	d.Set(object.String("a"), object.Integer(1))

	got, err := jsonFunction(t, "marshal").Call(object.CallArgs{Positional: object.Args{d}})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	// Key order is unspecified, so accept either ordering of the compact form.
	if s := got.String(); s != `{"a":1,"b":2}` && s != `{"b":2,"a":1}` {
		t.Errorf("marshal = %q, want compact two-key object", got.String())
	}
}

func TestJsonMarshalIndent(t *testing.T) {
	got, err := jsonFunction(t, "marshal").Call(object.CallArgs{Positional: object.Args{
		&object.List{Elements: []object.Object{object.Integer(1), object.Integer(2)}},
		object.Integer(2),
	}})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	want := "[\n  1,\n  2\n]"
	if got.String() != want {
		t.Errorf("marshal indent = %q, want %q", got.String(), want)
	}
}

func TestJsonMarshalNilAndEmpty(t *testing.T) {
	got, err := jsonFunction(t, "marshal").Call(object.CallArgs{Positional: object.Args{object.Unit{}}})
	if err != nil {
		t.Fatalf("marshal nil error: %v", err)
	}
	if got.String() != "null" {
		t.Errorf("marshal(nil) = %q, want null", got.String())
	}

	got, err = jsonFunction(t, "marshal").Call(object.CallArgs{Positional: object.Args{&object.Dict{}}})
	if err != nil {
		t.Fatalf("marshal empty dict error: %v", err)
	}
	if got.String() != "{}" {
		t.Errorf("marshal({}) = %q, want {}", got.String())
	}
}

func TestJsonMarshalUnsupportedType(t *testing.T) {
	if _, err := jsonFunction(t, "marshal").Call(object.CallArgs{Positional: object.Args{jsonFunction(t, "marshal")}}); err == nil {
		t.Fatalf("expected error for unsupported type")
	}
}

func TestJsonRoundTrip(t *testing.T) {
	original := object.NewDict()
	original.Set(object.String("name"), object.String("Bob"))
	original.Set(object.String("active"), object.True)

	s, err := jsonFunction(t, "marshal").Call(object.CallArgs{Positional: object.Args{original}})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	back, err := jsonFunction(t, "unmarshal").Call(object.CallArgs{Positional: object.Args{s}})
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	d := back.(*object.Dict)
	if v, _ := d.Get(object.String("name")); v.String() != "Bob" {
		t.Errorf("round-trip name = %q, want Bob", v.String())
	}
	if v, _ := d.Get(object.String("active")); v.String() != "true" {
		t.Errorf("round-trip active = %q, want true", v.String())
	}
}

func TestJsonMarshalArgCount(t *testing.T) {
	_, err := jsonFunction(t, "marshal").Call(object.CallArgs{Positional: object.Args{}})
	if err == nil || !strings.Contains(err.Error(), "1 or 2") {
		t.Fatalf("expected arg-count error, got %v", err)
	}
}
