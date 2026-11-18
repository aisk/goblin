package object

import (
	"reflect"
	"testing"
)

// TestModuleLiveBinding: a member bound with Bind is read afresh at every
// access, so an importer sees the module's later assignments.
func TestModuleLiveBinding(t *testing.T) {
	var counter Object = Integer(0)
	f := &Function{Name: "bump"}
	m := &Module{Name: "lib", Members: map[string]Object{"bump": f}}
	m.Bind("counter", func() Object { return counter })

	if v, err := m.GetAttr("counter"); err != nil || v != Integer(0) {
		t.Fatalf("counter = %v, %v", v, err)
	}
	counter = Integer(2)
	if v, err := m.GetAttr("counter"); err != nil || v != Integer(2) {
		t.Fatalf("counter after the module assigned it = %v, %v; want 2", v, err)
	}
	if v, err := m.GetAttr("bump"); err != nil || v != f {
		t.Fatalf("bump = %v, %v", v, err)
	}
	if got := m.Attributes(); !reflect.DeepEqual(got, []string{"bump", "counter"}) {
		t.Fatalf("Attributes = %v", got)
	}
	if _, err := m.GetAttr("missing"); err == nil || err.Error() != "Module has no attribute 'missing'" {
		t.Fatalf("missing member: %v", err)
	}
	if err := SetAttr(m, "counter", Integer(5)); err == nil {
		t.Fatal("assigning to a module member from outside must be refused")
	}
}
