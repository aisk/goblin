package object

import (
	"errors"
	"math"
	"testing"
)

// ordObj stands in for a user-defined type with an Ord impl: it orders itself
// against Integers only, the way a user type that compares against a built-in
// does. Anything else is unordered.
type ordObj struct {
	Unit
	value int
}

func (o *ordObj) TypeName() string { return "ordObj" }

func (o *ordObj) Compare(other Object) (int, error) {
	v, ok := other.(Integer)
	if !ok {
		return 0, NewTypeError("cannot compare ordObj and %s", other.TypeName())
	}
	switch {
	case o.value < int(v):
		return -1, nil
	case o.value > int(v):
		return 1, nil
	}
	return 0, nil
}

// raisingObj stands in for a user type whose compare fails for a reason other
// than an unordered pair; that failure must not be swallowed by a reflected
// retry.
type raisingObj struct{ Unit }

var errBoom = NewZeroDivisionError("division by zero")

func (r *raisingObj) Compare(Object) (int, error) { return 0, errBoom }

func TestCompare(t *testing.T) {
	obj := &ordObj{value: 3}

	cases := []struct {
		name string
		a, b Object
		want int
	}{
		{"int/int less", Integer(1), Integer(2), -1},
		{"int/int greater", Integer(2), Integer(1), 1},
		{"int/float equal", Integer(1), Float(1.0), 0},
		{"string order", String("abc"), String("def"), -1},
		{"cmp dispatch lhs", obj, Integer(5), -1},
		{"cmp dispatch rhs (reflected)", Integer(5), obj, 1},
		{"reflected equal", Integer(3), obj, 0},
		{"list first difference", &List{Elements: []Object{Integer(1), Integer(3)}}, &List{Elements: []Object{Integer(2)}}, -1},
		{"list prefix first", &List{Elements: []Object{Integer(1)}}, &List{Elements: []Object{Integer(1), Integer(0)}}, -1},
		{"list numeric equal", &List{Elements: []Object{Integer(1)}}, &List{Elements: []Object{Float(1.0)}}, 0},
	}
	for _, tc := range cases {
		got, err := Compare(tc.a, tc.b)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: Compare(%s, %s) = %d, want %d", tc.name, inspect(tc.a), inspect(tc.b), got, tc.want)
		}
	}
}

func TestCompareUnordered(t *testing.T) {
	// Neither side can order the pair: the left operand's error is reported,
	// since it names the operands in the order the expression used them.
	_, err := Compare(&List{}, &ordObj{value: 1})
	if err == nil {
		t.Fatal("expected an error for an unordered pair")
	}
	if want := "cannot compare List and ordObj"; err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	if !errors.Is(err, TypeError) {
		t.Fatalf("error %q should be a TypeError", err)
	}
}

func TestCompareKeepsNonTypeErrors(t *testing.T) {
	// A failure raised inside a user compare is not a "these operands have no
	// ordering" signal, so there is no reflected retry and the original error
	// survives.
	if _, err := Compare(&raisingObj{}, Integer(1)); !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
}

// TestNaNOrdering: the ordering operators answer false for a NaN operand, as
// IEEE 754 has it, and everything that needs an order raises ValueError.
func TestNaNOrdering(t *testing.T) {
	nan := Float(math.NaN())
	ops := map[string]func(a, b Object) (bool, error){
		"<": Less, "<=": LessEqual, ">": Greater, ">=": GreaterEqual,
	}
	for name, op := range ops {
		for _, pair := range [][2]Object{{nan, Integer(1)}, {Integer(1), nan}, {nan, Float(1)}, {Float(1), nan}, {nan, nan}} {
			if r, err := op(pair[0], pair[1]); err != nil || r {
				t.Errorf("%s %s %s = %v, %v; want false", inspect(pair[0]), name, inspect(pair[1]), r, err)
			}
		}
	}
	// The *Int variants the transpiler uses with a native right operand agree.
	for _, op := range []func(Object, int64) (bool, error){LessInt, LessEqualInt, GreaterInt, GreaterEqualInt} {
		if r, err := op(nan, 1); err != nil || r {
			t.Errorf("NaN against a native Int = %v, %v", r, err)
		}
	}
	// Ordinary numbers keep their order, the equal case included.
	if r, _ := LessEqual(Float(2), Integer(2)); !r {
		t.Error("2.0 <= 2 must hold")
	}
	if r, _ := GreaterEqual(Integer(3), Float(2.5)); !r {
		t.Error("3 >= 2.5 must hold")
	}
	if r, _ := Greater(Float(1), Float(1)); r {
		t.Error("1.0 > 1.0 must not hold")
	}

	for _, pair := range [][2]Object{{nan, Integer(1)}, {Integer(1), nan}, {nan, nan}} {
		if _, err := Compare(pair[0], pair[1]); !errors.Is(err, ValueError) || err.Error() != "cannot order NaN" {
			t.Errorf("Compare(%s, %s) error = %v", inspect(pair[0]), inspect(pair[1]), err)
		}
	}
	if _, err := Float(1).Compare(nan); !errors.Is(err, ValueError) {
		t.Errorf("Float.Compare with NaN: %v", err)
	}
	// A list or structural comparison that reaches a NaN element has no answer.
	if _, err := Less(&List{Elements: []Object{nan}}, &List{Elements: []Object{Float(1)}}); !errors.Is(err, ValueError) {
		t.Errorf("[nan] < [1.0]: %v", err)
	}

	l := &List{Elements: []Object{Float(3), nan, Float(1)}}
	if _, err := CallMethod(l, "sort", CallArgs{}); !errors.Is(err, ValueError) {
		t.Errorf("sort with NaN: %v", err)
	}
	if l.Elements[0] != Float(3) || !math.IsNaN(float64(l.Elements[1].(Float))) || l.Elements[2] != Float(1) {
		t.Errorf("a failed sort changed the list: %v", l)
	}
	nanKey := &Function{Name: "k", Fn: func(CallArgs) (Object, error) { return nan, nil }}
	keyed := &List{Elements: []Object{Integer(2), Integer(1)}}
	if _, err := CallMethod(keyed, "sort", CallArgs{Keyword: Kwargs{{Name: "key", Value: nanKey}}}); !errors.Is(err, ValueError) {
		t.Errorf("sort(key=) with a NaN key: %v", err)
	}
	if keyed.Elements[0] != Integer(2) {
		t.Errorf("a failed sort(key=) changed the list: %v", keyed)
	}
}
