package object

import (
	"errors"
	"testing"
)

func TestAttributesReturnsResolvableNames(t *testing.T) {
	objects := []Object{
		Nil, True, Integer(1), Float(1), String("x"), Bytes("x"),
		&List{Elements: []Object{Integer(1)}}, NewDict(), NewChan(0),
		&Function{Name: "f", Fn: func(CallArgs) (Object, error) { return Nil, nil }},
		NewError("boom"), &Module{Name: "m", Members: map[string]Object{"value": Integer(1)}},
	}
	for _, obj := range objects {
		for _, name := range obj.Attributes() {
			if _, err := obj.GetAttr(name); err != nil {
				t.Errorf("%T.Attributes() contains %q, but GetAttr failed: %v", obj, name, err)
			}
		}
		// Reflection is a set of global functions, never a member.
		for _, name := range []string{"attributes", "constructor"} {
			if _, err := obj.GetAttr(name); !errors.Is(err, AttributeError) {
				t.Errorf("%T has a %q member: %v", obj, name, err)
			}
		}
	}
}

func TestAttributesReturnsFreshList(t *testing.T) {
	obj := &List{}
	first := Attributes(obj)
	first.Elements[0] = String("changed")
	if got := Attributes(obj).Elements[0]; got != String("size") {
		t.Fatalf("second attributes() result starts with %v, want size", got)
	}
}

func TestTypeOf(t *testing.T) {
	tests := []struct {
		value Object
		want  *Function
	}{
		{Integer(1), IntConstructorFn},
		{Float(1), FloatConstructorFn},
		{True, BoolConstructorFn},
		{String("x"), StringConstructorFn},
		{Bytes("x"), BytesConstructorFn},
		{&List{}, ListConstructorFn},
		{NewDict(), DictConstructorFn},
		{NewError("x"), ErrorConstructorFn},
		{TypeError, ErrorConstructorFn},
		{IntConstructorFn, FunctionConstructorFn},
		{NewChan(0), ChanConstructorFn},
	}
	for _, tt := range tests {
		if got := TypeOf(tt.value); got != tt.want {
			t.Errorf("TypeOf(%v) = %v, want %v", tt.value, got, tt.want)
		}
	}
	// A type that cannot be constructed has one stand-in per type name.
	if TypeOf(Nil) != TypeOf(Unit{}) || TypeOf(Nil).Name != "Nil" {
		t.Errorf("TypeOf(nil) = %v", TypeOf(Nil))
	}
	if mod := (&Module{Name: "m"}); TypeOf(mod) == TypeOf(Nil) || TypeOf(mod).Name != "Module" {
		t.Errorf("TypeOf(module) = %v", TypeOf(mod))
	}
	if _, err := TypeOf(Nil).Call(CallArgs{}); err == nil || err.Error() != "Nil cannot be constructed" {
		t.Errorf("calling type_of(nil): %v", err)
	}
}
