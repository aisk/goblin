package object

import "testing"

func TestFunctionEquals_SamePointer(t *testing.T) {
	f := &Function{Name: "f", Fn: func(CallArgs) (Object, error) { return nil, nil }}

	if !Equals(f, f) {
		t.Fatal("same pointer should be equal")
	}
}

func TestFunctionEquals_DifferentPointer(t *testing.T) {
	f := &Function{Name: "f", Fn: func(CallArgs) (Object, error) { return nil, nil }}
	g := &Function{Name: "g", Fn: func(CallArgs) (Object, error) { return nil, nil }}

	if Equals(f, g) {
		t.Fatal("different pointers should not be equal")
	}
}

func TestFunctionCompare_AlwaysErrors(t *testing.T) {
	f := &Function{Name: "f", Fn: func(CallArgs) (Object, error) { return nil, nil }}

	if _, err := f.Compare(f); err == nil {
		t.Fatal("functions have no ordering; Compare should error")
	}
	if _, err := f.Compare(Integer(0)); err == nil {
		t.Fatal("comparing Function and Integer should error")
	}
}
