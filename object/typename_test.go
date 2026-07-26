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
		// Types outside this package that do not implement Named fall back to
		// their Go type name, stripped of pointer and package qualifier.
		{&ordObj{}, "ordObj"},
	}
	for _, tc := range cases {
		if got := TypeName(tc.obj); got != tc.want {
			t.Errorf("TypeName(%T) = %q, want %q", tc.obj, got, tc.want)
		}
	}
}
