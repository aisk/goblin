package object

import (
	"math"
	"testing"
)

// cmpObj is a minimal non-core Object whose Compare reports equality with any
// Integer, standing in for a user-defined type with __cmp.
type cmpObj struct{ Unit }

func (c *cmpObj) Compare(other Object) (int, error) {
	if _, ok := other.(Integer); ok {
		return 0, nil
	}
	return 0, NewTypeError("cannot compare")
}

// plainObj is a non-core Object without usable comparison, standing in for a
// user-defined type without __cmp. The pad field keeps the struct non-zero
// sized so distinct allocations have distinct addresses.
type plainObj struct {
	Unit
	pad int
}

func (p *plainObj) Compare(other Object) (int, error) {
	return 0, NewTypeError("cannot compare")
}

func TestEquals(t *testing.T) {
	shared := &List{Elements: []Object{Integer(1)}}
	proto := &cmpObj{}
	plain := &plainObj{}

	cases := []struct {
		name string
		a, b Object
		want bool
	}{
		{"int/int equal", Integer(1), Integer(1), true},
		{"int/int unequal", Integer(1), Integer(2), false},
		{"int/float cross", Integer(1), Float(1.0), true},
		{"float/int cross", Float(2.5), Integer(2), false},
		{"nan not equal to itself", Float(math.NaN()), Float(math.NaN()), false},
		{"string equal", String("a"), String("a"), true},
		{"bool unequal", True, False, false},
		{"nil equals nil", Nil, Nil, true},
		{"nil vs int", Nil, Integer(0), false},
		{"int vs string never equal", Integer(1), String("1"), false},
		{"list structural", &List{Elements: []Object{Integer(1), String("x")}}, &List{Elements: []Object{Integer(1), String("x")}}, true},
		{"list nested", &List{Elements: []Object{shared}}, &List{Elements: []Object{&List{Elements: []Object{Integer(1)}}}}, true},
		{"list length mismatch", &List{Elements: []Object{Integer(1)}}, &List{Elements: []Object{}}, false},
		{"list vs int", &List{Elements: []Object{}}, Integer(0), false},
		{"bytes structural via Compare", Bytes("ab"), Bytes("ab"), true},
		{"bytes unequal", Bytes("ab"), Bytes("ac"), false},
		{"cmp dispatch lhs", proto, Integer(42), true},
		{"cmp dispatch rhs (reflected)", Integer(42), proto, true},
		{"no cmp: identity", plain, plain, true},
		{"no cmp: distinct", plain, &plainObj{}, false},
	}
	for _, tc := range cases {
		if got := Equals(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: Equals(%s, %s) = %v, want %v", tc.name, tc.a.String(), tc.b.String(), got, tc.want)
		}
		if got := Equals(tc.b, tc.a); got != tc.want {
			t.Errorf("%s (swapped): Equals(%s, %s) = %v, want %v", tc.name, tc.b.String(), tc.a.String(), got, tc.want)
		}
	}
}

func TestEqualsDict(t *testing.T) {
	d1 := &Dict{Entries: map[string]DictEntry{}}
	d1.Set(String("k"), Integer(1))
	d2 := &Dict{Entries: map[string]DictEntry{}}
	d2.Set(String("k"), Integer(1))
	if !Equals(d1, d2) {
		t.Fatal("dicts with equal entries should be equal")
	}
	d2.Set(String("k"), Integer(2))
	if Equals(d1, d2) {
		t.Fatal("dicts with different values should not be equal")
	}
	d3 := &Dict{Entries: map[string]DictEntry{}}
	if Equals(d1, d3) {
		t.Fatal("dicts of different sizes should not be equal")
	}
}
