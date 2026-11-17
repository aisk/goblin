package object

import "testing"

type constructedValue struct {
	OpaqueBase
	ctor *NativeConstructor
}

func (v *constructedValue) String() string                 { return "<value>" }
func (v *constructedValue) ToString() (string, error)      { return v.String(), nil }
func (v *constructedValue) GetAttr(string) (Object, error) { return nil, NewAttributeError("no") }
func (v *constructedValue) Attributes() []string           { return nil }
func (v *constructedValue) Constructor() *Function         { return v.ctor.Function }

func TestNativeConstructorIsTheTypeOfItsValues(t *testing.T) {
	typeIdentity := NewNativeConstructor("Example", func(CallArgs) (Object, error) {
		return Nil, nil
	})
	value := &constructedValue{OpaqueBase: MakeOpaqueBase("Example"), ctor: typeIdentity}
	if got := TypeOf(value); got != typeIdentity.Function {
		t.Fatalf("TypeOf(value) = %v, want the native constructor", got)
	}
}
