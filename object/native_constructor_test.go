package object

import "testing"

func TestNativeConstructorProvidesStableIdentity(t *testing.T) {
	typeIdentity := NewNativeConstructor("Example", func(CallArgs) (Object, error) {
		return Nil, nil
	})
	value, ok := typeIdentity.Attribute("constructor")
	if !ok || value != typeIdentity.Function {
		t.Fatalf("constructor attribute = %v, %v", value, ok)
	}
	attributes := typeIdentity.Attributes("attributes", "value")
	if got := attributes[len(attributes)-1]; got != "constructor" {
		t.Fatalf("last attribute = %q", got)
	}
	attributes = typeIdentity.Attributes(attributes...)
	if len(attributes) != 3 {
		t.Fatalf("constructor duplicated in %v", attributes)
	}
}
