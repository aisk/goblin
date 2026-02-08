package object

import "fmt"

type Module struct {
	Members map[string]Object
}

func (m *Module) String() string            { return fmt.Sprintf("<module>") }
func (m *Module) Repr() string              { return fmt.Sprintf("<module>") }
func (m *Module) Bool() bool                { return true }
func (m *Module) Compare(Object) (int, error) {
	return 0, fmt.Errorf("cannot compare Module")
}
func (m *Module) Add(Object) (Object, error)      { return nil, fmt.Errorf("cannot add Module") }
func (m *Module) Minus(Object) (Object, error)    { return nil, fmt.Errorf("cannot subtract Module") }
func (m *Module) Multiply(Object) (Object, error) { return nil, fmt.Errorf("cannot multiply Module") }
func (m *Module) Divide(Object) (Object, error)   { return nil, fmt.Errorf("cannot divide Module") }
func (m *Module) And(Object) (Object, error)      { return nil, fmt.Errorf("cannot perform AND on Module") }
func (m *Module) Or(Object) (Object, error)       { return nil, fmt.Errorf("cannot perform OR on Module") }
func (m *Module) Not() (Object, error)            { return nil, fmt.Errorf("cannot perform NOT on Module") }
func (m *Module) Iter() ([]Object, error)         { return nil, fmt.Errorf("Module does not support iteration") }
func (m *Module) Index(Object) (Object, error)    { return nil, fmt.Errorf("Module is not indexable") }
func (m *Module) GetAttr(name string) (Object, error) {
	if val, ok := m.Members[name]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("module has no attribute '%s'", name)
}

var _ Object = (*Module)(nil)
