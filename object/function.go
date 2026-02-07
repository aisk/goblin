package object

import "fmt"

type Function struct {
	Name string
	Fn   func(Args, KwArgs) (Object, error)
}

func (f *Function) Call(args Args, kwargs KwArgs) (Object, error) {
	return f.Fn(args, kwargs)
}

func (f *Function) String() string            { return fmt.Sprintf("<function %s>", f.Name) }
func (f *Function) Repr() string              { return fmt.Sprintf("<function %s>", f.Name) }
func (f *Function) Bool() bool                { return true }
func (f *Function) Compare(Object) (int, error) {
	return 0, fmt.Errorf("cannot compare Function")
}
func (f *Function) Add(Object) (Object, error)      { return nil, fmt.Errorf("cannot add Function") }
func (f *Function) Minus(Object) (Object, error)    { return nil, fmt.Errorf("cannot subtract Function") }
func (f *Function) Multiply(Object) (Object, error) { return nil, fmt.Errorf("cannot multiply Function") }
func (f *Function) Divide(Object) (Object, error)   { return nil, fmt.Errorf("cannot divide Function") }
func (f *Function) And(Object) (Object, error)      { return nil, fmt.Errorf("cannot perform AND on Function") }
func (f *Function) Or(Object) (Object, error)       { return nil, fmt.Errorf("cannot perform OR on Function") }
func (f *Function) Not() (Object, error)             { return nil, fmt.Errorf("cannot perform NOT on Function") }
func (f *Function) Iter() ([]Object, error)          { return nil, fmt.Errorf("Function does not support iteration") }
func (f *Function) Index(Object) (Object, error)     { return nil, fmt.Errorf("Function is not indexable") }
func (f *Function) GetAttr(name string) (Object, error) {
	return nil, fmt.Errorf("Function has no attribute '%s'", name)
}

var _ Object = (*Function)(nil)
