package object

import (
	"errors"
	"testing"
)

// rightObj stands in for a user type defining __radd and __rdiv only: it
// completes those operations from the right, stays silent about subtraction,
// and raises from multiplication the way a failing reflected method does.
type rightObj struct{ Unit }

func (r *rightObj) TypeName() string { return "rightObj" }

func (r *rightObj) RAdd(left Object) (Object, bool, error) {
	return String("radd " + left.String()), true, nil
}

func (r *rightObj) RDivide(left Object) (Object, bool, error) {
	return String("rdiv " + left.String()), true, nil
}

func (r *rightObj) RMultiply(Object) (Object, bool, error) {
	return nil, true, NewValueError("bad multiplier")
}

// pickyObj accepts only integers on its right-hand side, the way a built-in
// that reads in either operand order does.
type pickyObj struct{ Unit }

func (p *pickyObj) TypeName() string { return "pickyObj" }

func (p *pickyObj) RAdd(left Object) (Object, bool, error) {
	if _, ok := left.(Integer); !ok {
		return nil, false, nil
	}
	return String("picky"), true, nil
}

func TestArithNumeric(t *testing.T) {
	cases := []struct {
		name string
		got  func() (Object, error)
		want Object
	}{
		{"int + int", func() (Object, error) { return Add(Integer(1), Integer(2)) }, Integer(3)},
		{"int + float", func() (Object, error) { return Add(Integer(1), Float(0.5)) }, Float(1.5)},
		{"float - int", func() (Object, error) { return Minus(Float(1.5), Integer(1)) }, Float(0.5)},
		{"int * float", func() (Object, error) { return Multiply(Integer(3), Float(0.5)) }, Float(1.5)},
		{"int / int truncates", func() (Object, error) { return Divide(Integer(7), Integer(2)) }, Integer(3)},
		{"int / float", func() (Object, error) { return Divide(Integer(7), Float(2)) }, Float(3.5)},
		{"string + string", func() (Object, error) { return Add(String("a"), String("b")) }, String("ab")},
	}
	for _, tc := range cases {
		got, err := tc.got()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if !Equals(got, tc.want) {
			t.Errorf("%s = %s, want %s", tc.name, got.String(), tc.want.String())
		}
	}
}

func TestArithDivideByZero(t *testing.T) {
	for _, divisor := range []Object{Integer(0), Float(0)} {
		for _, dividend := range []Object{Integer(1), Float(1)} {
			_, err := Divide(dividend, divisor)
			if !errors.Is(err, ZeroDivisionError) {
				t.Errorf("%s / %s: error = %v, want ZeroDivisionError", dividend.String(), divisor.String(), err)
			}
		}
	}
}

func TestArithReflected(t *testing.T) {
	right := &rightObj{}

	if got, err := Add(Integer(1), right); err != nil || got.String() != "radd 1" {
		t.Errorf("Add = %v, %v; want \"radd 1\"", got, err)
	}
	if got, err := Divide(Integer(8), right); err != nil || got.String() != "rdiv 8" {
		t.Errorf("Divide = %v, %v; want \"rdiv 8\"", got, err)
	}

	// No RMinus at all: the left operand's error stands.
	_, err := Minus(Integer(1), right)
	if want := "cannot subtract Integer and rightObj"; err == nil || err.Error() != want {
		t.Errorf("Minus error = %v, want %q", err, want)
	}

	// A reflected method that fails reports its own error, not the left
	// operand's type error.
	_, err = Multiply(Integer(1), right)
	if !errors.Is(err, ValueError) {
		t.Errorf("Multiply error = %v, want ValueError", err)
	}
}

func TestArithReflectedDeclined(t *testing.T) {
	// The handler exists but does not accept this left operand, so the pair is
	// reported by the left operand, which is the one the expression named first.
	_, err := Add(String("x"), &pickyObj{})
	if want := "cannot add String and pickyObj"; err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if got, err := Add(Integer(1), &pickyObj{}); err != nil || got.String() != "picky" {
		t.Fatalf("Add = %v, %v; want \"picky\"", got, err)
	}
}

func TestArithKeepsNonTypeErrors(t *testing.T) {
	// Division by zero is not a "these operands do not fit" signal, so it is
	// never replaced by a reflected attempt.
	if _, err := Divide(Integer(1), Integer(0)); !errors.Is(err, ZeroDivisionError) {
		t.Fatalf("error = %v, want ZeroDivisionError", err)
	}
}

func TestArithBuiltinRepetition(t *testing.T) {
	got, err := Multiply(Integer(3), String("ab"))
	if err != nil || got.String() != "ababab" {
		t.Fatalf("3 * \"ab\" = %v, %v; want \"ababab\"", got, err)
	}
	list := &List{Elements: []Object{Integer(1), Integer(2)}}
	got, err = Multiply(Integer(2), list)
	if err != nil || !Equals(got, &List{Elements: []Object{Integer(1), Integer(2), Integer(1), Integer(2)}}) {
		t.Fatalf("2 * [1, 2] = %v, %v", got, err)
	}
	// A non-integer left operand is still an error, reported by that operand.
	if _, err := Multiply(String("x"), String("y")); err == nil {
		t.Fatal("expected an error multiplying two strings")
	}
}
