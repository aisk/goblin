package object

import "testing"

func TestFunctionCompare_SamePointer(t *testing.T) {
	f := &Function{Name: "f", Fn: func(CallArgs) (Object, error) { return nil, nil }}

	cmp, err := f.Compare(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmp != 0 {
		t.Fatalf("same pointer should compare equal (0), got %d", cmp)
	}
}

func TestFunctionCompare_DifferentPointer(t *testing.T) {
	f := &Function{Name: "f", Fn: func(CallArgs) (Object, error) { return nil, nil }}
	g := &Function{Name: "g", Fn: func(CallArgs) (Object, error) { return nil, nil }}

	cmp, err := f.Compare(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmp == 0 {
		t.Fatalf("different pointers should not compare equal")
	}
}

func TestFunctionCompare_NonFunctionErrors(t *testing.T) {
	f := &Function{Name: "f", Fn: func(CallArgs) (Object, error) { return nil, nil }}

	_, err := f.Compare(Integer(0))
	if err == nil {
		t.Fatal("comparing Function and Integer should error")
	}
}
