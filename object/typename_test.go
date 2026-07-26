package object

import "testing"

// namedObj stands in for a user-defined type, which reports its declared
// Goblin name rather than the Go type its backend represents it with.
type namedObj struct{ Unit }

func (n *namedObj) TypeName() string { return "Point" }

func TestTypeName(t *testing.T) {
	cases := []struct {
		obj  Object
		want string
	}{
		{Integer(1), "Integer"},
		{Float(1.5), "Float"},
		{String("x"), "String"},
		{True, "Bool"},
		{Bytes("ab"), "Bytes"},
		{Nil, "Nil"},
		{&List{}, "List"},
		{NewDict(), "Dict"},
		{NewPath("/tmp"), "Path"},
		{NewError("boom"), "Error"},
		{&Function{Name: "f"}, "Function"},
		{&Module{Name: "m"}, "Module"},
		{&namedObj{}, "Point"},
		// Embedding another type for its defaults does not name a type:
		// ordObj embeds Unit and would report "Nil" without its own method.
		{&ordObj{}, "ordObj"},
	}
	for _, tc := range cases {
		if got := tc.obj.TypeName(); got != tc.want {
			t.Errorf("%T.TypeName() = %q, want %q", tc.obj, got, tc.want)
		}
	}
}
