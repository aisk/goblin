package object

import "fmt"

type Method struct {
	Fn func(Args, KwArgs) (Object, error)
}

func (m *Method) Call(args Args, kwargs KwArgs) (Object, error) {
	return m.Fn(args, kwargs)
}

func (m *Method) String() string            { return "<method>" }
func (m *Method) Repr() string              { return "<method>" }
func (m *Method) Bool() bool                { return true }
func (m *Method) Compare(Object) (int, error) {
	return 0, fmt.Errorf("cannot compare Method")
}
func (m *Method) Add(Object) (Object, error)      { return nil, fmt.Errorf("cannot add Method") }
func (m *Method) Minus(Object) (Object, error)     { return nil, fmt.Errorf("cannot subtract Method") }
func (m *Method) Multiply(Object) (Object, error)  { return nil, fmt.Errorf("cannot multiply Method") }
func (m *Method) Divide(Object) (Object, error)    { return nil, fmt.Errorf("cannot divide Method") }
func (m *Method) And(Object) (Object, error)       { return nil, fmt.Errorf("cannot perform AND on Method") }
func (m *Method) Or(Object) (Object, error)        { return nil, fmt.Errorf("cannot perform OR on Method") }
func (m *Method) Not() (Object, error)             { return nil, fmt.Errorf("cannot perform NOT on Method") }
func (m *Method) Iter() ([]Object, error)          { return nil, fmt.Errorf("Method does not support iteration") }
func (m *Method) Index(Object) (Object, error)     { return nil, fmt.Errorf("Method is not indexable") }
func (m *Method) GetAttr(name string) (Object, error) {
	return nil, fmt.Errorf("Method has no attribute '%s'", name)
}

var _ Object = (*Method)(nil)
