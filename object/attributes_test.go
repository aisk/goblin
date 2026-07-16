package object

import "testing"

func TestAttributesMethodReturnsResolvableNames(t *testing.T) {
	objects := []Object{
		Nil, True, Integer(1), Float(1), String("x"), Bytes("x"),
		&List{}, NewDict(), &Chan{ch: make(chan Object)}, NewPath("."),
		&Function{Name: "f", Fn: func(CallArgs) (Object, error) { return Nil, nil }},
		NewError("boom"), &Module{Name: "m", Members: map[string]Object{"value": Integer(1)}},
	}
	for _, obj := range objects {
		for _, name := range obj.Attributes() {
			if _, err := obj.GetAttr(name); err != nil {
				t.Errorf("%T.Attributes() contains %q, but GetAttr failed: %v", obj, name, err)
			}
		}
	}
}

func TestAttributesMethodReturnsFreshList(t *testing.T) {
	obj := &List{}
	method, err := obj.GetAttr("attributes")
	if err != nil {
		t.Fatal(err)
	}
	first, err := Call(method, CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	first.(*List).Elements[0] = String("changed")

	second, err := Call(method, CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if got := second.(*List).Elements[0]; got != String("attributes") {
		t.Fatalf("second attributes() result starts with %v, want attributes", got)
	}
}

func TestAttributesMethodRejectsArguments(t *testing.T) {
	method, _ := Integer(1).GetAttr("attributes")
	if _, err := Call(method, CallArgs{Positional: Args{Integer(1)}}); err == nil {
		t.Fatal("attributes() accepted a positional argument")
	}
}
