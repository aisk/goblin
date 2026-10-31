package object

import "testing"

// Every built-in type words a missing operation the same way, and an
// operation it has but not for this operand as "cannot <op> X and Y".
func TestOperatorErrorWording(t *testing.T) {
	fn := &Function{Name: "f"}
	ch := NewChan(0)
	cases := []struct {
		name string
		run  func() error
		want string
	}{
		{"function add", func() error { _, err := fn.Add(Integer(1)); return err }, "cannot add Function"},
		{"function compare", func() error { _, err := fn.Compare(fn); return err }, "cannot compare Function"},
		{"module iter", func() error { _, err := (&Module{Name: "m"}).Iter(); return err }, "Module is not iterable"},
		{"trait index", func() error { _, err := EqTrait.Index(Integer(0)); return err }, "Trait is not indexable"},
		{"error subtract", func() error { _, err := NewError("e").Minus(Integer(1)); return err }, "cannot subtract Error"},
		{"nil add", func() error { _, err := Nil.Add(Integer(1)); return err }, "cannot add Nil"},
		{"nil compare", func() error { _, err := Nil.Compare(Integer(1)); return err }, "cannot compare Nil and Integer"},
		{"dict add", func() error { _, err := NewDict().Add(NewDict()); return err }, "cannot add Dict"},
		{"chan compare", func() error { _, err := ch.Compare(ch); return err }, "cannot compare Chan"},
		{"integer operand", func() error { _, err := Integer(1).Add(String("a")); return err }, "cannot add Integer and String"},
		{"integer iter", func() error { _, err := Integer(1).Iter(); return err }, "Integer is not iterable"},
	}
	for _, c := range cases {
		err := c.run()
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: got %v, want %q", c.name, err, c.want)
		}
	}
}
