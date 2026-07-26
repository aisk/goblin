package object

import (
	"errors"
	"testing"
)

// ordObj stands in for a user-defined type with __cmp: it orders itself
// against Integers only, the way a user type that compares against a built-in
// does. Anything else is unordered.
type ordObj struct {
	Unit
	value int
}

func (o *ordObj) Compare(other Object) (int, error) {
	v, ok := other.(Integer)
	if !ok {
		return 0, NewTypeError("cannot compare ordObj and %s", TypeName(other))
	}
	switch {
	case o.value < int(v):
		return -1, nil
	case o.value > int(v):
		return 1, nil
	}
	return 0, nil
}

// raisingObj stands in for a user type whose __cmp fails for a reason other
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
	}
	for _, tc := range cases {
		got, err := Compare(tc.a, tc.b)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: Compare(%s, %s) = %d, want %d", tc.name, tc.a.String(), tc.b.String(), got, tc.want)
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
	// A failure raised inside a user __cmp is not a "these operands have no
	// ordering" signal, so there is no reflected retry and the original error
	// survives.
	if _, err := Compare(&raisingObj{}, Integer(1)); !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
}
