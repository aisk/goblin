package object

import (
	"errors"
	"math"
	"testing"
)

// cmpObj stands in for a user-defined type with an Eq impl that considers
// itself equal to any Integer.
type cmpObj struct{ Unit }

func (c *cmpObj) Compare(other Object) (int, error) {
	if _, ok := other.(Integer); ok {
		return 0, nil
	}
	return 0, NewTypeError("cannot compare")
}

func (c *cmpObj) Equals(other Object) (bool, error) {
	cmp, err := c.Compare(other)
	if err != nil {
		return false, nil
	}
	return cmp == 0, nil
}

// plainObj stands in for a user-defined type without Eq: no structural
// equality of its own, so it is equal only to itself via the identity
// backstop in the package-level Equals. The pad field keeps the struct
// non-zero sized so distinct allocations have distinct addresses.
type plainObj struct {
	Unit
	pad int
}

func (p *plainObj) Compare(other Object) (int, error) {
	return 0, NewTypeError("cannot compare")
}

func (p *plainObj) Equals(Object) (bool, error) { return false, nil }

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
		got, err := Equals(tc.a, tc.b)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: Equals(%s, %s) = %v, want %v", tc.name, inspect(tc.a), inspect(tc.b), got, tc.want)
		}
		if got, err := Equals(tc.b, tc.a); err != nil || got != tc.want {
			t.Errorf("%s (swapped): Equals(%s, %s) = %v, %v; want %v", tc.name, inspect(tc.b), inspect(tc.a), got, err, tc.want)
		}
	}
}

func TestEqualsDict(t *testing.T) {
	d1 := NewDict()
	d1.Set(String("k"), Integer(1))
	d2 := NewDict()
	d2.Set(String("k"), Integer(1))
	if eq, err := Equals(d1, d2); err != nil || !eq {
		t.Fatal("dicts with equal entries should be equal")
	}
	d2.Set(String("k"), Integer(2))
	if eq, err := Equals(d1, d2); err != nil || eq {
		t.Fatal("dicts with different values should not be equal")
	}
	d3 := NewDict()
	if eq, err := Equals(d1, d3); err != nil || eq {
		t.Fatal("dicts of different sizes should not be equal")
	}
}

// raisingCmp stands in for a user type whose eq fails: == must report that
// failure rather than answering "not equal".
type raisingCmp struct{ Unit }

func (r *raisingCmp) TypeName() string { return "raisingCmp" }

func (r *raisingCmp) Compare(Object) (int, error) { return 0, NewValueError("cmp exploded") }

func (r *raisingCmp) Equals(other Object) (bool, error) {
	_, err := r.Compare(other)
	return false, err
}

// typeErrorCmp stands in for the common eq shape, written only for the
// operands the type really compares against: `Money(5) == nil` must stay false
// rather than surfacing the TypeError from inside the method.
type typeErrorCmp struct{ Unit }

func (c *typeErrorCmp) TypeName() string { return "typeErrorCmp" }

func (c *typeErrorCmp) Equals(other Object) (bool, error) {
	if _, ok := other.(Integer); !ok {
		return false, NewTypeError("cannot subtract Int and %s", other.TypeName())
	}
	return true, nil
}

func TestEqualsTreatsTypeErrorAsUnequal(t *testing.T) {
	obj := &typeErrorCmp{}
	for _, other := range []Object{Nil, String("x"), &List{}} {
		eq, err := Equals(obj, other)
		if err != nil {
			t.Errorf("Equals(obj, %s) error = %v, want none", inspect(other), err)
		}
		if eq {
			t.Errorf("Equals(obj, %s) = true, want false", inspect(other))
		}
	}
	if eq, err := Equals(obj, Integer(1)); err != nil || !eq {
		t.Errorf("Equals(obj, 1) = %v, %v; want true", eq, err)
	}
}

func TestEqualsPropagatesComparisonFailure(t *testing.T) {
	for _, pair := range [][2]Object{{&raisingCmp{}, Integer(1)}, {Integer(1), &raisingCmp{}}} {
		if _, err := Equals(pair[0], pair[1]); !errors.Is(err, ValueError) {
			t.Errorf("Equals(%s, %s) error = %v, want ValueError", inspect(pair[0]), inspect(pair[1]), err)
		}
	}
}

