package object

import "fmt"

type Module struct {
	Name    string
	Members map[string]Object
}

var _ Object = (*Module)(nil)

func (m *Module) String() string {
	if m.Name != "" {
		return fmt.Sprintf("<module %q>", m.Name)
	}
	return "<module>"
}
func (m *Module) Bool() bool { return true }
func (m *Module) Compare(Object) (int, error) {
	return 0, NewTypeError("cannot compare Module")
}
func (m *Module) Add(Object) (Object, error)      { return nil, NewTypeError("cannot add Module") }
func (m *Module) Minus(Object) (Object, error)    { return nil, NewTypeError("cannot subtract Module") }
func (m *Module) Multiply(Object) (Object, error) { return nil, NewTypeError("cannot multiply Module") }
func (m *Module) Divide(Object) (Object, error)   { return nil, NewTypeError("cannot divide Module") }
func (m *Module) And(Object) (Object, error) {
	return nil, NewTypeError("cannot perform AND on Module")
}
func (m *Module) Or(Object) (Object, error) { return nil, NewTypeError("cannot perform OR on Module") }
func (m *Module) Not() (Object, error)      { return nil, NewTypeError("cannot perform NOT on Module") }
func (m *Module) Iter() ([]Object, error) {
	return nil, NewTypeError("Module does not support iteration")
}
func (m *Module) Index(Object) (Object, error) { return nil, NewTypeError("Module is not indexable") }
func (m *Module) GetAttr(name string) (Object, error) {
	if val, ok := m.Members[name]; ok {
		return val, nil
	}
	return nil, NewTypeError("module has no attribute '%s'", name)
}

var _ Object = (*Module)(nil)
