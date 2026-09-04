package object

import "testing"

// TestCallMethodMatchesGetAttr pins CallMethod's switch to the method list each
// type publishes through Attributes/GetAttr. The two are separate switches on
// purpose (CallMethod calls the Go method directly so escape analysis can see
// the argument list), so nothing but this test stops them from drifting apart.
func TestCallMethodMatchesGetAttr(t *testing.T) {
	// Every probe gets a fresh receiver: the methods run for real, and some
	// of them mutate.
	types := map[string]func() (Object, MethodCaller){
		"List":   func() (Object, MethodCaller) { l := &List{}; return l, l },
		"String": func() (Object, MethodCaller) { s := String("x"); return s, s },
		"Dict":   func() (Object, MethodCaller) { d := NewDict(); return d, d },
		"Bytes":  func() (Object, MethodCaller) { b := Bytes("x"); return b, b },
	}

	for typeName, fresh := range types {
		obj, _ := fresh()
		for _, name := range obj.Attributes() {
			_, caller := fresh()
			_, handled, _ := caller.CallMethod(name, CallArgs{})
			// "attributes" and "constructor" are not methods of the
			// receiver, so CallMethod declines them and the GetAttr
			// fallback answers instead.
			want := name != "attributes" && name != "constructor"
			if handled != want {
				t.Errorf("%s.CallMethod(%q) handled = %v, want %v", typeName, name, handled, want)
			}
		}

		_, caller := fresh()
		if _, handled, _ := caller.CallMethod("no_such_method", CallArgs{}); handled {
			t.Errorf("%s.CallMethod handled an unknown name", typeName)
		}
	}
}

// TestCallMethodFallsBack covers the shapes CallMethod must not intercept: a
// field holding a function, and a name the receiver does not have at all.
func TestCallMethodFallsBack(t *testing.T) {
	list := &List{Elements: []Object{Integer(1)}}
	if _, err := CallMethod(list, "no_such_method", CallArgs{}); err == nil {
		t.Fatal("expected an attribute error for an unknown method")
	}

	ctor, err := CallMethod(list, "constructor", CallArgs{Positional: Args{list}})
	if err != nil {
		t.Fatalf("constructor via CallMethod: %v", err)
	}
	if _, ok := ctor.(*List); !ok {
		t.Fatalf("expected a list from the constructor, got %T", ctor)
	}

	size, err := CallMethod(list, "size", CallArgs{})
	if err != nil || size != Integer(1) {
		t.Fatalf("size via CallMethod = %v, %v", size, err)
	}
}