// eqType builds a user type whose eq answers with answer (or raises err),
// counting its calls.
func eqType(t *testing.T, name string, calls *int, answer Object, err error) *UserType {
	return sealed(t, name, []string{"v"}, ImplSpec{Protocol: EqProtocol, Methods: []ImplMethod{
		method("eq", 2, func([]Object) (Object, error) {
			*calls++
			return answer, err
		}),
	}})
}

// TestEqualsNilNeverReachesEq: nil is unequal to every user value without
// the value's eq being asked, in either order, so an eq that reads a field of
// other never sees nil.
func TestEqualsNilNeverReachesEq(t *testing.T) {
	calls := 0
	money := eqType(t, "Money", &calls, nil, NewAttributeError("Nil has no attribute 'amount'"))
	m := &fakeValue{typ: money, fields: []Object{Integer(5)}}
	for _, pair := range [][2]Object{{m, Nil}, {Nil, m}} {
		if eq, err := Equals(pair[0], pair[1]); err != nil || eq {
			t.Errorf("Equals(%s, %s) = %v, %v; want false", inspect(pair[0]), inspect(pair[1]), eq, err)
		}
		if ne, err := NotEquals(pair[0], pair[1]); err != nil || !ne {
			t.Errorf("NotEquals(%s, %s) = %v, %v; want true", inspect(pair[0]), inspect(pair[1]), ne, err)
		}
	}
	l := &List{Elements: []Object{m, Nil}}
	if r, err := CallMethod(l, "index", CallArgs{Positional: Args{Nil}}); err != nil || r != Integer(1) {
		t.Errorf("index(nil) = %v, %v", r, err)
	}
	if r, err := CallMethod(l, "count", CallArgs{Positional: Args{Nil}}); err != nil || r != Integer(1) {
		t.Errorf("count(nil) = %v, %v", r, err)
	}
	d := NewDict()
	d.Set(Nil, Integer(1))
	if r, err := d.Index(Nil); err != nil || r != Integer(1) {
		t.Errorf("d[nil] = %v, %v", r, err)
	}
	if calls != 0 {
		t.Errorf("eq ran %d times against nil", calls)
	}
}

// TestEqualsLeftDecides: the left operand's eq answers when it returns a
// Bool; the right one is asked only when the left has no eq for the pair, and
// the same object is equal to itself without eq being asked.
func TestEqualsLeftDecides(t *testing.T) {
	var neverCalls, alwaysCalls, pickyCalls int
	never := eqType(t, "Never", &neverCalls, False, nil)
	always := eqType(t, "Always", &alwaysCalls, True, nil)
	picky := eqType(t, "Picky", &pickyCalls, nil, NewTypeError("no"))
	plain := sealed(t, "Plain", []string{"v"})
	boomCalls := 0
	boom := eqType(t, "Boom", &boomCalls, nil, NewZeroDivisionError("division by zero"))
	v := func(typ *UserType) *fakeValue { return &fakeValue{typ: typ, fields: []Object{Integer(1)}} }

	n := v(never)
	cases := []struct {
		name string
		a, b Object
		want bool
	}{
		{"left false is final", v(never), v(always), false},
		{"left true is final", v(always), v(never), true},
		{"same object without eq", n, n, true},
		{"same type, eq says no", v(never), v(never), false},
		{"left without impl Eq asks right", v(plain), v(always), true},
		{"left TypeError asks right", v(picky), v(always), true},
		{"left TypeError, right without Eq", v(picky), v(plain), false},
		{"built-in left asks right", Integer(1), v(always), true},
		{"user left against built-in", v(never), Integer(1), false},
		{"plain is only itself", v(plain), v(plain), false},
	}
	for _, c := range cases {
		if got, err := Equals(c.a, c.b); err != nil || got != c.want {
			t.Errorf("%s: Equals = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
	before := neverCalls
	if eq, _ := Equals(n, n); !eq || neverCalls != before {
		t.Errorf("identity must answer without eq: eq=%v, calls %d -> %d", eq, before, neverCalls)
	}
	if ne, err := NotEquals(v(never), v(always)); err != nil || !ne {
		t.Errorf("!= must negate the left's answer: %v, %v", ne, err)
	}
	if _, err := Equals(v(boom), v(always)); !errors.Is(err, ZeroDivisionError) {
		t.Errorf("an eq that fails for its own reason must propagate, got %v", err)
	}
	if alwaysCalls == 0 || pickyCalls == 0 {
		t.Errorf("eq calls: always %d, picky %d", alwaysCalls, pickyCalls)
	}
}
